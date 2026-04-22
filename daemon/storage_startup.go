package main

// NimOS Storage — Startup, detection, disk scanning, pool dirs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Startup functions (called from main.go) ────────────────────────────────

func zfsAutoImportOnStartup() {
	if !hasZfs {
		return
	}
	// Import all known ZFS pools
	runSafe("zpool", "import", "-a", "-N")

	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, poolRaw := range confPools {
		pm, _ := poolRaw.(map[string]interface{})
		poolType, _ := pm["type"].(string)
		if poolType != "zfs" {
			continue
		}
		zpoolName, _ := pm["zpoolName"].(string)
		mountPoint, _ := pm["mountPoint"].(string)
		if zpoolName == "" || mountPoint == "" {
			continue
		}
		// Check if pool is imported
		if _, ok := runSafe("zpool", "list", "-H", "-o", "name", zpoolName); !ok {
			runSafe("zpool", "import", zpoolName)
		}
		// Set mount point and mount
		runSafe("zfs", "set", "mountpoint="+mountPoint, zpoolName)
		runSafe("zfs", "mount", "-a")
	}
	logMsg("ZFS auto-import completed")
}

func btrfsAutoMountOnStartup() {
	if !hasBtrfs {
		return
	}
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, poolRaw := range confPools {
		pm, _ := poolRaw.(map[string]interface{})
		poolType, _ := pm["type"].(string)
		if poolType != "btrfs" {
			continue
		}
		mountPoint, _ := pm["mountPoint"].(string)
		if mountPoint == "" {
			continue
		}
		// Try mount from fstab
		runSafe("mount", mountPoint)
	}
	logMsg("Btrfs auto-mount completed")
}

func startupStorage() {
	logMsg("startup: Storage initialization...")
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	if len(confPools) == 0 {
		logMsg("startup: No pools configured")
		return
	}
	// Verify pools are mounted and create dirs if needed
	for _, poolRaw := range confPools {
		pm, _ := poolRaw.(map[string]interface{})
		mountPoint, _ := pm["mountPoint"].(string)
		poolName, _ := pm["name"].(string)
		if mountPoint == "" {
			continue
		}
		if isPathOnMountedPool(mountPoint) {
			logMsg("startup: Pool '%s' mounted at %s", poolName, mountPoint)
			createPoolDirs(mountPoint)
		} else {
			logMsg("startup: WARNING — Pool '%s' NOT mounted at %s", poolName, mountPoint)
		}
	}
	logMsg("startup: Storage initialization complete")
}

func startStorageMonitoring() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			checkStorageHealthGo()
		}
	}()
}

func startZfsScheduler() {
	// TODO: reimplement with new storage_health.go
	logMsg("ZFS scheduler: stub (pending rewrite)")
}

// ─── Detection (called from hardware.go) ─────────────────────────────────────

func detectBtrfs() {
	if _, ok := runSafe("which", "mkfs.btrfs"); ok {
		hasBtrfs = true
		logMsg("Btrfs: available")
	} else {
		logMsg("Btrfs: not available")
	}
}

// ─── Disk detection ──────────────────────────────────────────────────────────

