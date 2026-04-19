package main

// NimOS Storage — Disk management (attach/detach/replace/resilver)

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func findPoolConfig(poolName string) (map[string]interface{}, string) {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, p := range confPools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == poolName {
			poolType, _ := pm["type"].(string)
			return pm, poolType
		}
	}
	return nil, ""
}

// POST /api/storage/pool/replace-disk
// Body: { pool: "valume1", oldDisk: "sdb", newDisk: "sdc" }
// POST /api/storage/pool/detach-disk
// Body: { pool: "valume1", disk: "sdb" }
// Removes a disk from a mirror pool WITHOUT requiring a replacement.
// The pool continues in degraded mode with the remaining disk(s).
func handleDetachDisk(body map[string]interface{}) map[string]interface{} {
	poolName := bodyStr(body, "pool")
	diskName := bodyStr(body, "disk")

	if poolName == "" || diskName == "" {
		return map[string]interface{}{"error": "Missing pool or disk"}
	}

	poolConf, poolType := findPoolConfig(poolName)
	if poolConf == nil {
		return map[string]interface{}{"error": "Pool not found"}
	}

	// Ensure disk belongs to the pool
	disks, _ := poolConf["disks"].([]interface{})
	found := false
	for _, d := range disks {
		ds, _ := d.(string)
		if strings.TrimPrefix(ds, "/dev/") == diskName {
			found = true
			break
		}
	}
	if !found {
		return map[string]interface{}{"error": fmt.Sprintf("Disk %s is not part of pool %s", diskName, poolName)}
	}

	// Cannot detach if only 1 disk — would destroy the pool
	if len(disks) <= 1 {
		return map[string]interface{}{"error": "Cannot detach the only disk in the pool. Use 'Destroy volume' instead."}
	}

	// Detach is only valid for mirror/raid1 — RAIDZ does not support removing individual disks
	vdevType, _ := poolConf["vdevType"].(string)
	profile, _ := poolConf["profile"].(string)
	vt := strings.ToLower(vdevType + profile)
	if strings.Contains(vt, "raidz") || strings.Contains(vt, "raid5") || strings.Contains(vt, "raid6") {
		return map[string]interface{}{"error": "No se puede desmontar un disco de un pool RAIDZ. Usa 'Reemplazar disco' en su lugar."}
	}

	// ── Service barrier (obligatoria) ──
	// SIEMPRE comprobar justo antes de ejecutar — el backend no confía en el frontend
	activeSvcs, err := checkPoolDependencies(poolName)
	if err != nil {
		logMsg("DETACH DISK: error checking services for pool %s: %v", poolName, err)
	}
	if len(activeSvcs) > 0 {
		svcNames := make([]string, 0, len(activeSvcs))
		for _, s := range activeSvcs {
			svcNames = append(svcNames, s.AppName)
		}
		return map[string]interface{}{
			"error":    "services_active",
			"message":  fmt.Sprintf("No se puede desconectar el disco: %d servicios activos en el pool", len(activeSvcs)),
			"services": svcNames,
		}
	}

	switch poolType {
	case "zfs":
		return detachDiskZfs(poolConf, diskName)
	case "btrfs":
		return detachDiskBtrfs(poolConf, diskName)
	default:
		return map[string]interface{}{"error": fmt.Sprintf("Unsupported pool type: %s", poolType)}
	}
}

