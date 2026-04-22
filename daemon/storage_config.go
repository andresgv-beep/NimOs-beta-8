package main

// NimOS Storage — Config, pool queries, JSON helpers

import (
	"encoding/json"
	"os"
	"sync"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const nimbusPoolsDir = "/nimbus/pools"
// storageConfigFile is declared in shares.go

// ─── Global vars ─────────────────────────────────────────────────────────────

var hasBtrfs bool
// hasZfs is declared in hardware.go
var storageAlertsGo []map[string]interface{}

// LOGIC-001/002: Mutex for storage.json read/write and storageAlertsGo
var storageConfigMu sync.RWMutex
var storageAlertsMu sync.RWMutex

// ─── Config read/write (needed by docker.go, shares.go) ─────────────────────

func getStorageConfigFull() map[string]interface{} {
	storageConfigMu.RLock()
	defer storageConfigMu.RUnlock()
	data, err := os.ReadFile(storageConfigFile)
	if err != nil {
		return map[string]interface{}{"pools": []interface{}{}, "primaryPool": nil}
	}
	var conf map[string]interface{}
	if json.Unmarshal(data, &conf) != nil {
		return map[string]interface{}{"pools": []interface{}{}, "primaryPool": nil}
	}
	return conf
}

func saveStorageConfigFull(config map[string]interface{}) {
	storageConfigMu.Lock()
	defer storageConfigMu.Unlock()
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(storageConfigFile, data, 0644)
}

// ─── Pool queries (needed by various files) ──────────────────────────────────

func hasPoolGo() bool {
	conf := getStorageConfigFull()
	pools, _ := conf["pools"].([]interface{})
	if len(pools) == 0 {
		return false
	}
	// Verify at least one pool is actually mounted
	for _, poolRaw := range pools {
		pm, _ := poolRaw.(map[string]interface{})
		if mp, _ := pm["mountPoint"].(string); mp != "" {
			if isPathOnMountedPool(mp) {
				return true
			}
		}
	}
	return false
}

func getStoragePoolsGo() []map[string]interface{} {
	conf := getStorageConfigFull()
	var pools []map[string]interface{}
	confPools, _ := conf["pools"].([]interface{})
	primaryPool, _ := conf["primaryPool"].(string)

	for _, poolRaw := range confPools {
		poolConf, _ := poolRaw.(map[string]interface{})
		if poolConf == nil {
			continue
		}
		poolType, _ := poolConf["type"].(string)
		switch poolType {
		case "zfs":
			pools = append(pools, getZfsPoolInfo(poolConf, primaryPool))
		case "btrfs":
			pools = append(pools, getBtrfsPoolInfo(poolConf, primaryPool))
		}
	}
	if pools == nil {
		pools = []map[string]interface{}{}
	}
	return pools
}

// ─── JSON helpers (used across storage) ──────────────────────────────────────

func jsonToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		return parseInt64(val)
	case json.Number:
		n, _ := val.Int64()
		return n
	}
	return 0
}

func jsonToBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "1" || val == "true"
	case float64:
		return val == 1
	}
	return false
}

