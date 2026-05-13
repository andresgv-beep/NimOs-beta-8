// storage_service.go — Capa de orquestación del módulo storage (Beta 8).
//
// StorageService es la ÚNICA capa que ejecuta operaciones. Coordina:
//   - StorageRepo (persistencia en SQLite)
//   - PolicyChecker (validación de permisos)
//   - BtrfsExecutor (operaciones reales sobre BTRFS) ← futuro Bloque 2+
//
// Patrón de cada método:
//   1. Verificar policy (¿el caller puede hacer esto?)
//   2. Crear Operation en DB con status pending/in_progress
//   3. Ejecutar la acción física (BTRFS o metadata-only)
//   4. Persistir resultado y marcar Operation completed/failed
//   5. Devolver la Operation al caller
//
// Todo dentro de transacción SQLite cuando toca múltiples tablas.
//
// see docs/storage_invariants.md
// see docs/storage_api.md §4 para firmas completas

package main

import (
	"context"
	"database/sql"
	"fmt"
)

// ═════════════════════════════════════════════════════════════════════════════
// StorageService
// ═════════════════════════════════════════════════════════════════════════════

// StorageService es la capa de orquestación. Recibe dependencias por
// constructor para facilitar tests con mocks.
type StorageService struct {
	repo   *StorageRepo
	policy *PolicyChecker
	db     *sql.DB // necesario para iniciar transacciones
	// btrfs BtrfsExecutor — se añadirá en Bloque 2
}

// NewStorageService crea el servicio con sus dependencias inyectadas.
func NewStorageService(db *sql.DB, repo *StorageRepo, policy *PolicyChecker) *StorageService {
	return &StorageService{
		repo:   repo,
		policy: policy,
		db:     db,
	}
}

// Instancia global, conveniente para código que aún no usa inyección.
var storageService *StorageService