// detachDiskZfs runs: zpool detach <pool> <disk>
func detachDiskZfs(poolConf map[string]interface{}, diskName string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	zpoolName, _ := poolConf["zpoolName"].(string)
	if zpoolName == "" {
		zpoolName = "nimos-" + poolName
	}

	diskPart := partitionName("/dev/" + diskName)

	res, err := runCmd("zpool", []string{"detach", zpoolName, diskPart}, CmdOptions{Timeout: 30 * time.Second})
	if err != nil || !res.OK {
		errMsg := res.Stderr
		if errMsg == "" {
			errMsg = res.Stdout
		}
		return map[string]interface{}{"error": fmt.Sprintf("zpool detach failed: %s", errMsg)}
	}

	// Remove disk from config
	removeDiskFromPoolConfig(poolName, diskName)

	addNotification("warning", "system",
		fmt.Sprintf("Disco %s desmontado de %s", diskName, poolName),
		fmt.Sprintf("El volumen %s funciona en modo degradado. Reemplaza el disco lo antes posible.", poolName))

	logMsg("DISK DETACH: pool %s, removed %s (ZFS)", poolName, diskName)

	return map[string]interface{}{"ok": true, "message": fmt.Sprintf("Disk %s detached. Pool is now degraded.", diskName)}
}

// detachDiskBtrfs runs: btrfs device delete <disk> <mountpoint>
func detachDiskBtrfs(poolConf map[string]interface{}, diskName string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	mountPoint, _ := poolConf["mountPoint"].(string)

	if mountPoint == "" {
		return map[string]interface{}{"error": "Pool mount point not found"}
	}

	// Run in background — btrfs device delete can take a long time (rebalances data)
	go func() {
		res, err := runCmd("btrfs", []string{"device", "delete", "/dev/" + diskName, mountPoint}, CmdOptions{Timeout: 0})
		if err == nil && res.OK {
			removeDiskFromPoolConfig(poolName, diskName)
			addNotification("warning", "system",
				fmt.Sprintf("Disco %s desmontado de %s", diskName, poolName),
				fmt.Sprintf("El volumen %s funciona con menos discos. Añade un disco de reemplazo.", poolName))
			logMsg("DISK DETACH: pool %s, removed %s (BTRFS complete)", poolName, diskName)
		} else {
			errMsg := res.Stderr
			if errMsg == "" && err != nil {
				errMsg = err.Error()
			}
			addNotification("error", "system",
				fmt.Sprintf("Error al desmontar disco de %s", poolName),
				fmt.Sprintf("No se pudo eliminar %s: %s", diskName, errMsg))
			logMsg("DISK DETACH FAILED: pool %s, %s: %s", poolName, diskName, errMsg)
		}
	}()

	addNotification("info", "system",
		fmt.Sprintf("Desmontando disco %s de %s", diskName, poolName),
		"El proceso puede tardar un rato mientras se rebalancea la información.")

	return map[string]interface{}{"ok": true, "message": "Disk removal started"}
}

// removeDiskFromPoolConfig removes a disk from the pool config
func removeDiskFromPoolConfig(poolName, diskName string) {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, p := range confPools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == poolName {
			disks, _ := pm["disks"].([]interface{})
			var newDisks []interface{}
			for _, d := range disks {
				ds, _ := d.(string)
				if strings.TrimPrefix(ds, "/dev/") != diskName {
					newDisks = append(newDisks, d)
				}
			}
			pm["disks"] = newDisks
			break
		}
	}
	conf["pools"] = confPools
	saveStorageConfigFull(conf)
}

// addDiskToPoolConfig adds a disk to the pool config
func addDiskToPoolConfig(poolName, diskName string) {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, p := range confPools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == poolName {
			disks, _ := pm["disks"].([]interface{})
			disks = append(disks, "/dev/"+diskName)
			pm["disks"] = disks
			break
		}
	}
	conf["pools"] = confPools
	saveStorageConfigFull(conf)
}

