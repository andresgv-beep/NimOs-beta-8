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

// removeFstabEntry removes a mount point entry from /etc/fstab.
//
// Beta 8 bug fix: previously used strings.Contains(line, mountPoint)
// which matched any line containing the path as a substring. That
// would remove /nimbus/pools/data-backup when asked to remove
// /nimbus/pools/data, or /etc/cron.d/data-stuff if a path collided.
//
// fstab format: <device> <mountpoint> <fstype> <opts> <dump> <pass>
// Now we parse fields by whitespace and compare field[1] exactly.
func removeFstabEntry(mountPoint string) {
	if mountPoint == "" {
		return
	}
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return
	}
	var kept []string
	removed := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		// Preserve comments and blank lines verbatim
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && fields[1] == mountPoint {
			logMsg("Removing fstab entry: %s", trimmed)
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		// Nothing matched: don't bother rewriting (avoids unnecessary IO
		// and possible permission issues).
		return
	}
	// Atomic write: write to tmp, then rename. Avoids partial files if
	// the daemon dies mid-write.
	tmpPath := "/etc/fstab.nimos.tmp"
	if err := os.WriteFile(tmpPath, []byte(strings.Join(kept, "\n")), 0644); err != nil {
		logMsg("removeFstabEntry: write tmp failed: %v", err)
		return
	}
	if err := os.Rename(tmpPath, "/etc/fstab"); err != nil {
		logMsg("removeFstabEntry: rename failed: %v", err)
		os.Remove(tmpPath)
	}
}

// ─── Orphan Cleanup ──────────────────────────────────────────────────────────

// cleanOrphanPoolDirs removes directories in /nimbus/pools/ that are not
// associated with any configured pool and have nothing mounted on them.
// Safe to call AFTER pool operations (destroy, create), never at startup
// before pools have mounted.
//
// Beta 8 safety guard: if the pool config is empty or unreadable, we
// REFUSE to clean. Otherwise a corrupt/missing storage.json would cause
// us to delete every directory under /nimbus/pools/ — including the
// mount points of pools whose mount currently isn't visible to us due
// to a transient error.
//
// Rule: deletion is only allowed when we have a positively-known list
// of pools to compare against. "Empty list" is treated as "I don't know
// what's there", which is the safe default.
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

	// SAFETY GUARD: if we have no known pools, do nothing. A corrupt or
	// missing config would otherwise lead to mass deletion under
	// /nimbus/pools/.
	if len(knownMounts) == 0 {
		// Check if there's anything at all in /nimbus/pools/. If yes,
		// it's suspicious — log a warning so the admin notices.
		if entries, err := os.ReadDir(nimbusPoolsDir); err == nil && len(entries) > 0 {
			logMsg("cleanOrphanPoolDirs: REFUSING to clean — config has no pools but %d directories exist in %s. Possible corrupt config.",
				len(entries), nimbusPoolsDir)
		}
		return
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

		// Extra safety: only delete if directory is EMPTY. If it has
		// content, log it — could be data the user wants to recover.
		subEntries, err := os.ReadDir(dirPath)
		if err == nil && len(subEntries) > 0 {
			logMsg("cleanOrphanPoolDirs: skipping non-empty orphan %s (%d items inside)",
				dirPath, len(subEntries))
			continue
		}

		// Orphan AND empty AND nothing mounted — safe to remove.
		if err := os.Remove(dirPath); err != nil {
			logMsg("cleanOrphanPoolDirs: failed to remove %s: %v", dirPath, err)
			continue
		}
		logMsg("Cleaned empty orphan directory: %s", dirPath)
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