func detectStorageDisksGo() map[string]interface{} {
	// TODO: rewrite with storage_disks.go from plan v2
	// For now: minimal implementation that works
	result := map[string]interface{}{
		"eligible":    []interface{}{},
		"nvme":        []interface{}{},
		"usb":         []interface{}{},
		"provisioned": []interface{}{},
	}

	lsblkRaw, ok := runSafe("lsblk", "-J", "-b", "-o", "NAME,SIZE,TYPE,ROTA,MOUNTPOINT,MODEL,SERIAL,TRAN,RM,FSTYPE,LABEL,PKNAME")
	if !ok || lsblkRaw == "" {
		return result
	}

	var data struct {
		BlockDevices []json.RawMessage `json:"blockdevices"`
	}
	if json.Unmarshal([]byte(lsblkRaw), &data) != nil {
		return result
	}

	rootDisk := findRootDiskGo(lsblkRaw)
	confPools := getStorageConfigFull()
	poolDisks := map[string]bool{}
	if pools, ok := confPools["pools"].([]interface{}); ok {
		for _, p := range pools {
			pm, _ := p.(map[string]interface{})
			if disks, ok := pm["disks"].([]interface{}); ok {
				for _, d := range disks {
					if ds, _ := d.(string); ds != "" {
						poolDisks[ds] = true
					}
				}
			}
		}
	}

	var eligible, nvme, usb, provisioned []interface{}

	for _, raw := range data.BlockDevices {
		var dev map[string]interface{}
		json.Unmarshal(raw, &dev)

		devType, _ := dev["type"].(string)
		if devType != "disk" {
			continue
		}
		devName, _ := dev["name"].(string)

		// Whitelist: only sd*, nvme*, vd*
		validPrefix := false
		for _, prefix := range []string{"sd", "nvme", "vd"} {
			if strings.HasPrefix(devName, prefix) {
				validPrefix = true
				break
			}
		}
		if !validPrefix {
			continue
		}

		size := jsonToInt64(dev["size"])
		if size < 1024*1024*1024 { // < 1GB
			continue
		}

		transport, _ := dev["tran"].(string)
		model, _ := dev["model"].(string)
		serial, _ := dev["serial"].(string)
		rotaBool := jsonToBool(dev["rota"])
		removableBool := jsonToBool(dev["rm"])

		diskInfo := map[string]interface{}{
			"name":          devName,
			"path":          "/dev/" + devName,
			"model":         strings.TrimSpace(model),
			"serial":        strings.TrimSpace(serial),
			"size":          size,
			"sizeFormatted": formatBytes(size),
			"transport":     transport,
			"rotational":    rotaBool,
			"removable":     removableBool,
			"isBoot":        devName == rootDisk,
			"partitions":    []interface{}{},
		}

		// Parse partitions
		var partitions []interface{}
		if children, ok := dev["children"].([]interface{}); ok {
			for _, child := range children {
				cm, ok := child.(map[string]interface{})
				if !ok {
					continue
				}
				partSize := jsonToInt64(cm["size"])
				partitions = append(partitions, map[string]interface{}{
					"name":       cm["name"],
					"path":       "/dev/" + fmt.Sprintf("%v", cm["name"]),
					"size":       partSize,
					"fstype":     cm["fstype"],
					"label":      cm["label"],
					"mountpoint": cm["mountpoint"],
				})
			}
		}
		if partitions == nil {
			partitions = []interface{}{}
		}
		diskInfo["partitions"] = partitions
		diskInfo["hasExistingData"] = len(partitions) > 0

		// Classify
		if devName == rootDisk {
			continue // boot disk — never show
		}

		if poolDisks["/dev/"+devName] {
			diskInfo["classification"] = "provisioned"
			provisioned = append(provisioned, diskInfo)
			continue
		}

		// USB pendrive: USB + removable + < 10GB
		if transport == "usb" && removableBool && size < 10*1024*1024*1024 {
			diskInfo["classification"] = "usb"
			usb = append(usb, diskInfo)
			continue
		}

		// NVMe that isn't boot
		if strings.HasPrefix(devName, "nvme") {
			diskInfo["classification"] = "nvme"
			nvme = append(nvme, diskInfo)
			continue
		}

		// Everything else is eligible
		diskInfo["classification"] = "eligible"

		// Add SMART status from cache (lightweight — no smartctl call)
		smartStatus, smartDetails := getSmartDetailsForDisk(devName)
		diskInfo["smartStatus"] = smartStatus
		smart := map[string]interface{}{
			"temperature":  smartDetails.Temperature,
			"powerOnHours": smartDetails.PowerOnHours,
			"pendingSectors": smartDetails.PendingSectors,
			"uncorrectable": smartDetails.Uncorrectable,
			"reallocatedSectors": smartDetails.ReallocatedSectors,
		}
		diskInfo["smart"] = smart

		eligible = append(eligible, diskInfo)
	}

	if eligible == nil { eligible = []interface{}{} }
	if nvme == nil { nvme = []interface{}{} }
	if usb == nil { usb = []interface{}{} }
	if provisioned == nil { provisioned = []interface{}{} }

	result["eligible"] = eligible
	result["nvme"] = nvme
	result["usb"] = usb
	result["provisioned"] = provisioned
	return result
}

func findRootDiskGo(lsblkJSON string) string {
	var data struct {
		BlockDevices []struct {
			Name     string `json:"name"`
			Children []struct {
				Mountpoint interface{} `json:"mountpoint"`
			} `json:"children"`
			Mountpoint interface{} `json:"mountpoint"`
		} `json:"blockdevices"`
	}
	json.Unmarshal([]byte(lsblkJSON), &data)
	for _, dev := range data.BlockDevices {
		for _, child := range dev.Children {
			if mp, _ := child.Mountpoint.(string); mp == "/" {
				return dev.Name
			}
		}
		if mp, _ := dev.Mountpoint.(string); mp == "/" {
			return dev.Name
		}
	}
	return ""
}

