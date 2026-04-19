package main

// NimOS Storage — HTTP route handler

import (
	"fmt"
	"net/http"
)

func handleStorageRoutes(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	method := r.Method

	if method == "GET" {
		session := requireAdmin(w, r)
		if session == nil {
			return
		}
		switch urlPath {
		case "/api/storage", "/api/storage/pools":
			jsonOk(w, getStoragePoolsGo())
		case "/api/storage/disks":
			jsonOk(w, detectStorageDisksGo())
		case "/api/storage/status":
			pools := getStoragePoolsGo()
			mountedCount := 0
			for _, p := range pools {
				if s, _ := p["status"].(string); s == "active" || s == "degraded" {
					mountedCount++
				}
			}
			storageAlertsMu.RLock()
			currentAlerts := storageAlertsGo
			storageAlertsMu.RUnlock()
			jsonOk(w, map[string]interface{}{
				"pools":        pools,
				"alerts":       currentAlerts,
				"hasPool":      hasPoolGo(),
				"mountedPools": mountedCount,
				"totalPools":   len(pools),
			})
		case "/api/storage/alerts":
			storageAlertsMu.RLock()
			currentAlerts2 := storageAlertsGo
			storageAlertsMu.RUnlock()
			jsonOk(w, map[string]interface{}{"alerts": currentAlerts2})
		case "/api/storage/capabilities":
			jsonOk(w, map[string]interface{}{
				"zfs":   hasZfs,
				"btrfs": hasBtrfs,
				"arch":  systemArch,
				"ramGB": systemRamGB,
			})
		case "/api/storage/health":
			jsonOk(w, checkStorageHealthGo())
		case "/api/storage/restorable":
			jsonOk(w, map[string]interface{}{"pools": scanForRestorablePoolsGo()})
		case "/api/storage/snapshots":
			pool := r.URL.Query().Get("pool")
			jsonOk(w, listSnapshots(pool))
		case "/api/storage/scrub/status":
			pool := r.URL.Query().Get("pool")
			jsonOk(w, getScrubStatus(pool))
		case "/api/storage/resilver/status":
			pool := r.URL.Query().Get("pool")
			jsonOk(w, getResilverStatus(pool))
		case "/api/storage/datasets":
			pool := r.URL.Query().Get("pool")
			jsonOk(w, listDatasets(pool))
		default:
			jsonError(w, 404, "Not found")
		}
		return
	}

	if method == "POST" || method == "DELETE" || method == "PUT" {
		session := requireAdmin(w, r)
		if session == nil {
			return
		}
		body, _ := readBody(r)

		switch urlPath {
		case "/api/storage/pool":
			poolType := bodyStr(body, "type")
			if poolType == "zfs" || (hasZfs && poolType == "") {
				jsonOk(w, createPoolZfs(body))
			} else if poolType == "btrfs" && hasBtrfs {
				jsonOk(w, createPoolBtrfs(body))
			} else {
				jsonError(w, 400, "No supported filesystem available")
			}
		case "/api/storage/scan":
			rescanSCSIBuses()
			jsonOk(w, map[string]interface{}{"ok": true, "disks": detectStorageDisksGo()})
		case "/api/storage/wipe":
			disk := bodyStr(body, "disk")
			if disk == "" {
				jsonError(w, 400, "Provide disk path")
			} else {
				jsonOk(w, wipeDiskGo(disk))
			}
		case "/api/storage/pool/destroy":
			poolName := bodyStr(body, "name")
			if poolName == "" {
				jsonError(w, 400, "Provide pool name")
			} else {
				conf := getStorageConfigFull()
				confPools, _ := conf["pools"].([]interface{})
				poolType := ""
				for _, p := range confPools {
					pm, _ := p.(map[string]interface{})
					if n, _ := pm["name"].(string); n == poolName {
						poolType, _ = pm["type"].(string)
						break
					}
				}
				switch poolType {
				case "zfs":
					jsonOk(w, destroyPoolZfs(poolName))
				case "btrfs":
					jsonOk(w, destroyPoolBtrfs(poolName))
				default:
					jsonError(w, 400, fmt.Sprintf("Unknown pool type '%s'", poolType))
				}
			}
		case "/api/storage/pool/export":
			poolName := bodyStr(body, "name")
			if poolName == "" {
				jsonError(w, 400, "Provide pool name")
			} else {
				conf := getStorageConfigFull()
				confPools, _ := conf["pools"].([]interface{})
				poolType := ""
				for _, p := range confPools {
					pm, _ := p.(map[string]interface{})
					if n, _ := pm["name"].(string); n == poolName {
						poolType, _ = pm["type"].(string)
						break
					}
				}
				switch poolType {
				case "zfs":
					jsonOk(w, exportPoolZfs(poolName))
				case "btrfs":
					jsonOk(w, exportPoolBtrfs(poolName))
				default:
					jsonError(w, 400, fmt.Sprintf("Unknown pool type '%s'", poolType))
				}
			}
		case "/api/storage/pool/restore":
			jsonOk(w, restorePoolFromIdentity(body))
		case "/api/storage/pool/replace-disk":
			jsonOk(w, handleReplaceDisk(body))
		case "/api/storage/pool/detach-disk":
			jsonOk(w, handleDetachDisk(body))
		case "/api/storage/pool/attach-disk":
			jsonOk(w, handleAttachDisk(body))
		case "/api/storage/pool/resilver-status":
			poolName := bodyStr(body, "pool")
			if poolName == "" {
				jsonError(w, 400, "Provide pool name")
			} else {
				jsonOk(w, getResilverStatus(poolName))
			}
		case "/api/storage/backup":
			backupConfigToPoolGo()
			jsonOk(w, map[string]interface{}{"ok": true})
		case "/api/storage/snapshot":
			if method == "POST" {
				jsonOk(w, createSnapshot(body))
			} else if method == "DELETE" {
				jsonOk(w, deleteSnapshot(body))
			}
		case "/api/storage/snapshot/rollback":
			jsonOk(w, rollbackSnapshot(body))
		case "/api/storage/scrub":
			jsonOk(w, startScrub(body))
		case "/api/storage/dataset":
			if method == "POST" {
				jsonOk(w, createDataset(body))
			} else if method == "DELETE" {
				jsonOk(w, deleteDataset(body))
			}
		default:
			jsonError(w, 404, "Not found")
		}
		return
	}

	jsonError(w, 405, "Method not allowed")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Disk Replacement — Replace a disk in a ZFS or BTRFS pool
// ═══════════════════════════════════════════════════════════════════════════════

// findPoolConfig returns the pool config map by pool name