// initStorageService crea la instancia global. Llamar tras
// initStorageRepo() y initStoragePolicy().
func initStorageService() {
	storageService = NewStorageService(db, storageRepo, storagePolicy)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers compartidos para los métodos del service
// ─────────────────────────────────────────────────────────────────────────────

// runInTx ejecuta fn dentro de una transacción. Si fn devuelve error,
// hace rollback automático. Si fn devuelve nil, hace commit.
//
// Centralizar este patrón evita repetir BeginTx/defer Rollback/Commit
// en cada método del service.
func (s *StorageService) runInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("BeginTx: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// checkPolicy es helper que valida y devuelve error semántico si no permite.
// Centraliza el patrón "policy.Allows + error con código".
func (s *StorageService) checkPolicy(pool *Pool, op OperationType) error {
	allowed, code := s.policy.AllowsWithReason(pool, op)
	if !allowed {
		return &ServiceError{
			Code: code,
			Msg:  fmt.Sprintf("operation %s not permitted on pool %s", op, pool.ID),
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ServiceError — error con código semántico
// ─────────────────────────────────────────────────────────────────────────────

// ServiceError es el error que devuelven los métodos del service cuando
// fallan por una razón identificable. El handler HTTP puede leer el Code
// y devolver el código HTTP correcto al frontend.
type ServiceError struct {
	Code string // ErrCode* (ver storage_types.go)
	Msg  string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

// errFromCode devuelve un ServiceError con el código y mensaje dados.
func errFromCode(code, msg string) error {
	return &ServiceError{Code: code, Msg: msg}
}

// ═════════════════════════════════════════════════════════════════════════════
// Queries síncronas — proyecciones que el frontend pedirá
// ═════════════════════════════════════════════════════════════════════════════

// ListPools devuelve todos los pools con sus devices cargados.
// Esta es la query principal que el frontend usa para mostrar el estado.
func (s *StorageService) ListPools(ctx context.Context) ([]*Pool, error) {
	pools, err := s.repo.ListPools(ctx)
	if err != nil {
		return nil, err
	}

	// Hidratar cada pool con sus devices y capabilities
	for _, p := range pools {
		devices, err := s.repo.ListDevicesInPool(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("ListPools: hydrate devices for %s: %w", p.ID, err)
		}
		p.Devices = make([]Device, len(devices))
		for i, d := range devices {
			p.Devices[i] = *d
		}

		caps, err := s.repo.GetPoolCapabilities(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("ListPools: hydrate caps for %s: %w", p.ID, err)
		}
		p.Capabilities = caps
	}

	return pools, nil
}

// GetPool devuelve un pool por su ID con devices y capabilities hidratados.
func (s *StorageService) GetPool(ctx context.Context, id string) (*Pool, error) {
	pool, err := s.repo.GetPool(ctx, id)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, errFromCode(ErrCodePoolNotFound,
			fmt.Sprintf("pool %s not found", id))
	}

	devices, err := s.repo.ListDevicesInPool(ctx, pool.ID)
	if err != nil {
		return nil, err
	}
	pool.Devices = make([]Device, len(devices))
	for i, d := range devices {
		pool.Devices[i] = *d
	}

	caps, err := s.repo.GetPoolCapabilities(ctx, pool.ID)
	if err != nil {
		return nil, err
	}
	pool.Capabilities = caps

	return pool, nil
}

// ListDevices devuelve todos los devices del sistema.
func (s *StorageService) ListDevices(ctx context.Context) ([]*Device, error) {
	return s.repo.ListDevices(ctx)
}

// ListAvailableDevices devuelve devices libres (no asignados a pool).
func (s *StorageService) ListAvailableDevices(ctx context.Context) ([]*Device, error) {
	return s.repo.ListAvailableDevices(ctx)
}

// ListOperations devuelve operaciones del journal según filtro.
// Útil para el activity timeline del frontend.
func (s *StorageService) ListOperations(ctx context.Context, f OperationFilter) ([]*Operation, error) {
	return s.repo.ListOperations(ctx, f)
}

// GetGeneration devuelve el contador global de mutaciones.
// El frontend puede usarlo para detectar si algo cambió antes de re-fetch.
func (s *StorageService) GetGeneration(ctx context.Context) (int64, error) {
	return s.repo.GetGlobalGeneration(ctx)
}

// ═════════════════════════════════════════════════════════════════════════════
// Mutaciones síncronas — metadata-only (sin BTRFS)
// ═════════════════════════════════════════════════════════════════════════════
//
// Estas operaciones solo modifican metadata en SQLite, sin tocar el
// filesystem real. Generan Operation con status=completed inmediato.
// see docs/storage_api.md §4.2

// RenamePool cambia el nombre legible de un pool.
// Síncrona. El id interno no cambia. Las shares siguen funcionando.
func (s *StorageService) RenamePool(ctx context.Context, id, newName string) (*Operation, error) {
	pool, err := s.repo.GetPool(ctx, id)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, errFromCode(ErrCodePoolNotFound, fmt.Sprintf("pool %s not found", id))
	}

	if err := s.checkPolicy(pool, OpTypeRenamePool); err != nil {
		return nil, err
	}

	// Verificar que el nombre no esté tomado
	other, err := s.repo.GetPoolByName(ctx, newName)
	if err != nil {
		return nil, err
	}
	if other != nil && other.ID != pool.ID {
		return nil, errFromCode(ErrCodePoolNameTaken,
			fmt.Sprintf("pool name %q already in use", newName))
	}

	// Crear Operation + ejecutar dentro de la misma tx
	op := &Operation{
		ID:     newUUID(),
		Type:   OpTypeRenamePool,
		PoolID: &pool.ID,
		Status: OpStatusInProgress,
		Data:   rawJSON(map[string]string{"from": pool.Name, "to": newName}),
	}

	err = s.runInTx(ctx, func(tx *sql.Tx) error {
		if err := s.repo.CreateOperation(ctx, tx, op); err != nil {
			return err
		}
		if err := s.repo.RenamePool(ctx, tx, pool.ID, newName); err != nil {
			return err
		}
		return s.repo.UpdateOperationStatus(ctx, tx, op.ID, OpStatusCompleted, nil, nil)
	})
	if err != nil {
		return nil, err
	}

	// Recargar la operation con su completed_at actualizado
	return s.repo.GetOperation(ctx, op.ID)
}

// SetPoolCompression cambia la compresión de un pool.
// Síncrona. Solo afecta a archivos escritos a partir del cambio.
func (s *StorageService) SetPoolCompression(ctx context.Context, id, algorithm string) (*Operation, error) {
	// Usar GetPool del service (no del repo) porque hidrata capabilities,
	// que policy necesita para validar la op.
	pool, err := s.GetPool(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.checkPolicy(pool, OpTypeSetCompression); err != nil {
		return nil, err
	}

	op := &Operation{
		ID:     newUUID(),
		Type:   OpTypeSetCompression,
		PoolID: &pool.ID,
		Status: OpStatusInProgress,
		Data:   rawJSON(map[string]string{"from": pool.Compression, "to": algorithm}),
	}

	err = s.runInTx(ctx, func(tx *sql.Tx) error {
		if err := s.repo.CreateOperation(ctx, tx, op); err != nil {
			return err
		}
		if err := s.repo.SetPoolCompression(ctx, tx, pool.ID, algorithm); err != nil {
			return err
		}
		return s.repo.UpdateOperationStatus(ctx, tx, op.ID, OpStatusCompleted, nil, nil)
	})
	if err != nil {
		return nil, err
	}

	return s.repo.GetOperation(ctx, op.ID)
}

// ═════════════════════════════════════════════════════════════════════════════
// Stubs de mutaciones async (Bloque 2+)
// ═════════════════════════════════════════════════════════════════════════════
//
// Estos métodos están declarados como esqueleto. Su implementación real
// llegará cuando integremos BtrfsExecutor (Bloque 2 en adelante).
// De momento devuelven "not implemented" para que el código que los llame
// falle explícitamente en vez de silenciosamente.

// CreatePoolRequest es el payload de CreatePool.
type CreatePoolRequest struct {
	Name      string   `json:"name"`
	Profile   Profile  `json:"profile"`
	DeviceIDs []string `json:"device_ids"` // serials o IDs internos
}

// CreatePool crea un nuevo pool BTRFS con los devices indicados.
// Asíncrona. Genera Operation con status=pending → in_progress → completed/failed.
// IMPLEMENTACIÓN PENDIENTE — Bloque 2.
func (s *StorageService) CreatePool(ctx context.Context, req CreatePoolRequest) (*Operation, error) {
	return nil, errFromCode(ErrCodeInternal, "CreatePool: not yet implemented (Bloque 2)")
}

// DestroyPool destruye un pool BTRFS y libera sus devices.
// IMPLEMENTACIÓN PENDIENTE — Bloque 2.
func (s *StorageService) DestroyPool(ctx context.Context, poolID string) (*Operation, error) {
	return nil, errFromCode(ErrCodeInternal, "DestroyPool: not yet implemented (Bloque 2)")
}