// ─── Pool dirs ───────────────────────────────────────────────────────────────

func createPoolDirs(mountPoint string) {
	dirs := []string{"shares", "system-backup/config", "system-backup/snapshots"}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(mountPoint, d), 0755)
	}
}

// ─── Health ──────────────────────────────────────────────────────────────────

func checkStorageHealthGo() []map[string]interface{} {
	var alerts []map[string]interface{}
	pools := getStoragePoolsGo()
	for _, pool := range pools {
		pct, _ := pool["usagePercent"].(int)
		name, _ := pool["name"].(string)
		if pct >= 95 {
			alerts = append(alerts, map[string]interface{}{"severity": "critical", "pool": name, "message": fmt.Sprintf("Pool %s is %d%% full", name, pct)})
		} else if pct >= 85 {
			alerts = append(alerts, map[string]interface{}{"severity": "warning", "pool": name, "message": fmt.Sprintf("Pool %s is %d%% full", name, pct)})
		}
	}
	if alerts == nil { alerts = []map[string]interface{}{} }
	storageAlertsMu.Lock()
	storageAlertsGo = alerts
	storageAlertsMu.Unlock()
	return alerts
}

// ─── Wipe (implemented in storage_wipe.go) ──────────────────────────────────

// ─── Scan / Restore (stubs) ──────────────────────────────────────────────────

func rescanSCSIBuses() {
	entries, err := os.ReadDir("/sys/class/scsi_host")
	if err != nil {
		return
	}
	for _, e := range entries {
		scanPath := filepath.Join("/sys/class/scsi_host", e.Name(), "scan")
		os.WriteFile(scanPath, []byte("- - -"), 0200)
	}
	runSafe("udevadm", "settle", "--timeout=5")
}

func scanForRestorablePoolsGo() []map[string]interface{} {
	var restorable []map[string]interface{}

	// Step 1: Import any ZFS pools not yet imported
	if hasZfs {
		runSafe("zpool", "import", "-a", "-N")
	}

	// Step 2: Get list of imported ZFS pools
	zpoolList, _ := runSafe("zpool", "list", "-H", "-o", "name,size,health")

	// Step 3: Get pools already in storage.json
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	knownPools := map[string]bool{}
	for _, raw := range confPools {
		pm, _ := raw.(map[string]interface{})
		if zn, _ := pm["zpoolName"].(string); zn != "" {
			knownPools[zn] = true
		}
		if n, _ := pm["name"].(string); n != "" {
			knownPools["nimos-"+n] = true
		}
	}

	// Step 4: For each imported ZFS pool, check if it has .nimbus-pool.json
	for _, line := range strings.Split(zpoolList, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		zpoolName := fields[0]
		zpoolSize := fields[1]
		zpoolHealth := fields[2]

		// Skip if already known
		if knownPools[zpoolName] {
			continue
		}

		// Try to find mount point or use default
		mountPoint := "/nimbus/pools/" + strings.TrimPrefix(zpoolName, "nimos-")

		// Try to mount temporarily to read identity
		runSafe("zfs", "set", "mountpoint="+mountPoint, zpoolName)
		runSafe("zfs", "mount", zpoolName)

		// Read identity file
		identityPath := filepath.Join(mountPoint, ".nimbus-pool.json")
		data, err := os.ReadFile(identityPath)
		if err != nil {
			// No identity file — not a NimOS pool, unmount
			runSafe("zfs", "unmount", zpoolName)
			continue
		}

		var identity map[string]interface{}
		if json.Unmarshal(data, &identity) != nil {
			runSafe("zfs", "unmount", zpoolName)
			continue
		}

		// Check if there's a config backup
		hasBackup := false
		backupDir := filepath.Join(mountPoint, "system-backup", "config")
		if _, err := os.Stat(filepath.Join(backupDir, "nimos.db")); err == nil {
			hasBackup = true
		}

		// Check what shares exist on this pool
		sharesDir := filepath.Join(mountPoint, "shares")
		var shareNames []string
		if entries, err := os.ReadDir(sharesDir); err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					shareNames = append(shareNames, e.Name())
				}
			}
		}

		// Check if docker data exists
		hasDocker := false
		if _, err := os.Stat(filepath.Join(mountPoint, "docker", "data")); err == nil {
			hasDocker = true
		}

		poolName, _ := identity["name"].(string)
		poolType, _ := identity["type"].(string)
		vdevType, _ := identity["vdevType"].(string)

		restorable = append(restorable, map[string]interface{}{
			"zpoolName":  zpoolName,
			"name":       poolName,
			"type":       poolType,
			"vdevType":   vdevType,
			"size":       zpoolSize,
			"health":     zpoolHealth,
			"mountPoint": mountPoint,
			"hasBackup":  hasBackup,
			"hasDocker":  hasDocker,
			"shares":     shareNames,
			"identity":   identity,
		})

		// Unmount — user decides whether to restore
		runSafe("zfs", "unmount", zpoolName)
	}

	return restorable
}