// POST /api/storage/pool/attach-disk
// Body: { pool: "valume1", newDisk: "sdc" }
// Adds a disk to a mirror pool to restore redundancy.
func handleAttachDisk(body map[string]interface{}) map[string]interface{} {
	poolName := bodyStr(body, "pool")
	newDisk := bodyStr(body, "newDisk")

	if poolName == "" || newDisk == "" {
		return map[string]interface{}{"error": "Missing pool or newDisk"}
	}

	poolConf, poolType := findPoolConfig(poolName)
	if poolConf == nil {
		return map[string]interface{}{"error": "Pool not found"}
	}

	// Ensure new disk is not already in any pool
	conf := getStorageConfigFull()
	allPools, _ := conf["pools"].([]interface{})
	for _, p := range allPools {
		pm, _ := p.(map[string]interface{})
		pDisks, _ := pm["disks"].([]interface{})
		for _, d := range pDisks {
			ds, _ := d.(string)
			if strings.TrimPrefix(ds, "/dev/") == newDisk {
				pn, _ := pm["name"].(string)
				return map[string]interface{}{"error": fmt.Sprintf("Disk %s is already in pool %s", newDisk, pn)}
			}
		}
	}

	newDiskPath := "/dev/" + newDisk
	if err := preFlightCheck(newDiskPath); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("Disk %s: %s", newDisk, err.Error())}
	}

	switch poolType {
	case "zfs":
		return attachDiskZfs(poolConf, newDisk)
	case "btrfs":
		return attachDiskBtrfs(poolConf, newDisk)
	default:
		return map[string]interface{}{"error": fmt.Sprintf("Unsupported pool type: %s", poolType)}
	}
}

// attachDiskZfs runs: zpool attach <pool> <existing-disk> <new-disk>
func attachDiskZfs(poolConf map[string]interface{}, newDisk string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	zpoolName, _ := poolConf["zpoolName"].(string)
	if zpoolName == "" {
		zpoolName = "nimos-" + poolName
	}

	// Get existing disk from pool
	disks, _ := poolConf["disks"].([]interface{})
	if len(disks) == 0 {
		return map[string]interface{}{"error": "Pool has no disks"}
	}
	existingDisk := strings.TrimPrefix(disks[0].(string), "/dev/")
	existingPart := partitionName("/dev/" + existingDisk)

	opts := CmdOptions{Timeout: 60 * time.Second}
	optsShort := CmdOptions{Timeout: 10 * time.Second}

	// Wipe and partition new disk
	runCmd("wipefs", []string{"-a", "/dev/" + newDisk}, optsShort)
	runCmd("sgdisk", []string{"-Z", "/dev/" + newDisk}, optsShort)
	runCmd("sgdisk", []string{"-n", "1:0:0", "-t", "1:BF01", "/dev/" + newDisk}, opts)
	runCmd("udevadm", []string{"settle", "--timeout=5"}, optsShort)
	time.Sleep(time.Second)

	newPart := partitionName("/dev/" + newDisk)
	waitForDevice(newPart, 10*time.Second)

	// zpool attach — adds the new disk as mirror of existing
	res, err := runCmd("zpool", []string{"attach", "-f", zpoolName, existingPart, newPart}, CmdOptions{Timeout: 30 * time.Second})
	if err != nil || !res.OK {
		errMsg := res.Stderr
		if errMsg == "" {
			errMsg = res.Stdout
		}
		return map[string]interface{}{"error": fmt.Sprintf("zpool attach failed: %s", errMsg)}
	}

	addDiskToPoolConfig(poolName, newDisk)

	addNotification("success", "system",
		fmt.Sprintf("Disco añadido a %s", poolName),
		fmt.Sprintf("Se ha añadido %s al espejo. El resilver reconstruirá la redundancia.", newDisk))

	logMsg("DISK ATTACH: pool %s, added %s (ZFS resilver started)", poolName, newDisk)

	return map[string]interface{}{"ok": true, "message": "Disk attached, resilver started"}
}

