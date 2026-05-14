package main

// ═══════════════════════════════════════════════════════════════════════════════
// NimOS Storage — BTRFS Features (Snapshots, Scrub, Scheduler)
//
// Endpoints match the existing UI contract.
//
// Beta 8 note: ZFS support removed in Fase 5. Snapshot/dataset endpoints
// that were ZFS-only are now stubs returning empty/unsupported. They can
// be re-implemented for BTRFS (subvolumes) in Beta 9+ when needed.
// ═══════════════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ─── Core snapshot primitives (used by storage UI and backup) ────────────────

// btrfsSnapshotCreate creates a BTRFS read-only subvolume snapshot.
// source is the path to the subvolume to snapshot, snapPath is the destination.
// Returns ("", nil) on success, ("error message", error) on failure.
func btrfsSnapshotCreate(source, snapPath string) (string, error) {
	opts := CmdOptions{Timeout: 30 * time.Second}
	res, err := runCmd("btrfs", []string{"subvolume", "snapshot", "-r", source, snapPath}, opts)
	if err != nil {
		return res.Stderr, err
	}
	return "", nil
}

// btrfsSnapshotDestroy destroys a BTRFS subvolume snapshot.
func btrfsSnapshotDestroy(snapPath string) (string, error) {
	opts := CmdOptions{Timeout: 30 * time.Second}
	res, err := runCmd("btrfs", []string{"subvolume", "delete", snapPath}, opts)
	if err != nil {
		return res.Stderr, err
	}
	return "", nil
}

// ─── Snapshot endpoints (BTRFS subvolume implementation pending Beta 9) ──────

// listSnapshots returns snapshots for a pool.
// Beta 8: returns empty array. BTRFS snapshot listing via subvolumes is
// pending in Beta 9 (requires walking the subvolume tree).
func listSnapshots(poolName string) map[string]interface{} {
	return map[string]interface{}{"snapshots": []interface{}{}}
}

// createSnapshot stub. Beta 8: not supported.
func createSnapshot(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"ok":    false,
		"error": "snapshot management not yet implemented for BTRFS (pending Beta 9)",
	}
}

// deleteSnapshot stub. Beta 8: not supported.
func deleteSnapshot(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"ok":    false,
		"error": "snapshot management not yet implemented for BTRFS (pending Beta 9)",
	}
}

// rollbackSnapshot stub. Beta 8: not supported.
func rollbackSnapshot(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"ok":    false,
		"error": "snapshot rollback not yet implemented for BTRFS (pending Beta 9)",
	}
}

// ─── SCRUB ───────────────────────────────────────────────────────────────────

// startScrub starts a BTRFS integrity check.
// POST /api/storage/scrub { pool }
func startScrub(body map[string]interface{}) map[string]interface{} {
	pool := bodyStr(body, "pool")

	// Resolve mount point from config
	mountPoint := ""
	conf := getStorageConfigFull()
	confPools, _ := conf["pools"].([]interface{})
	for _, p := range confPools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == pool {
			mountPoint, _ = pm["mountPoint"].(string)
			break
		}
	}
	if mountPoint == "" {
		mountPoint = nimbusPoolsDir + "/" + pool
	}

	if _, err := runCmd("btrfs", []string{"filesystem", "show", mountPoint}, CmdOptions{Timeout: 5 * time.Second}); err != nil {
		return map[string]interface{}{"ok": false, "error": "Pool not found or not a BTRFS filesystem"}
	}

	_, err := runCmd("btrfs", []string{"scrub", "start", mountPoint}, CmdOptions{Timeout: 15 * time.Second})
	if err != nil {
		return map[string]interface{}{"ok": false, "error": fmt.Sprintf("btrfs scrub failed: %s", err)}
	}
	logMsg("BTRFS scrub started on %s", mountPoint)
	addNotification("info", "system", "Verificación iniciada",
		fmt.Sprintf("Verificación de integridad iniciada en volumen %s", pool))
	return map[string]interface{}{"ok": true, "type": "btrfs"}
}

// getScrubStatus returns detailed scrub status for a BTRFS pool.
// GET /api/storage/scrub/status?pool=NAME
func getScrubStatus(poolName string) map[string]interface{} {
	mountPoint := ""
	conf := getStorageConfigFull()
	pools, _ := conf["pools"].([]interface{})
	for _, p := range pools {
		pm, _ := p.(map[string]interface{})
		if n, _ := pm["name"].(string); n == poolName {
			mountPoint, _ = pm["mountPoint"].(string)
			break
		}
	}
	if mountPoint == "" {
		mountPoint = nimbusPoolsDir + "/" + poolName
	}

	if _, err := runCmd("btrfs", []string{"filesystem", "show", mountPoint}, CmdOptions{Timeout: 5 * time.Second}); err != nil {
		return map[string]interface{}{"status": "error", "error": "Pool not found", "filesystem": "unknown"}
	}
	return getBtrfsScrubStatus(mountPoint, poolName)
}

