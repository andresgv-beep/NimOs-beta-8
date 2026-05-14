package main

// NimOS Storage — Pool info (ZFS + BTRFS), SMART enrichment

import (
	"strings"
)

func enrichDisksWithSmart(diskNames []interface{}) []interface{} {
	smartMu.Lock()
	defer smartMu.Unlock()

	enriched := make([]interface{}, 0, len(diskNames))
	for _, d := range diskNames {
		raw, _ := d.(string)
		if raw == "" {
			continue
		}

		// Strip /dev/ prefix — config stores "/dev/sda", smartHistory uses "sda"
		name := strings.TrimPrefix(raw, "/dev/")

		// Check if disk physically exists
		model := ""
		sizeStr := ""
		diskExists := false
		if out, ok := runSafe("lsblk", "-d", "-n", "-o", "MODEL,SIZE", "/dev/"+name); ok && out != "" {
			diskExists = true
			parts := strings.Fields(strings.TrimSpace(out))
			if len(parts) >= 2 {
				sizeStr = parts[len(parts)-1]
				model = strings.Join(parts[:len(parts)-1], " ")
			} else if len(parts) == 1 {
				sizeStr = parts[0]
			}
		}

		// Determine status
		smartStatus := "unknown"
		if !diskExists {
			smartStatus = "missing"
		} else if s, ok := smartHistory[name]; ok {
			smartStatus = s
		}

		enriched = append(enriched, map[string]interface{}{
			"name":        name,
			"model":       model,
			"size":        sizeStr,
			"smartStatus": smartStatus, // "ok" | "warning" | "critical" | "missing" | "unknown"
		})
	}
	return enriched
}

// ─── BTRFS Pool Info (needed by getStoragePoolsGo) ──────────────────────────

func getBtrfsPoolInfo(poolConf map[string]interface{}, primaryPool string) map[string]interface{} {
	poolName, _ := poolConf["name"].(string)
	mountPoint, _ := poolConf["mountPoint"].(string)
	profile, _ := poolConf["profile"].(string)
	createdAt, _ := poolConf["createdAt"].(string)

	total, used, available := int64(0), int64(0), int64(0)
	poolStatus := "offline"

	// Check if mounted
	mountSrc, _ := runSafe("findmnt", "-n", "-o", "SOURCE", mountPoint)
	if strings.TrimSpace(mountSrc) != "" {
		rootSrc, _ := runSafe("findmnt", "-n", "-o", "SOURCE", "/")
		if strings.TrimSpace(mountSrc) != strings.TrimSpace(rootSrc) {
			poolStatus = "active"
			// Use btrfs filesystem usage for RAID-correct capacity
			// df reports raw capacity which is 2x actual for RAID1
			if bfsOut, ok := runSafe("btrfs", "filesystem", "usage", "-b", mountPoint); ok {
				for _, line := range strings.Split(bfsOut, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Device size:") {
						total = parseInt64(strings.TrimSpace(strings.TrimPrefix(line, "Device size:")))
					} else if strings.HasPrefix(line, "Used:") {
						used = parseInt64(strings.TrimSpace(strings.TrimPrefix(line, "Used:")))
					} else if strings.HasPrefix(line, "Free (estimated):") {
						// Format: "Free (estimated):      1234567890    (min: 123456789)"
						val := strings.TrimSpace(strings.TrimPrefix(line, "Free (estimated):"))
						// Take just the first number (before any parenthetical)
						if idx := strings.Index(val, "("); idx > 0 {
							val = strings.TrimSpace(val[:idx])
						}
						available = parseInt64(val)
					}
				}
			}
			// Fallback to df if btrfs command failed
			if total == 0 {
				if dfOut, ok := runSafe("df", "-B1", "--output=size,used,avail", mountPoint); ok {
					lines := strings.Split(strings.TrimSpace(dfOut), "\n")
					if len(lines) > 1 {
						parts := strings.Fields(lines[1])
						if len(parts) >= 3 {
							total = parseInt64(parts[0])
							used = parseInt64(parts[1])
							available = parseInt64(parts[2])
						}
					}
				}
			}
		}
	}

	// Extract config disk list as []string for health system
	var configDisks []string
	if d, ok := poolConf["disks"].([]interface{}); ok {
		for _, raw := range d {
			if s, ok := raw.(string); ok && s != "" {
				configDisks = append(configDisks, s)
			}
		}
	}

	// Parse per-disk IO errors
	var diskStatuses map[string]DiskStatus
	if poolStatus == "active" {
		diskStatuses, _ = parseBtrfsDeviceStats(mountPoint)
	}

	// Enrich disks with full info
	enrichedDisks := enrichDisksComplete(configDisks, diskStatuses)
	disksForJSON := make([]interface{}, 0, len(enrichedDisks))
	for _, ed := range enrichedDisks {
		disksForJSON = append(disksForJSON, ed.ToMap())
	}

	// Map BTRFS profile to vdev type for health system
	btrfsVdevType := profile
	switch strings.ToLower(profile) {
	case "raid1":
		btrfsVdevType = "mirror"
	case "raid1c3":
		btrfsVdevType = "raidz2" // 3 copies ≈ raidz2 tolerance
	case "raid1c4":
		btrfsVdevType = "raidz3"
	case "raid5":
		btrfsVdevType = "raidz1"
	case "raid6":
		btrfsVdevType = "raidz2"
	}

	// Build pool health
	poolHealth := buildPoolHealth(DiagnosticInput{
		PoolType:    "btrfs",
		VdevType:    btrfsVdevType,
		ConfigDisks: configDisks,
		ZpoolName:   "",
		MountPoint:  mountPoint,
		ZpoolHealth: "",
	})

	usagePct := 0
	if total > 0 {
		usagePct = int(float64(used) / float64(total) * 100)
	}

	return map[string]interface{}{
		"name":               poolName,
		"type":               "btrfs",
		"profile":            profile,
		"mountPoint":         mountPoint,
		"raidLevel":          profile,
		"filesystem":         "btrfs",
		"createdAt":          createdAt,
		"disks":              disksForJSON,
		"status":             poolStatus,
		"total":              total,
		"used":               used,
		"available":          available,
		"totalFormatted":     formatBytes(total),
		"usedFormatted":      formatBytes(used),
		"availableFormatted": formatBytes(available),
		"usagePercent":       usagePct,
		"isPrimary":          poolName == primaryPool,
		"poolHealth":         poolHealth.ToMap(),
	}
}

// ─── HTTP Routes (called from http.go) ───────────────────────────────────────