// attachDiskBtrfs runs: btrfs device add <new-disk> <mountpoint>
func attachDiskBtrfs(poolConf map[string]interface{}, newDisk string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	mountPoint, _ := poolConf["mountPoint"].(string)

	if mountPoint == "" {
		return map[string]interface{}{"error": "Pool mount point not found"}
	}

	opts := CmdOptions{Timeout: 60 * time.Second}

	runCmd("wipefs", []string{"-a", "/dev/" + newDisk}, opts)

	res, err := runCmd("btrfs", []string{"device", "add", "-f", "/dev/" + newDisk, mountPoint}, opts)
	if err != nil || !res.OK {
		errMsg := res.Stderr
		if errMsg == "" {
			errMsg = res.Stdout
		}
		return map[string]interface{}{"error": fmt.Sprintf("btrfs device add failed: %s", errMsg)}
	}

	// Rebalance in background
	go func() {
		runCmd("btrfs", []string{"balance", "start", "-dconvert=raid1", "-mconvert=raid1", mountPoint}, CmdOptions{Timeout: 0})
		addNotification("success", "system",
			fmt.Sprintf("Rebalanceo completado en %s", poolName),
			fmt.Sprintf("El disco %s se ha integrado completamente.", newDisk))
	}()

	addDiskToPoolConfig(poolName, newDisk)

	addNotification("info", "system",
		fmt.Sprintf("Disco añadido a %s", poolName),
		fmt.Sprintf("Se ha añadido %s. Rebalanceando datos para restaurar redundancia.", newDisk))

	logMsg("DISK ATTACH: pool %s, added %s (BTRFS balance started)", poolName, newDisk)

	return map[string]interface{}{"ok": true, "message": "Disk added, rebalance started"}
}

func handleReplaceDisk(body map[string]interface{}) map[string]interface{} {
	poolName := bodyStr(body, "pool")
	oldDisk := bodyStr(body, "oldDisk")
	newDisk := bodyStr(body, "newDisk")

	if poolName == "" || oldDisk == "" || newDisk == "" {
		return map[string]interface{}{"error": "Missing pool, oldDisk, or newDisk"}
	}

	poolConf, poolType := findPoolConfig(poolName)
	if poolConf == nil {
		return map[string]interface{}{"error": "Pool not found"}
	}

	// Ensure old disk belongs to the pool
	disks, _ := poolConf["disks"].([]interface{})
	found := false
	for _, d := range disks {
		ds, _ := d.(string)
		if strings.TrimPrefix(ds, "/dev/") == oldDisk {
			found = true
			break
		}
	}
	if !found {
		return map[string]interface{}{"error": fmt.Sprintf("Disk %s is not part of pool %s", oldDisk, poolName)}
	}

	// Ensure new disk is not already in a pool
	conf := getStorageConfigFull()
	allPools, _ := conf["pools"].([]interface{})
	for _, p := range allPools {
		pm, _ := p.(map[string]interface{})
		pDisks, _ := pm["disks"].([]interface{})
		for _, d := range pDisks {
			ds, _ := d.(string)
			if strings.TrimPrefix(ds, "/dev/") == newDisk {
				pn, _ := pm["name"].(string)
				return map[string]interface{}{"error": fmt.Sprintf("Disk %s is already in pool %s", newDisk, pn)}
			}
		}
	}

	// ── Service barrier (obligatoria) ──
	// SIEMPRE comprobar justo antes de ejecutar
	activeSvcs, err := checkPoolDependencies(poolName)
	if err != nil {
		logMsg("REPLACE DISK: error checking services for pool %s: %v", poolName, err)
	}
	if len(activeSvcs) > 0 {
		svcNames := make([]string, 0, len(activeSvcs))
		for _, s := range activeSvcs {
			svcNames = append(svcNames, s.AppName)
		}
		return map[string]interface{}{
			"error":    "services_active",
			"message":  fmt.Sprintf("No se puede reemplazar el disco: %d servicios activos en el pool", len(activeSvcs)),
			"services": svcNames,
		}
	}

	// Pre-flight check on new disk
	newDiskPath := "/dev/" + newDisk
	if err := preFlightCheck(newDiskPath); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("New disk %s: %s", newDisk, err.Error())}
	}

	switch poolType {
	case "zfs":
		return replaceDiskZfs(poolConf, oldDisk, newDisk)
	case "btrfs":
		return replaceDiskBtrfs(poolConf, oldDisk, newDisk)
	default:
		return map[string]interface{}{"error": fmt.Sprintf("Unsupported pool type: %s", poolType)}
	}
}