func getBtrfsScrubStatus(mountPoint, poolName string) map[string]interface{} {
	opts := CmdOptions{Timeout: 10 * time.Second}
	res, _ := runCmd("btrfs", []string{"scrub", "status", mountPoint}, opts)
	output := res.Stdout

	result := map[string]interface{}{
		"status":       "idle",
		"progress":     0,
		"errors":       0,
		"duration":     "—",
		"lastScrub":    nil,
		"lastDuration": nil,
		"lastErrors":   nil,
		"dataErrors":   "—",
		"filesystem":   "btrfs",
	}

	if strings.Contains(output, "no stats available") || strings.Contains(output, "not started") {
		result["status"] = "never"
		return result
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch status {
			case "running":
				result["status"] = "scrubbing"
			case "finished":
				result["status"] = "done"
			case "aborted":
				result["status"] = "canceled"
			}
		}

		if strings.HasPrefix(line, "Scrub started:") {
			timeStr := strings.TrimSpace(strings.TrimPrefix(line, "Scrub started:"))
			for _, layout := range []string{
				"Mon Jan  2 15:04:05 2006",
				"Mon Jan 2 15:04:05 2006",
				"2006-01-02 15:04:05",
			} {
				if t, err := time.Parse(layout, timeStr); err == nil {
					result["lastScrub"] = t.Format(time.RFC3339)
					break
				}
			}
		}

		if strings.HasPrefix(line, "Duration:") {
			dur := strings.TrimSpace(strings.TrimPrefix(line, "Duration:"))
			result["duration"] = dur
			result["lastDuration"] = dur
		}

		if strings.HasPrefix(line, "Rate:") {
			result["speed"] = strings.TrimSpace(strings.TrimPrefix(line, "Rate:"))
		}

		if strings.HasPrefix(line, "Error summary:") {
			errStr := strings.TrimSpace(strings.TrimPrefix(line, "Error summary:"))
			result["dataErrors"] = errStr
			if strings.Contains(errStr, "no errors") {
				result["errors"] = 0
				result["lastErrors"] = 0
			} else {
				totalErrs := 0
				for _, part := range strings.Split(errStr, " ") {
					if strings.Contains(part, "=") {
						kv := strings.SplitN(part, "=", 2)
						if len(kv) == 2 {
							n, _ := strconv.Atoi(kv[1])
							totalErrs += n
						}
					}
				}
				result["errors"] = totalErrs
				result["lastErrors"] = totalErrs
			}
		}

		if strings.HasPrefix(line, "Total to scrub:") {
			result["totalSize"] = strings.TrimSpace(strings.TrimPrefix(line, "Total to scrub:"))
		}
	}

	return result
}

// formatDuration converts a time.Duration to "HH:MM:SS"
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// ─── DATASETS (BTRFS subvolumes — pending Beta 9) ────────────────────────────

// listDatasets stub. Beta 8: BTRFS subvolumes will need different listing.
func listDatasets(poolName string) map[string]interface{} {
	return map[string]interface{}{"datasets": []interface{}{}}
}

// ─── SCRUB SCHEDULER ─────────────────────────────────────────────────────────

func initScrubScheduleTable() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS scrub_schedule (
		pool_name TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 0,
		frequency TEXT NOT NULL DEFAULT 'monthly',
		day_of_week INTEGER DEFAULT 0,
		day_of_month INTEGER DEFAULT 1,
		hour INTEGER NOT NULL DEFAULT 3,
		minute INTEGER NOT NULL DEFAULT 0,
		last_run TEXT,
		next_run TEXT,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		logMsg("initScrubScheduleTable: %v", err)
	}
}

func getScrubSchedule(poolName string) map[string]interface{} {
	row := db.QueryRow(`SELECT pool_name, enabled, frequency, day_of_week, day_of_month, hour, minute, last_run, next_run
		FROM scrub_schedule WHERE pool_name = ?`, poolName)

	var (
		name             string
		enabled          int
		freq             string
		dow, dom, h, m   int
		lastRun, nextRun *string
	)
	err := row.Scan(&name, &enabled, &freq, &dow, &dom, &h, &m, &lastRun, &nextRun)
	if err != nil {
		// No schedule set
		return map[string]interface{}{
			"poolName":   poolName,
			"enabled":    false,
			"frequency":  "monthly",
			"dayOfWeek":  0,
			"dayOfMonth": 1,
			"hour":       3,
			"minute":     0,
			"lastRun":    nil,
			"nextRun":    nil,
		}
	}
	return map[string]interface{}{
		"poolName":   name,
		"enabled":    enabled == 1,
		"frequency":  freq,
		"dayOfWeek":  dow,
		"dayOfMonth": dom,
		"hour":       h,
		"minute":     m,
		"lastRun":    derefStringOrNil(lastRun),
		"nextRun":    derefStringOrNil(nextRun),
	}
}