func backupConfigToPoolGo() {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	if len(confPools) == 0 {
		return
	}

	// Files to back up — these are everything needed to reconstruct NimOS after reinstall
	configFiles := []string{
		"/var/lib/nimbusos/config/nimos.db",
		"/var/lib/nimbusos/config/storage.json",
		"/var/lib/nimbusos/config/docker.json",
		"/var/lib/nimbusos/config/remote-access.json",
		"/var/lib/nimbusos/config/security.json",
		"/etc/docker/daemon.json",
	}

	backed := 0
	for _, poolRaw := range confPools {
		pm, _ := poolRaw.(map[string]interface{})
		mountPoint, _ := pm["mountPoint"].(string)
		if mountPoint == "" {
			continue
		}

		backupDir := filepath.Join(mountPoint, "system-backup", "config")
		os.MkdirAll(backupDir, 0755)

		for _, src := range configFiles {
			data, err := os.ReadFile(src)
			if err != nil {
				continue // file doesn't exist, skip
			}
			dst := filepath.Join(backupDir, filepath.Base(src))
			if err := os.WriteFile(dst, data, 0600); err != nil {
				logMsg("config backup: failed to write %s → %s: %v", src, dst, err)
			}
		}
		backed++
	}

	if backed > 0 {
		logMsg("config backup: saved to %d pool(s)", backed)
	}
}

// startConfigBackupLoop runs backupConfigToPoolGo periodically (every 30 min)
// and once at startup after a delay.
func startConfigBackupLoop() {
	// Wait for system to settle
	time.Sleep(60 * time.Second)

	// Initial backup
	backupConfigToPoolGo()

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		backupConfigToPoolGo()
	}
}

func appendFstab(uuid, mountPoint, filesystem string) {
	existing, _ := os.ReadFile("/etc/fstab")
	if strings.Contains(string(existing), mountPoint) {
		return
	}
	opts := "defaults,nofail,noatime"
	if filesystem == "btrfs" {
		opts = "defaults,nofail,noatime,compress=zstd"
	}
	entry := fmt.Sprintf("UUID=%s %s %s %s 0 2\n", uuid, mountPoint, filesystem, opts)
	f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
	log.Printf("appendFstab: added %s", mountPoint)
}

// ─── ZFS Pool Info (needed by getStoragePoolsGo) ────────────────────────────