// replaceDiskZfs runs: zpool replace <pool> <old> <new>
func replaceDiskZfs(poolConf map[string]interface{}, oldDisk, newDisk string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	zpoolName, _ := poolConf["zpoolName"].(string)
	if zpoolName == "" {
		zpoolName = "nimos-" + poolName
	}

	newDiskPath := "/dev/" + newDisk
	newPart := partitionName(newDiskPath)
	opts := CmdOptions{Timeout: 60 * time.Second}
	optsShort := CmdOptions{Timeout: 15 * time.Second}

	// Wipe and partition new disk
	runCmd("wipefs", []string{"-a", newDiskPath}, opts)
	runCmd("sgdisk", []string{"-Z", newDiskPath}, optsShort)
	runCmd("sgdisk", []string{"-n", "1:0:0", "-t", "1:BF01", newDiskPath}, opts)
	runCmd("udevadm", []string{"settle", "--timeout=5"}, optsShort)
	time.Sleep(time.Second)
	waitForDevice(newPart, 10*time.Second)

	// Find the old partition in the pool
	oldPart := partitionName("/dev/" + oldDisk)

	// zpool replace — this starts resilver automatically
	res, err := runCmd("zpool", []string{"replace", "-f", zpoolName, oldPart, newPart}, CmdOptions{Timeout: 30 * time.Second})
	if err != nil || !res.OK {
		errMsg := res.Stderr
		if errMsg == "" {
			errMsg = res.Stdout
		}
		return map[string]interface{}{"error": fmt.Sprintf("zpool replace failed: %s", errMsg)}
	}

	// Update config: replace old disk with new
	updatePoolConfigDisk(poolName, oldDisk, newDisk)

	addNotification("info", "system",
		fmt.Sprintf("Reemplazo de disco iniciado en %s", poolName),
		fmt.Sprintf("Reemplazando %s por %s. El resilver puede tardar horas según el tamaño.", oldDisk, newDisk))

	logMsg("DISK REPLACE: pool %s, %s -> %s (ZFS resilver started)", poolName, oldDisk, newDisk)

	return map[string]interface{}{"ok": true, "message": "Resilver started"}
}

// replaceDiskBtrfs runs: btrfs device add + btrfs device delete
func replaceDiskBtrfs(poolConf map[string]interface{}, oldDisk, newDisk string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	mountPoint, _ := poolConf["mountPoint"].(string)

	if mountPoint == "" {
		return map[string]interface{}{"error": "Pool mount point not found"}
	}

	opts := CmdOptions{Timeout: 60 * time.Second}
	newDiskPath := "/dev/" + newDisk
	oldDiskPath := "/dev/" + oldDisk

	// Wipe new disk
	runCmd("wipefs", []string{"-a", newDiskPath}, opts)

	// Add new disk to the filesystem
	res, err := runCmd("btrfs", []string{"device", "add", "-f", newDiskPath, mountPoint}, opts)
	if err != nil || !res.OK {
		errMsg := res.Stderr
		if errMsg == "" {
			errMsg = res.Stdout
		}
		return map[string]interface{}{"error": fmt.Sprintf("btrfs device add failed: %s", errMsg)}
	}

	// Remove old disk — this triggers automatic rebalance
	// Run in background because it can take a very long time
	go func() {
		res, err := runCmd("btrfs", []string{"device", "delete", oldDiskPath, mountPoint}, CmdOptions{Timeout: 0})
		if err == nil && res.OK {
			updatePoolConfigDisk(poolName, oldDisk, newDisk)
			addNotification("success", "system",
				fmt.Sprintf("Disco reemplazado en %s", poolName),
				fmt.Sprintf("Se ha completado el reemplazo de %s por %s.", oldDisk, newDisk))
			logMsg("DISK REPLACE: pool %s, %s -> %s (BTRFS complete)", poolName, oldDisk, newDisk)
		} else {
			errMsg := res.Stderr
			if errMsg == "" && err != nil {
				errMsg = err.Error()
			}
			addNotification("error", "system",
				fmt.Sprintf("Error al reemplazar disco en %s", poolName),
				fmt.Sprintf("No se pudo eliminar %s: %s", oldDisk, errMsg))
			logMsg("DISK REPLACE FAILED: pool %s, btrfs device delete %s: %s", poolName, oldDisk, errMsg)
		}
	}()

	addNotification("info", "system",
		fmt.Sprintf("Reemplazo de disco iniciado en %s", poolName),
		fmt.Sprintf("Añadido %s, eliminando %s. El rebalanceo puede tardar horas.", newDisk, oldDisk))

	logMsg("DISK REPLACE: pool %s, %s -> %s (BTRFS started)", poolName, oldDisk, newDisk)

	return map[string]interface{}{"ok": true, "message": "Disk replacement started"}
}

