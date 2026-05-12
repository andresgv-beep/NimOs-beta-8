// storage_boot.go — Inicialización del módulo storage Beta 8.
//
// Centraliza el arranque del nuevo stack (Repo + Policy) en una sola
// función llamada desde main.go tras initStorageSchema().
//
// Orden de arranque del módulo:
//   1. openDB              ← db.go (PRAGMA foreign_keys + WAL)
//   2. createTables        ← db.go (tablas legacy de Beta 7)
//   3. migrateFromJSON     ← db.go (compatibilidad con JSON viejo)
//   4. initStorageSchema   ← storage_schema.go (tablas storage_* Beta 8)
//   5. initStorageModule   ← este archivo (Repo + Policy listos)
//
// Tras este punto:
//   - storageRepo es la instancia global del StorageRepo
//   - storagePolicy es la instancia global del PolicyChecker
//   - Ambos pueden usarse desde cualquier parte del daemon
//
// see docs/storage_invariants.md
// see docs/storage_api.md §2 (capa de servicio)

package main

import (
	"context"
	"fmt"
)

// initStorageModule inicializa el módulo de storage Beta 8.
// Debe llamarse DESPUÉS de initStorageSchema() (tablas creadas) y ANTES
// de cualquier código que use storageRepo o storagePolicy.
//
// Crea los singletons globales del módulo y verifica que están operativos
// con una query de comprobación (defensive: si la conexión está rota
// queremos saberlo aquí, no en el primer request HTTP).
func initStorageModule() error {
	if db == nil {
		return fmt.Errorf("initStorageModule: db is nil (call openDB first)")
	}

	// Crear singletons globales.
	initStorageRepo()
	initStoragePolicy()

	// Verificación defensiva: leer global_generation. Si esto falla,
	// algo está mal con la conexión o con el schema.
	gen, err := storageRepo.GetGlobalGeneration(context.Background())
	if err != nil {
		return fmt.Errorf("initStorageModule: cannot read global_generation: %w", err)
	}

	logMsg("Storage module ready (global_generation=%d)", gen)
	return nil
}