func getAllScrubSchedules() map[string]interface{} {
	rows, err := db.Query(`SELECT pool_name, enabled, frequency, day_of_week, day_of_month, hour, minute, last_run, next_run
		FROM scrub_schedule`)
	if err != nil {
		return map[string]interface{}{"schedules": []interface{}{}}
	}
	defer rows.Close()

	var schedules []interface{}
	for rows.Next() {
		var (
			name             string
			enabled          int
			freq             string
			dow, dom, h, m   int
			lastRun, nextRun *string
		)
		if err := rows.Scan(&name, &enabled, &freq, &dow, &dom, &h, &m, &lastRun, &nextRun); err != nil {
			continue
		}
		schedules = append(schedules, map[string]interface{}{
			"poolName":   name,
			"enabled":    enabled == 1,
			"frequency":  freq,
			"dayOfWeek":  dow,
			"dayOfMonth": dom,
			"hour":       h,
			"minute":     m,
			"lastRun":    derefStringOrNil(lastRun),
			"nextRun":    derefStringOrNil(nextRun),
		})
	}
	if schedules == nil {
		schedules = []interface{}{}
	}
	return map[string]interface{}{"schedules": schedules}
}

func saveScrubSchedule(body map[string]interface{}) map[string]interface{} {
	poolName := bodyStr(body, "poolName")
	if poolName == "" {
		return map[string]interface{}{"ok": false, "error": "poolName is required"}
	}
	enabled := 0
	if b, _ := body["enabled"].(bool); b {
		enabled = 1
	}
	freq := bodyStr(body, "frequency")
	if freq == "" {
		freq = "monthly"
	}
	dow := int(bodyFloat(body, "dayOfWeek", 0))
	dom := int(bodyFloat(body, "dayOfMonth", 1))
	hour := int(bodyFloat(body, "hour", 3))
	minute := int(bodyFloat(body, "minute", 0))

	nextRun := calculateNextRun(freq, hour, minute, dow, dom)

	_, err := db.Exec(`INSERT INTO scrub_schedule (pool_name, enabled, frequency, day_of_week, day_of_month, hour, minute, next_run, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(pool_name) DO UPDATE SET
			enabled = excluded.enabled,
			frequency = excluded.frequency,
			day_of_week = excluded.day_of_week,
			day_of_month = excluded.day_of_month,
			hour = excluded.hour,
			minute = excluded.minute,
			next_run = excluded.next_run,
			updated_at = datetime('now')`,
		poolName, enabled, freq, dow, dom, hour, minute, nextRun)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "nextRun": nextRun}
}

func calculateNextRun(freq string, hour, minute, dow, dom int) interface{} {
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	switch freq {
	case "daily":
		if !target.After(now) {
			target = target.Add(24 * time.Hour)
		}
	case "weekly":
		daysUntil := (dow - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 && !target.After(now) {
			daysUntil = 7
		}
		target = target.AddDate(0, 0, daysUntil)
	case "monthly":
		target = time.Date(now.Year(), now.Month(), dom, hour, minute, 0, 0, now.Location())
		if !target.After(now) {
			target = target.AddDate(0, 1, 0)
		}
	default:
		return nil
	}
	return target.Format(time.RFC3339)
}

func bodyFloat(body map[string]interface{}, key string, def float64) float64 {
	if v, ok := body[key].(float64); ok {
		return v
	}
	return def
}

func derefStringOrNil(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

// startScrubScheduler starts a background goroutine that periodically
// checks the scrub_schedule table and runs scheduled scrubs.
func startScrubScheduler() {
	go func() {
		// Wait a bit on startup so we don't run immediately after boot
		time.Sleep(60 * time.Second)
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			checkAndRunScheduledScrubs()
		}
	}()
	logMsg("Scrub scheduler started (check interval: 60s)")
}

func checkAndRunScheduledScrubs() {
	rows, err := db.Query(`SELECT pool_name, frequency, day_of_week, day_of_month, hour, minute, last_run
		FROM scrub_schedule WHERE enabled = 1`)
	if err != nil {
		return
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var (
			poolName       string
			freq           string
			dow, dom, h, m int
			lastRun        *string
		)
		if err := rows.Scan(&poolName, &freq, &dow, &dom, &h, &m, &lastRun); err != nil {
			continue
		}
		lastRunStr := ""
		if lastRun != nil {
			lastRunStr = *lastRun
		}
		if shouldRunNow(freq, h, m, dow, dom, lastRunStr, now) {
			logMsg("Scrub scheduler: starting scheduled scrub on %s", poolName)
			startScrub(map[string]interface{}{"pool": poolName})
			_, _ = db.Exec(`UPDATE scrub_schedule SET last_run = ?, next_run = ? WHERE pool_name = ?`,
				now.Format(time.RFC3339),
				calculateNextRun(freq, h, m, dow, dom),
				poolName)
		}
	}
}

func shouldRunNow(freq string, hour, minute, dow, dom int, lastRun string, now time.Time) bool {
	// Avoid double-runs within the same minute window
	if lastRun != "" {
		if last, err := time.Parse(time.RFC3339, lastRun); err == nil {
			if now.Sub(last) < 50*time.Second {
				return false
			}
		}
	}
	if now.Hour() != hour || now.Minute() != minute {
		return false
	}
	switch freq {
	case "daily":
		return true
	case "weekly":
		return int(now.Weekday()) == dow
	case "monthly":
		return now.Day() == dom
	}
	return false
}