// updatePoolConfigDisk updates the stored config replacing old disk with new
func updatePoolConfigDisk(poolName, oldDisk, newDisk string) {
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, p := range confPools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == poolName {
			disks, _ := pm["disks"].([]interface{})
			for i, d := range disks {
				ds, _ := d.(string)
				if strings.TrimPrefix(ds, "/dev/") == oldDisk {
					disks[i] = "/dev/" + newDisk
					break
				}
			}
			pm["disks"] = disks
			break
		}
	}
	conf["pools"] = confPools
	saveStorageConfigFull(conf)
}

// getResilverStatus returns the current resilver/rebuild progress
// GET /api/storage/resilver/status?pool=valume1
func getResilverStatus(poolName string) map[string]interface{} {
	poolConf, poolType := findPoolConfig(poolName)
	if poolConf == nil {
		return map[string]interface{}{"error": "Pool not found", "active": false}
	}

	switch poolType {
	case "zfs":
		zpoolName, _ := poolConf["zpoolName"].(string)
		if zpoolName == "" {
			zpoolName = "nimos-" + poolName
		}
		out, ok := runSafe("zpool", "status", zpoolName)
		if !ok {
			return map[string]interface{}{"active": false, "error": "Cannot read pool status"}
		}

		result := map[string]interface{}{
			"active":   false,
			"progress": 0,
			"eta":      "",
			"speed":    "",
		}

		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			// Look for: "scan: resilver in progress since..."
			if strings.Contains(line, "resilver in progress") {
				result["active"] = true
			}
			// Look for progress line: "X.XXM scanned at Y.YYM/s, Z.ZZM issued at W.WWM/s, 1.82T total"
			if strings.Contains(line, "issued") && strings.Contains(line, "total") {
				result["detail"] = line
			}
			// Look for: "X.XX% done, HH:MM:SS to go"
			if strings.Contains(line, "% done") {
				parts := strings.Fields(line)
				for i, p := range parts {
					if p == "done," && i > 0 {
						pctStr := strings.TrimSuffix(parts[i-1], "%")
						pct, _ := strconv.ParseFloat(pctStr, 64)
						result["progress"] = pct
					}
					if p == "go" && i > 0 {
						result["eta"] = parts[i-1]
					}
				}
			}
		}
		return result

	case "btrfs":
		mountPoint, _ := poolConf["mountPoint"].(string)
		out, ok := runSafe("btrfs", "balance", "status", mountPoint)
		if !ok {
			return map[string]interface{}{"active": false}
		}
		active := strings.Contains(out, "in progress") || strings.Contains(out, "running")
		result := map[string]interface{}{
			"active": active,
			"detail": strings.TrimSpace(out),
		}
		// Try to extract percentage
		if active {
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "% done") || strings.Contains(line, "estimated") {
					result["detail"] = strings.TrimSpace(line)
				}
			}
		}
		return result

	default:
		return map[string]interface{}{"active": false, "error": "Unknown pool type"}
	}
}
