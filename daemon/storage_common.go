package main

// ═══════════════════════════════════════════════════════════════════════════════
// NimOS Storage — Common helpers shared by ZFS and BTRFS pool operations
// ═══════════════════════════════════════════════════════════════════════════════

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Device Helpers ──────────────────────────────────────────────────────────

// partitionName returns the correct partition 1 name.
// SATA/USB: sda → sda1. NVMe: nvme0n1 → nvme0n1p1.
func partitionName(diskName string) string {
	if strings.HasPrefix(diskName, "nvme") {
		return diskName + "p1"
	}
	return diskName + "1"
}

// waitForDevice waits for a device file to appear in /dev/
func waitForDevice(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", path)
}

// getZfsInUseDisks returns a set of base disk names (e.g. "sda", "nvme0n1")
// that are currently in use by ANY imported zpool, regardless of whether the
// pool is registered in storage.json. Used to prevent the disk detector from
// misclassifying disks as "eligible" when ZFS is holding them, and to let
// preFlightCheck reject operations on disks owned by ZFS.
//
// Uses `zpool status -P` which prints full device paths. Parses /dev/sdXN,
// /dev/disk/by-id/..., /dev/nvmeXnY paths and extracts the base disk.
func getZfsInUseDisks() map[string]bool {
	inUse := map[string]bool{}
	if !hasZfs {
		return inUse
	}
	out, ok := runSafe("zpool", "status", "-P")
	if !ok || out == "" {
		return inUse
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// First field is the device path in `status -P` output
		path := fields[0]
		if !strings.HasPrefix(path, "/dev/") {
			continue
		}
		// Resolve symlinks (e.g. /dev/disk/by-id/wwn-... → /dev/sda1)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			resolved = path
		}
		// Extract base disk: /dev/sda1 → sda, /dev/nvme0n1p1 → nvme0n1
		base := strings.TrimPrefix(resolved, "/dev/")
		// Strip partition suffix
		base = stripPartitionSuffix(base)
		if base != "" {
			inUse[base] = true
		}
	}
	return inUse
}

// stripPartitionSuffix removes the partition number from a device name.
// sda1 → sda, sdb2 → sdb, nvme0n1p1 → nvme0n1, nvme0n1p12 → nvme0n1
func stripPartitionSuffix(name string) string {
	if strings.HasPrefix(name, "nvme") {
		// nvme0n1p1 → nvme0n1 (strip "p" + digits)
		if idx := strings.LastIndex(name, "p"); idx > 0 {
			rest := name[idx+1:]
			allDigits := rest != ""
			for _, c := range rest {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return name[:idx]
			}
		}
		return name
	}
	// sdX, vdX: strip trailing digits
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i > 0 {
		return name[:i]
	}
	return name
}

// ─── Pool Identity ───────────────────────────────────────────────────────────

// writePoolIdentity writes the .nimbus-pool.json identity file
func writePoolIdentity(mountPoint, name, poolType, vdevType string, disks []string) {
	identity := map[string]interface{}{
		"name":          name,
		"type":          poolType,
		"vdevType":      vdevType,
		"disks":         disks,
		"createdAt":     time.Now().UTC().Format(time.RFC3339),
		"nimbusVersion": "6.0.0-beta",
	}
	data, _ := json.MarshalIndent(identity, "", "  ")
	os.WriteFile(filepath.Join(mountPoint, ".nimbus-pool.json"), data, 0644)
}

// ─── Config Helpers ──────────────────────────────────────────────────────────

// deleteSharesForPool removes all shares associated with a pool from the DB.
func deleteSharesForPool(poolName, mountPoint string) {
	shares, _ := dbSharesListRaw()
	for _, s := range shares {
		if s.Pool == poolName || s.Volume == poolName || (mountPoint != "" && strings.HasPrefix(s.Path, mountPoint)) {
			handleOp(Request{Op: "share.delete", ShareName: s.Name})
			dbSharesDelete(s.Name)
		}
	}
}

// ─── Fstab ───────────────────────────────────────────────────────────────────

// removeFstabEntry removes a mount point entry from /etc/fstab
func removeFstabEntry(mountPoint string) {
	if mountPoint == "" {
		return
	}
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, mountPoint) {
			logMsg("Removing fstab entry: %s", strings.TrimSpace(line))
			continue
		}
		kept = append(kept, line)
	}
	os.WriteFile("/etc/fstab", []byte(strings.Join(kept, "\n")), 0644)
}

// ─── Orphan Cleanup ──────────────────────────────────────────────────────────

// cleanOrphanPoolDirs removes directories in /nimbus/pools/ that are not
// associated with any configured pool and have nothing mounted on them.
// Safe to call AFTER pool operations (destroy, create), never at startup
// before ZFS has mounted.
func cleanOrphanPoolDirs() {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})

	// Build set of known mount points
	knownMounts := map[string]bool{}
	for _, poolRaw := range confPools {
		pm, _ := poolRaw.(map[string]interface{})
		if mp, _ := pm["mountPoint"].(string); mp != "" {
			knownMounts[mp] = true
		}
	}

	entries, err := os.ReadDir(nimbusPoolsDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(nimbusPoolsDir, e.Name())

		// Skip known pools
		if knownMounts[dirPath] {
			continue
		}

		// Skip if something real is mounted here
		if isPathOnMountedPool(dirPath) {
			continue
		}

		// Orphan on system disk — safe to remove
		os.RemoveAll(dirPath)
		logMsg("Cleaned orphan directory: %s", dirPath)
	}
}

// ─── Torrent Config ──────────────────────────────────────────────────────────

// updateTorrentConfig updates NimTorrent's download_dir to point to the primary
// pool's shares directory. Called after create/destroy pool.
// Without this, NimTorrent writes to the system disk.
const torrentConfPath = "/etc/nimos/torrent.conf"

func updateTorrentConfig() {
	conf := getStorageConfigFull()
	primaryPool, _ := conf["primaryPool"].(string)

	newDir := ""
	if primaryPool != "" {
		confPools, _ := conf["pools"].([]interface{})
		for _, p := range confPools {
			pm, _ := p.(map[string]interface{})
			if n, _ := pm["name"].(string); n == primaryPool {
				mp, _ := pm["mountPoint"].(string)
				if mp != "" {
					newDir = filepath.Join(mp, "shares")
				}
				break
			}
		}
	}

	// Read current config
	data, err := os.ReadFile(torrentConfPath)
	if err != nil {
		// No torrent config — nothing to update
		return
	}

	// Replace download_dir line
	var lines []string
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "download_dir=") {
			if newDir != "" {
				lines = append(lines, "download_dir="+newDir)
			} else {
				lines = append(lines, "download_dir=")
			}
			found = true
		} else {
			lines = append(lines, line)
		}
	}
	if !found && newDir != "" {
		lines = append(lines, "download_dir="+newDir)
	}

	os.WriteFile(torrentConfPath, []byte(strings.Join(lines, "\n")), 0644)

	// Restart torrentd to pick up new config
	runCmd("systemctl", []string{"restart", "nimos-torrentd"}, CmdOptions{Timeout: 10 * time.Second})

	if newDir != "" {
		logMsg("Updated NimTorrent download_dir to %s", newDir)
	} else {
		logMsg("Cleared NimTorrent download_dir (no pools)")
	}
}