// restorePoolFromIdentity restores a pool found by scanForRestorablePoolsGo.
// Re-adds it to storage.json, recreates shares from the shares/ directory,
// optionally restores Docker config, and restores the DB from backup.
func restorePoolFromIdentity(body map[string]interface{}) map[string]interface{} {
	zpoolName := bodyStr(body, "zpoolName")
	poolName := bodyStr(body, "name")
	restoreConfig, _ := body["restoreConfig"].(bool)

	if zpoolName == "" || poolName == "" {
		return map[string]interface{}{"error": "zpoolName and name required"}
	}

	mountPoint := "/nimbus/pools/" + poolName

	// 1. Mount the pool
	os.MkdirAll(mountPoint, 0755)
	runSafe("zfs", "set", "mountpoint="+mountPoint, zpoolName)
	runSafe("zfs", "mount", "-a")

	// 2. Read identity
	identityPath := filepath.Join(mountPoint, ".nimbus-pool.json")
	identityData, err := os.ReadFile(identityPath)
	if err != nil {
		return map[string]interface{}{"error": "Cannot read pool identity: " + err.Error()}
	}
	var identity map[string]interface{}
	json.Unmarshal(identityData, &identity)

	poolType, _ := identity["type"].(string)
	vdevType, _ := identity["vdevType"].(string)
	disksRaw, _ := identity["disks"].([]interface{})

	// 3. Add pool to storage.json
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})

	// Check if already exists
	for _, raw := range confPools {
		pm, _ := raw.(map[string]interface{})
		if zn, _ := pm["zpoolName"].(string); zn == zpoolName {
			return map[string]interface{}{"error": "Pool already registered in config"}
		}
	}

	isFirst := len(confPools) == 0
	confPools = append(confPools, map[string]interface{}{
		"name":       poolName,
		"type":       poolType,
		"zpoolName":  zpoolName,
		"mountPoint": mountPoint,
		"vdevType":   vdevType,
		"disks":      disksRaw,
		"createdAt":  identity["createdAt"],
	})
	conf["pools"] = confPools
	if isFirst {
		conf["primaryPool"] = poolName
		conf["configuredAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	saveStorageConfigFull(conf)
	logMsg("restore: pool '%s' (%s) added to storage.json", poolName, zpoolName)

	// 4. Recreate shares from the shares/ directory
	sharesDir := filepath.Join(mountPoint, "shares")
	restoredShares := 0
	if entries, err := os.ReadDir(sharesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			shareName := e.Name()
			sharePath := filepath.Join(sharesDir, shareName)

			// Check if share already exists in DB
			existing, _ := dbSharesListRaw()
			found := false
			for _, s := range existing {
				if s.Name == shareName {
					found = true
					break
				}
			}
			if found {
				continue
			}

			// Create share in DB
			err := dbSharesCreate(shareName, shareName, "", sharePath, poolName, poolName, "restore")
			if err != nil {
				logMsg("restore: failed to create share '%s': %v", shareName, err)
				continue
			}
			restoredShares++
			logMsg("restore: recreated share '%s' → %s", shareName, sharePath)
		}
	}

	// 5. Restore Docker config if docker data exists
	dockerRestored := false
	dockerDataDir := filepath.Join(mountPoint, "docker", "data")
	if _, err := os.Stat(dockerDataDir); err == nil {
		// Write daemon.json pointing to this pool
		daemonJSON := fmt.Sprintf(`{"data-root":"%s"}`, dockerDataDir)
		os.WriteFile("/etc/docker/daemon.json", []byte(daemonJSON), 0644)

		// Write NimOS docker config
		dockerConf := map[string]interface{}{
			"pool":     poolName,
			"dataRoot": dockerDataDir,
		}
		dockerConfData, _ := json.MarshalIndent(dockerConf, "", "  ")
		os.WriteFile("/var/lib/nimbusos/config/docker.json", dockerConfData, 0644)

		// Restart Docker to pick up new data root
		runSafe("systemctl", "restart", "docker")
		dockerRestored = true
		logMsg("restore: Docker reconfigured → %s", dockerDataDir)
	}

	// 6. Optionally restore DB from backup
	dbRestored := false
	if restoreConfig {
		backupDir := filepath.Join(mountPoint, "system-backup", "config")
		backupDB := filepath.Join(backupDir, "nimos.db")
		if _, err := os.Stat(backupDB); err == nil {
			// Close current DB, replace with backup, reopen
			db.Close()
			if err := copyFile(backupDB, "/var/lib/nimbusos/config/nimos.db"); err != nil {
				logMsg("restore: failed to restore DB: %v", err)
			} else {
				dbRestored = true
				logMsg("restore: database restored from pool backup")
			}
			// Reopen DB
			openDB()
		}

		// Restore other config files
		for _, name := range []string{"remote-access.json", "security.json"} {
			src := filepath.Join(backupDir, name)
			dst := filepath.Join("/var/lib/nimbusos/config", name)
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0644)
				logMsg("restore: restored %s", name)
			}
		}
	}

	// 7. Ensure standard dirs exist
	createPoolDirs(mountPoint)

	// 8. Register services
	reconcileServices()

	return map[string]interface{}{
		"ok":             true,
		"poolName":       poolName,
		"shares":         restoredShares,
		"dockerRestored": dockerRestored,
		"dbRestored":     dbRestored,
	}
}

// copyFile copies src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// enrichDisksWithSmart takes a flat disk name list and returns enriched objects
// with SMART status from the cached monitor data. Does NOT run smartctl — only
// reads from smartHistory to avoid false positives from stale or slow queries.
// The pool-level status/health is NEVER modified by this function.
