<script>
  /**
   * StorageApp · Gestión de almacenamiento (v3 · Fase A MVP)
   * ────────────────────────────────────────────────────────────
   * Migración desde Beta 7 siguiendo mockup "nimos-storage-retro.html".
   *
   * Scope Fase A:
   *   - Listar pools (ZFS + BTRFS) con info rica
   *   - Pantalla inteligente: banner si hay pools restaurables sin montar
   *   - Restaurar pool existente (flujo crítico · tu caso)
   *   - Ver discos con SMART status
   *   - Snapshots (listar)
   *   - Scrub manual
   *   - Escaneo de discos
   *
   * Fase B (pendiente):
   *   - Crear pool nuevo (+ wizard selector vdev)
   *   - Add/remove/replace disco
   *   - Exportar/destruir pool
   *   - Snapshots (crear/rollback/borrar)
   *   - Datasets
   *
   * TODO backend (gaps actuales):
   *   - Temperatura disco y horas operación (no se exponen en JSON)
   *   - Breakdown por categoría para donut (solo total used/available)
   *   - Rol disco (data/parity) en RAIDZ (inferible en frontend)
   *
   * Backend endpoints (sin cambios, ya existen en Beta 7):
   *   GET  /api/storage/pools
   *   GET  /api/storage/status
   *   GET  /api/storage/disks
   *   GET  /api/storage/restorable
   *   GET  /api/storage/alerts
   *   GET  /api/storage/capabilities
   *   GET  /api/storage/snapshots?pool=X
   *   GET  /api/storage/scrub/status?pool=X
   *   POST /api/storage/scan
   *   POST /api/storage/scrub
   *   POST /api/storage/pool/restore
   *   POST /api/storage/pool/export
   *   POST /api/storage/pool/destroy
   */
  import { onMount, onDestroy } from 'svelte';
  import { token, hdrs } from '$lib/stores/auth.js';
  import AppShell from '$lib/components/AppShell.svelte';
  import {
    KPICard, SectionHead, BevelButton, IconButton,
    LED, EmptyState, Spinner, Badge, StripeProgressBar
  } from '$lib/ui';

  // ─── State ───
  let active = 'overview'; // 'overview' | 'disks' | 'snapshots' | 'restore' | 'scrub' | 'smart'

  let pools = [];
  let disks = {};
  let alerts = [];
  let capabilities = {};
  let restorablePools = [];
  let status = {};

  // Expanded pools en vista overview
  let expandedPools = new Set();
  // Menu kebab abierto para un pool (solo uno a la vez)
  let kebabOpenFor = null;

  // Restaurar pool
  let restoring = {};
  let restoreMsg = '';
  let restoreMsgError = false;

  // Scrub
  let scrubbing = {};
  let scrubMsg = '';

  // Scan
  let scanning = false;

  // Snapshots (por pool)
  let snapshots = {};

  let loading = true;
  let pollInterval;

  // ─── Derived ───
  $: hasPools = pools.length > 0;
  $: hasRestorable = restorablePools.length > 0;
  $: showRestoreBanner = !hasPools && hasRestorable;
  $: totalDisksAssigned = pools.reduce((s, p) => s + (p.disks?.length || 0), 0);
  $: totalDisksFree = (disks.eligible?.length || 0);
  $: totalCapacity = pools.reduce((s, p) => s + (p.total || 0), 0);
  $: totalUsed = pools.reduce((s, p) => s + (p.used || 0), 0);
  $: totalFree = totalCapacity - totalUsed;
  $: overallHealth = pools.every(p => p.status === 'active') && alerts.length === 0 ? 'ok'
                   : pools.some(p => p.status === 'offline' || p.status === 'faulted') ? 'crit'
                   : 'warn';
  $: overallUsagePct = totalCapacity > 0 ? Math.round((totalUsed / totalCapacity) * 100) : 0;

  // ─── API ───
  async function loadAll() {
    try {
      const [poolsRes, statusRes, disksRes, alertsRes, capsRes, restorableRes] = await Promise.all([
        fetch('/api/storage/pools',        { headers: hdrs() }).then(r => r.json()).catch(() => []),
        fetch('/api/storage/status',       { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/storage/disks',        { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/storage/alerts',       { headers: hdrs() }).then(r => r.json()).catch(() => ({ alerts: [] })),
        fetch('/api/storage/capabilities', { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/storage/restorable',   { headers: hdrs() }).then(r => r.json()).catch(() => ({ pools: [] })),
      ]);
      pools = Array.isArray(poolsRes) ? poolsRes : [];
      status = statusRes || {};
      disks = disksRes || {};
      alerts = alertsRes?.alerts || [];
      capabilities = capsRes || {};
      restorablePools = restorableRes?.pools || [];
    } catch (e) {
      console.error('[StorageApp] loadAll failed', e);
    }
    loading = false;
  }

  async function loadSnapshots(poolName) {
    if (!poolName) return;
    try {
      const res = await fetch(`/api/storage/snapshots?pool=${encodeURIComponent(poolName)}`, {
        headers: hdrs(),
      });
      const data = await res.json();
      snapshots[poolName] = data?.snapshots || [];
      snapshots = snapshots;
    } catch {}
  }

  // ─── Scan ───
  async function rescanDisks() {
    scanning = true;
    try {
      await fetch('/api/storage/scan', { method: 'POST', headers: hdrs() });
      await loadAll();
    } catch {}
    scanning = false;
  }

  // ─── Pool expand/collapse ───
  function togglePoolExpand(poolName) {
    kebabOpenFor = null;
    if (expandedPools.has(poolName)) {
      expandedPools.delete(poolName);
    } else {
      expandedPools.add(poolName);
      loadSnapshots(poolName);
    }
    expandedPools = expandedPools;
  }

  function toggleKebab(poolName, event) {
    if (event) event.stopPropagation();
    kebabOpenFor = kebabOpenFor === poolName ? null : poolName;
  }

  // ─── Restore pool ───
  async function restorePool(restorable) {
    const key = restorable.zpoolName;
    restoring[key] = true;
    restoring = restoring;
    restoreMsg = '';
    try {
      const res = await fetch('/api/storage/pool/restore', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          zpoolName: restorable.zpoolName,
          name: restorable.name,
          restoreConfig: restorable.hasBackup,
        }),
      });
      const data = await res.json();
      if (data?.ok !== false) {
        restoreMsg = `Pool "${restorable.name}" restaurado correctamente`;
        restoreMsgError = false;
        await loadAll();
        active = 'overview';
      } else {
        restoreMsg = data.error || 'Error al restaurar';
        restoreMsgError = true;
      }
    } catch (e) {
      restoreMsg = 'Error de conexión durante la restauración';
      restoreMsgError = true;
    }
    restoring[key] = false;
    restoring = restoring;
  }

  // ─── Scrub ───
  async function startScrub(poolName) {
    if (!confirm(`¿Iniciar scrub del pool "${poolName}"? El sistema puede ir más lento mientras corre.`)) return;
    scrubbing[poolName] = true;
    scrubbing = scrubbing;
    scrubMsg = '';
    try {
      const res = await fetch('/api/storage/scrub', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ pool: poolName }),
      });
      const data = await res.json();
      if (data?.ok !== false) {
        scrubMsg = `Scrub iniciado en "${poolName}"`;
      } else {
        scrubMsg = data.error || 'Error al iniciar scrub';
      }
      await loadAll();
    } catch {
      scrubMsg = 'Error de conexión';
    }
    scrubbing[poolName] = false;
    scrubbing = scrubbing;
    kebabOpenFor = null;
  }

  // ─── Helpers ───
  function fmtBytes(b) {
    if (!b || b === 0) return '0 B';
    if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB';
    if (b >= 1e9)  return (b / 1e9).toFixed(1)  + ' GB';
    if (b >= 1e6)  return (b / 1e6).toFixed(0)  + ' MB';
    if (b >= 1e3)  return (b / 1e3).toFixed(0)  + ' KB';
    return b + ' B';
  }

  function fmtDate(iso) {
    if (!iso) return '—';
    try {
      const d = new Date(iso);
      return d.toLocaleDateString('es-ES') + ' ' + d.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' });
    } catch { return iso; }
  }

  // Inferir rol del disco en un pool según vdevType
  // (visual solo · en ZFS parity está distribuida, pero para UI es más claro)
  function inferDiskRole(disks, idx, vdevType) {
    const v = (vdevType || '').toLowerCase();
    const n = disks.length;
    if (v === 'raidz' || v === 'raidz1') return idx === n - 1 ? 'parity' : 'data';
    if (v === 'raidz2') return idx >= n - 2 ? 'parity' : 'data';
    if (v === 'raidz3') return idx >= n - 3 ? 'parity' : 'data';
    if (v === 'mirror') return 'mirror';
    return 'data';
  }

  function usageVariant(pct) {
    if (pct >= 90) return 'crit';
    if (pct >= 75) return 'warn';
    return 'ok';
  }

  function ledVariantForHealth(health) {
    const h = (health || '').toUpperCase();
    if (h === 'ONLINE' || h === 'OK') return 'ok';
    if (h === 'DEGRADED') return 'warn';
    if (h === 'FAULTED' || h === 'UNAVAIL' || h === 'OFFLINE') return 'crit';
    return 'off';
  }

  function smartVariant(smartStatus) {
    if (smartStatus === 'ok')       return 'ok';
    if (smartStatus === 'warning')  return 'warn';
    if (smartStatus === 'critical') return 'crit';
    if (smartStatus === 'missing')  return 'crit';
    return 'off';
  }

  // ─── Click-outside listener para kebab ───
  function onDocClick() {
    kebabOpenFor = null;
  }

  // ─── Lifecycle ───
  onMount(async () => {
    let attempts = 0;
    while (!$token && attempts < 10) { await new Promise(r => setTimeout(r, 200)); attempts++; }
    await loadAll();
    pollInterval = setInterval(loadAll, 20000); // 20s · storage es lento de cambiar
    document.addEventListener('click', onDocClick);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
    document.removeEventListener('click', onDocClick);
  });
</script>

<AppShell
  appId="storage"
  title="Storage"
  headerIcon="S"
  pathSegments={['storage', active]}
  sections={[
    {
      label: 'Volúmenes',
      items: [
        { id: 'overview',  label: 'Resumen',    keyHint: '1', badge: pools.length },
        { id: 'disks',     label: 'Discos',     keyHint: '2', badge: totalDisksAssigned + totalDisksFree },
        { id: 'snapshots', label: 'Snapshots',  keyHint: '3' },
        { id: 'restore',   label: 'Restaurar',  keyHint: '4',
          badge: restorablePools.length > 0 ? restorablePools.length : null,
          badgeVariant: 'warn' },
      ],
    },
    {
      label: 'Herramientas',
      items: [
        { id: 'scrub', label: 'Scrub', keyHint: 'S' },
        { id: 'smart', label: 'SMART', keyHint: 'M' },
      ],
    },
  ]}
  bind:active
>

  {#if loading}
    <div class="storage-loading">
      <Spinner label="Cargando volúmenes y discos..." />
    </div>
  {:else}

  <!-- Banner inteligente: pools restaurables cuando no hay pools configurados -->
  {#if showRestoreBanner && active !== 'restore'}
    <div class="restore-banner">
      <LED size={8} variant="warn" />
      <div class="rb-body">
        <div class="rb-title">
          <b>{restorablePools.length}</b> pool{restorablePools.length > 1 ? 's' : ''} sin montar
          encontrado{restorablePools.length > 1 ? 's' : ''} en el sistema
        </div>
        <div class="rb-hint">
          Tus pools existen pero no están registrados en NimOS. Restáuralos para recuperar datos y configuración.
        </div>
      </div>
      <BevelButton variant="primary" size="sm" onClick={() => active = 'restore'}>
        ▸ Ver pools restaurables
      </BevelButton>
    </div>
  {/if}

  <!-- Summary bar · SOLO en vista Resumen -->
  {#if active === 'overview'}
  <div class="st-kpis">
    <KPICard
      label="Volúmenes"
      value={String(pools.length)}
      unit=""
      state={pools.length > 0 ? 'online' : 'vacío'}
      stateVariant={pools.length > 0 ? 'ok' : 'warn'}
      valueVariant={pools.length > 0 ? 'accent' : 'default'}
      bracketVariant={pools.length > 0 ? 'accent' : 'warn'}
    />
    <KPICard
      label="Discos"
      value={String(totalDisksAssigned + totalDisksFree)}
      unit=""
      state={`${totalDisksAssigned} asignados · ${totalDisksFree} libres`}
      stateVariant="ok"
      valueVariant="default"
    />
    <KPICard
      label="Capacidad"
      value={fmtBytes(totalCapacity)}
      unit=""
      state={totalCapacity > 0 ? `${fmtBytes(totalFree)} libres · ${overallUsagePct}%` : '—'}
      stateVariant={usageVariant(overallUsagePct)}
      valueVariant={usageVariant(overallUsagePct) === 'crit' ? 'crit' : usageVariant(overallUsagePct) === 'warn' ? 'warn' : 'default'}
      bracketVariant={usageVariant(overallUsagePct) === 'crit' ? 'crit' : 'accent'}
    />
    <KPICard
      label="Salud"
      value={overallHealth === 'ok' ? 'OK' : overallHealth === 'warn' ? 'WARN' : 'CRIT'}
      unit=""
      state={alerts.length === 0 ? 'sin incidencias' : `${alerts.length} alerta${alerts.length > 1 ? 's' : ''}`}
      stateVariant={overallHealth}
      valueVariant={overallHealth === 'crit' ? 'crit' : overallHealth === 'warn' ? 'warn' : 'accent'}
      bracketVariant={overallHealth === 'crit' ? 'crit' : overallHealth === 'warn' ? 'warn' : 'accent'}
    />
  </div>
  {/if}

  <!-- ═══════ CONTENT PRINCIPAL ═══════ -->
  <div class="st-scroll">

    <!-- ══ RESUMEN (OVERVIEW) ══ -->
    {#if active === 'overview'}
      <div class="st-section">
        <div class="section-row">
          <SectionHead count={pools.length > 0 ? `· ${pools.length} activos` : ''}>
            Volúmenes
          </SectionHead>
          <div class="section-actions">
            <BevelButton size="sm" onClick={rescanDisks} disabled={scanning}>
              {scanning ? '▸ Escaneando...' : '↻ Escanear'}
            </BevelButton>
            <BevelButton variant="primary" size="sm" disabled>
              + Nuevo volumen <span class="tc-mute">(Fase B)</span>
            </BevelButton>
          </div>
        </div>

        {#if pools.length === 0}
          <EmptyState
            icon="◇"
            title="Sin volúmenes configurados"
            hint={hasRestorable
              ? `Se detectaron ${restorablePools.length} pools sin montar. Ve a la pestaña Restaurar.`
              : 'Crea un volumen nuevo o restaura uno existente para empezar.'}
          />
        {:else}
          <!-- Lista de pools -->
          <div class="pools">
            {#each pools as pool (pool.name)}
              <div
                class="pool"
                class:open={expandedPools.has(pool.name)}
                class:degraded={pool.status === 'degraded' || pool.health === 'DEGRADED'}
                class:crit={pool.status === 'offline' || pool.status === 'faulted' || pool.health === 'FAULTED'}
              >
                <!-- Pool header -->
                <div class="pool-head" on:click={() => togglePoolExpand(pool.name)}
                     on:keydown={(e) => e.key === 'Enter' && togglePoolExpand(pool.name)}
                     role="button" tabindex="0">
                  <div class="pool-head-icon">◆</div>
                  <div class="pool-ident">
                    <div class="pool-name">
                      {pool.name}
                      {#if pool.isPrimary}
                        <Badge size="sm" variant="accent">primary</Badge>
                      {/if}
                    </div>
                    <div class="pool-meta">
                      {(pool.type || 'zfs').toUpperCase()} · {pool.vdevType || 'single'} ·
                      {pool.disks?.length || 0} disco{pool.disks?.length === 1 ? '' : 's'} ·
                      {fmtBytes(pool.used)} usados
                    </div>
                  </div>
                  <div class="pool-bar-wrap">
                    <StripeProgressBar
                      value={pool.usagePct || 0}
                      variant={usageVariant(pool.usagePct || 0)}
                      showLabel={true}
                    />
                  </div>
                  <div class="pool-size">{fmtBytes(pool.total)}</div>
                  <div class="pool-status">
                    <LED size={8} variant={ledVariantForHealth(pool.health)} />
                  </div>
                  <div class="pool-chev" class:rot={expandedPools.has(pool.name)}>›</div>

                  <button
                    class="pool-kebab"
                    class:active={kebabOpenFor === pool.name}
                    on:click={(e) => toggleKebab(pool.name, e)}
                    title="Acciones"
                  >⋮</button>
                </div>

                <!-- Toolbar inline de acciones (sustituye al dropdown flotante) -->
                {#if kebabOpenFor === pool.name}
                  <div class="pool-actions-bar" on:click|stopPropagation>
                    <button class="pa-btn" disabled title="Disponible en Fase B">
                      <span class="pa-num">01</span>
                      <span>Snapshot</span>
                      <span class="pa-tag">Fase B</span>
                    </button>
                    <button
                      class="pa-btn"
                      on:click={() => startScrub(pool.name)}
                      disabled={scrubbing[pool.name]}
                    >
                      <span class="pa-num">02</span>
                      <span>{scrubbing[pool.name] ? 'Iniciando scrub...' : 'Scrub ahora'}</span>
                    </button>
                    <button class="pa-btn" disabled title="Disponible en Fase B">
                      <span class="pa-num">03</span>
                      <span>Añadir disco</span>
                      <span class="pa-tag">Fase B</span>
                    </button>
                    <div class="pa-sep"></div>
                    <button class="pa-btn" disabled title="Disponible en Fase B">
                      <span class="pa-num">04</span>
                      <span>Exportar</span>
                      <span class="pa-tag">Fase B</span>
                    </button>
                    <button class="pa-btn danger" disabled title="Disponible en Fase B">
                      <span class="pa-num">DEL</span>
                      <span>Destruir</span>
                      <span class="pa-tag">Fase B</span>
                    </button>
                  </div>
                {/if}

                <!-- Pool expanded body -->
                {#if expandedPools.has(pool.name)}
                  <div class="pool-body">

                    <!-- Info grid (reemplaza donut hasta tener backend) -->
                    <div class="pool-info-grid">
                      <div class="pig-col">
                        <div class="pig-label">Total</div>
                        <div class="pig-value">{fmtBytes(pool.total)}</div>
                      </div>
                      <div class="pig-col">
                        <div class="pig-label">Usado</div>
                        <div class="pig-value tc-accent">{fmtBytes(pool.used)}</div>
                      </div>
                      <div class="pig-col">
                        <div class="pig-label">Libre</div>
                        <div class="pig-value">{fmtBytes(pool.available)}</div>
                      </div>
                      <div class="pig-col">
                        <div class="pig-label">Uso</div>
                        <div class="pig-value" class:warn={pool.usagePct > 75} class:crit={pool.usagePct > 90}>
                          {pool.usagePct || 0}%
                        </div>
                      </div>
                      <div class="pig-col">
                        <div class="pig-label">Health</div>
                        <div class="pig-value">
                          <LED size={7} variant={ledVariantForHealth(pool.health)} />
                          <span>{pool.health || '—'}</span>
                        </div>
                      </div>
                      <div class="pig-col">
                        <div class="pig-label">Mount</div>
                        <div class="pig-value mono sm">{pool.mountPoint || '—'}</div>
                      </div>
                    </div>

                    <!-- Disk table -->
                    <div class="pool-disks">
                      <div class="pd-head">
                        Discos del volumen · {pool.disks?.length || 0}
                        <span class="tc-mute todo">
                          (temp y horas pendiente backend)
                        </span>
                      </div>
                      <div class="disk-table cols-6-pool">
                        <div class="disk-thead">
                          <div></div>
                          <div>Modelo</div>
                          <div>Dispositivo</div>
                          <div>Capacidad</div>
                          <div>Rol</div>
                          <div>SMART</div>
                        </div>
                        {#each (pool.disks || []) as disk, i}
                          <div class="disk-row">
                            <div class="disk-idx">D{i + 1}</div>
                            <div class="disk-cell mono">{disk.model || '—'}</div>
                            <div class="disk-cell mono">/dev/{disk.name}</div>
                            <div class="disk-cell">{disk.size || '—'}</div>
                            <div class="disk-cell">
                              <Badge size="sm" variant={inferDiskRole(pool.disks, i, pool.vdevType) === 'parity' ? 'warn' : 'default'}>
                                {inferDiskRole(pool.disks, i, pool.vdevType)}
                              </Badge>
                            </div>
                            <div class="disk-cell">
                              <LED size={7} variant={smartVariant(disk.smartStatus)} />
                              <span class="tc-dim sm">{disk.smartStatus || 'unknown'}</span>
                            </div>
                          </div>
                        {/each}
                      </div>
                    </div>

                    <!-- Snapshots (si hay, solo ZFS) -->
                    {#if (pool.type === 'zfs' || pool.filesystem === 'zfs') && snapshots[pool.name]?.length > 0}
                      <div class="pool-snapshots">
                        <div class="pd-head">
                          Snapshots · {snapshots[pool.name].length}
                        </div>
                        <div class="snap-list">
                          {#each snapshots[pool.name].slice(0, 5) as snap}
                            <div class="snap-row">
                              <span class="mono">{snap.name || snap}</span>
                              {#if snap.used}
                                <span class="tc-mute">{fmtBytes(snap.used)}</span>
                              {/if}
                              {#if snap.created}
                                <span class="tc-mute">{fmtDate(snap.created)}</span>
                              {/if}
                            </div>
                          {/each}
                          {#if snapshots[pool.name].length > 5}
                            <div class="snap-more">
                              <span class="tc-mute">+ {snapshots[pool.name].length - 5} más · ver pestaña Snapshots</span>
                            </div>
                          {/if}
                        </div>
                      </div>
                    {/if}

                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}

        {#if scrubMsg}
          <div class="msg">{scrubMsg}</div>
        {/if}
      </div>

      <!-- Alertas globales -->
      {#if alerts.length > 0}
        <div class="st-section">
          <SectionHead count="· {alerts.length}">Alertas del sistema</SectionHead>
          <div class="alerts-list">
            {#each alerts as alert}
              <div class="alert-row" class:crit={alert.level === 'critical'} class:warn={alert.level === 'warning'}>
                <LED size={7} variant={alert.level === 'critical' ? 'crit' : 'warn'} />
                <div class="alert-body">
                  <div class="alert-msg">{alert.message}</div>
                  {#if alert.pool}
                    <div class="alert-meta">
                      pool: <span class="mono">{alert.pool}</span> ·
                      {fmtDate(alert.timestamp)}
                    </div>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}

    <!-- ══ DISCOS ══ -->
    {#if active === 'disks'}
      <div class="st-section">
        <div class="section-row">
          <SectionHead count={`· ${totalDisksAssigned + totalDisksFree} detectados`}>
            Discos del sistema
          </SectionHead>
          <div class="section-actions">
            <BevelButton size="sm" onClick={rescanDisks} disabled={scanning}>
              {scanning ? '▸ Escaneando...' : '↻ Rescan buses'}
            </BevelButton>
          </div>
        </div>

        <!-- Discos asignados a pools -->
        {#if totalDisksAssigned > 0}
          <SectionHead count={`· ${totalDisksAssigned}`}>Asignados a pools</SectionHead>
          <div class="disk-table cols-5-disk">
            <div class="disk-thead">
              <div>Dispositivo</div>
              <div>Modelo</div>
              <div>Capacidad</div>
              <div>Pool</div>
              <div>SMART</div>
            </div>
            {#each pools as pool}
              {#each (pool.disks || []) as disk}
                <div class="disk-row">
                  <div class="disk-cell mono">/dev/{disk.name}</div>
                  <div class="disk-cell mono">{disk.model || '—'}</div>
                  <div class="disk-cell">{disk.size || '—'}</div>
                  <div class="disk-cell">
                    <Badge size="sm" variant="accent">{pool.name}</Badge>
                  </div>
                  <div class="disk-cell">
                    <LED size={7} variant={smartVariant(disk.smartStatus)} />
                    <span class="tc-dim sm">{disk.smartStatus || 'unknown'}</span>
                  </div>
                </div>
              {/each}
            {/each}
          </div>
        {/if}

        <!-- Discos libres -->
        <div style="margin-top:24px">
          <SectionHead count={`· ${disks.eligible?.length || 0}`}>Discos libres (elegibles)</SectionHead>
          {#if !disks.eligible || disks.eligible.length === 0}
            <EmptyState icon="◌" title="Sin discos libres" hint="Todos los discos están asignados a pools" />
          {:else}
            <div class="disk-table cols-5-disk">
              <div class="disk-thead">
                <div>Dispositivo</div>
                <div>Modelo</div>
                <div>Capacidad</div>
                <div>Tipo</div>
                <div>Estado</div>
              </div>
              {#each disks.eligible as disk}
                <div class="disk-row">
                  <div class="disk-cell mono">{disk.path || '/dev/' + disk.name}</div>
                  <div class="disk-cell mono">{disk.model || '—'}</div>
                  <div class="disk-cell">{disk.sizeH || fmtBytes(disk.size)}</div>
                  <div class="disk-cell">
                    <Badge size="sm" variant={disk.rotational ? 'default' : 'info'}>
                      {disk.rotational ? 'HDD' : 'SSD'}
                    </Badge>
                  </div>
                  <div class="disk-cell">
                    <Badge size="sm" variant="accent">disponible</Badge>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- USB si hay -->
        {#if disks.usb?.length > 0}
          <div style="margin-top:24px">
            <SectionHead count={`· ${disks.usb.length}`}>Dispositivos USB</SectionHead>
            <div class="disk-table cols-5-disk">
              <div class="disk-thead">
                <div>Dispositivo</div>
                <div>Modelo</div>
                <div>Capacidad</div>
                <div>Tipo</div>
                <div>Estado</div>
              </div>
              {#each disks.usb as disk}
                <div class="disk-row">
                  <div class="disk-cell mono">{disk.path || '/dev/' + disk.name}</div>
                  <div class="disk-cell mono">{disk.model || '—'}</div>
                  <div class="disk-cell">{disk.sizeH || fmtBytes(disk.size)}</div>
                  <div class="disk-cell"><Badge size="sm" variant="warn">USB</Badge></div>
                  <div class="disk-cell"><Badge size="sm">externo</Badge></div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- ══ SNAPSHOTS ══ -->
    {#if active === 'snapshots'}
      <div class="st-section">
        <SectionHead>Snapshots</SectionHead>

        {#if pools.length === 0}
          <EmptyState icon="◇" title="Sin pools configurados" hint="Crea o restaura un pool ZFS para gestionar snapshots" />
        {:else}
          {#each pools.filter(p => p.type === 'zfs' || p.filesystem === 'zfs') as pool}
            <div class="snap-block">
              <div class="snap-block-head">
                <div class="pool-head-icon sm">◆</div>
                <span class="mono">{pool.name}</span>
                {#if !snapshots[pool.name]}
                  <BevelButton size="sm" onClick={() => loadSnapshots(pool.name)}>Cargar</BevelButton>
                {/if}
                <div style="flex:1"></div>
                <BevelButton variant="primary" size="sm" disabled>
                  + Crear snapshot <span class="tc-mute">(Fase B)</span>
                </BevelButton>
              </div>

              {#if snapshots[pool.name]}
                {#if snapshots[pool.name].length === 0}
                  <EmptyState icon="◌" title="Sin snapshots" hint={`No hay snapshots en "${pool.name}"`} />
                {:else}
                  <div class="disk-table cols-4-snap">
                    <div class="disk-thead">
                      <div>Nombre</div>
                      <div>Usado</div>
                      <div>Creado</div>
                      <div>Acciones</div>
                    </div>
                    {#each snapshots[pool.name] as snap}
                      <div class="disk-row">
                        <div class="disk-cell mono">{snap.name || snap}</div>
                        <div class="disk-cell">{snap.used ? fmtBytes(snap.used) : '—'}</div>
                        <div class="disk-cell">{fmtDate(snap.created)}</div>
                        <div class="disk-cell">
                          <IconButton size="sm" title="Rollback" disabled>↺</IconButton>
                          <IconButton size="sm" variant="danger" title="Eliminar" disabled>×</IconButton>
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              {/if}
            </div>
          {/each}

          {#if pools.filter(p => p.type === 'zfs' || p.filesystem === 'zfs').length === 0}
            <EmptyState icon="!" title="Sin pools ZFS" hint="Los snapshots solo están disponibles en pools ZFS. Tus pools son BTRFS." />
          {/if}
        {/if}
      </div>
    {/if}

    <!-- ══ RESTAURAR ══ -->
    {#if active === 'restore'}
      <div class="st-section">
        <SectionHead count={restorablePools.length > 0 ? `· ${restorablePools.length}` : ''}>
          Pools restaurables
        </SectionHead>

        {#if restorablePools.length === 0}
          <EmptyState
            icon="◌"
            title="Sin pools para restaurar"
            hint="No se han encontrado pools sin montar con identidad NimOS (.nimbus-pool.json)"
          />
        {:else}
          <div class="restorable-list">
            {#each restorablePools as rp}
              <div class="restorable-card">
                <div class="rc-head">
                  <div class="rc-icon">◆</div>
                  <div class="rc-ident">
                    <div class="rc-name">{rp.name}</div>
                    <div class="rc-meta">
                      {(rp.type || 'zfs').toUpperCase()} · {rp.vdevType || 'single'} ·
                      <span class="mono">{rp.zpoolName}</span>
                    </div>
                  </div>
                  <div class="rc-size">
                    <div class="rc-size-val">{rp.size || '—'}</div>
                    <div class="rc-size-sub">
                      <LED size={6} variant={ledVariantForHealth(rp.health)} />
                      {rp.health}
                    </div>
                  </div>
                </div>

                <div class="rc-features">
                  {#if rp.hasBackup}
                    <div class="rc-feat ok">
                      <LED size={6} variant="ok" />
                      <span>Incluye config backup de NimOS</span>
                    </div>
                  {/if}
                  {#if rp.hasDocker}
                    <div class="rc-feat">
                      <LED size={6} variant="ok" />
                      <span>Incluye datos de Docker</span>
                    </div>
                  {/if}
                  {#if rp.shares?.length > 0}
                    <div class="rc-feat">
                      <LED size={6} variant="ok" />
                      <span>{rp.shares.length} shares · {rp.shares.join(', ')}</span>
                    </div>
                  {/if}
                  <div class="rc-feat mute">
                    <span class="tc-mute mono">mount point:</span>
                    <span class="mono">{rp.mountPoint}</span>
                  </div>
                </div>

                <div class="rc-actions">
                  <BevelButton
                    variant="primary"
                    size="sm"
                    onClick={() => restorePool(rp)}
                    disabled={restoring[rp.zpoolName]}
                  >
                    {restoring[rp.zpoolName] ? '▸ Restaurando...' : '▸ Restaurar este pool'}
                  </BevelButton>
                </div>
              </div>
            {/each}
          </div>
        {/if}

        {#if restoreMsg}
          <div class="msg" class:error={restoreMsgError}>{restoreMsg}</div>
        {/if}
      </div>
    {/if}

    <!-- ══ SCRUB ══ -->
    {#if active === 'scrub'}
      <div class="st-section">
        <SectionHead>Scrub manual</SectionHead>

        {#if pools.length === 0}
          <EmptyState icon="◇" title="Sin pools" hint="No hay pools para ejecutar scrub" />
        {:else}
          <div class="hint-box">
            <b>¿Qué es scrub?</b> Es un chequeo de integridad que recorre todos los datos del pool
            y verifica checksums. Útil mensualmente para detectar errores silenciosos.
            Puede tardar horas y el sistema irá más lento mientras corre.
          </div>

          <div class="disk-table cols-5-scrub">
            <div class="disk-thead">
              <div>Pool</div>
              <div>Tipo</div>
              <div>Tamaño</div>
              <div>Último scrub</div>
              <div>Acción</div>
            </div>
            {#each pools as pool}
              <div class="disk-row">
                <div class="disk-cell mono">{pool.name}</div>
                <div class="disk-cell">{(pool.type || 'zfs').toUpperCase()}</div>
                <div class="disk-cell">{fmtBytes(pool.total)}</div>
                <div class="disk-cell tc-mute">—</div>
                <div class="disk-cell">
                  <BevelButton
                    size="sm"
                    onClick={() => startScrub(pool.name)}
                    disabled={scrubbing[pool.name]}
                  >
                    {scrubbing[pool.name] ? '▸ Iniciando...' : '▸ Scrub ahora'}
                  </BevelButton>
                </div>
              </div>
            {/each}
          </div>

          {#if scrubMsg}
            <div class="msg">{scrubMsg}</div>
          {/if}
        {/if}
      </div>
    {/if}

    <!-- ══ SMART ══ -->
    {#if active === 'smart'}
      <div class="st-section">
        <SectionHead>SMART de discos</SectionHead>

        <div class="hint-box">
          <b>SMART</b> (Self-Monitoring, Analysis and Reporting Technology) es una tecnología
          que permite a los discos auto-diagnosticarse. Un SMART status <span class="tc-accent">ok</span>
          significa que el disco no reporta errores. <span class="tc-warn">warning</span> y
          <span class="tc-crit">critical</span> requieren atención.
        </div>

        {#if pools.length === 0 && (!disks.eligible || disks.eligible.length === 0)}
          <EmptyState icon="◌" title="Sin discos" hint="No hay discos detectados en el sistema" />
        {:else}
          <div class="disk-table cols-6-smart">
            <div class="disk-thead">
              <div>Dispositivo</div>
              <div>Modelo</div>
              <div>Capacidad</div>
              <div>Pool</div>
              <div>SMART</div>
              <div>Notas</div>
            </div>
            {#each pools as pool}
              {#each (pool.disks || []) as disk}
                <div class="disk-row">
                  <div class="disk-cell mono">/dev/{disk.name}</div>
                  <div class="disk-cell mono">{disk.model || '—'}</div>
                  <div class="disk-cell">{disk.size || '—'}</div>
                  <div class="disk-cell"><Badge size="sm" variant="accent">{pool.name}</Badge></div>
                  <div class="disk-cell">
                    <LED size={7} variant={smartVariant(disk.smartStatus)} />
                    <span class="sm">{disk.smartStatus || 'unknown'}</span>
                  </div>
                  <div class="disk-cell tc-mute sm">
                    {#if disk.smartStatus === 'critical'}Reemplazar cuanto antes
                    {:else if disk.smartStatus === 'warning'}Monitorizar
                    {:else if disk.smartStatus === 'missing'}Disco desconectado
                    {:else if disk.smartStatus === 'ok'}Sin incidencias
                    {:else}—{/if}
                  </div>
                </div>
              {/each}
            {/each}
          </div>

          <div class="todo-note">
            <b>TODO</b> · temperatura, horas de operación y errores detallados pendientes de añadir al backend.
          </div>
        {/if}
      </div>
    {/if}

  </div>
  {/if}

  <!-- Footer -->
  <svelte:fragment slot="footer">
    <span><span class="k">pools</span> <span class="v">{pools.length}</span></span>
    <span class="sep">·</span>
    <span><span class="k">disks</span> <span class="v">{totalDisksAssigned + totalDisksFree}</span></span>
    <span class="sep">·</span>
    <span><span class="k">zfs</span> <span class="v" class:tc-accent={capabilities.zfs}>{capabilities.zfs ? 'available' : 'n/a'}</span></span>
    <span class="sep">·</span>
    <span><span class="k">btrfs</span> <span class="v" class:tc-accent={capabilities.btrfs}>{capabilities.btrfs ? 'available' : 'n/a'}</span></span>
  </svelte:fragment>

  <svelte:fragment slot="footer-right">
    <span><span class="k">usage</span> <span class="v" class:tc-accent={overallUsagePct < 75}>{overallUsagePct}%</span></span>
  </svelte:fragment>

</AppShell>

<style>
  /* Loading ───── */
  .storage-loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 280px;
  }

  /* Restore banner ───── */
  .restore-banner {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 12px 20px;
    background: rgba(255, 184, 0, 0.06);
    border-bottom: 1px solid var(--warn);
    flex-shrink: 0;
    font-family: var(--font-mono);
  }
  .rb-body {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .rb-title {
    font-size: 11px;
    color: var(--fg);
    letter-spacing: 0.5px;
  }
  .rb-title b { color: var(--warn); font-weight: 700; }
  .rb-hint {
    font-size: 10px;
    color: var(--fg-dim);
    letter-spacing: 0.3px;
  }

  /* KPIs ───── */
  .st-kpis {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex-shrink: 0;
  }
  .st-kpis :global(.kpi) { border-right: 1px solid var(--border); }
  .st-kpis :global(.kpi:last-child) { border-right: none; }

  /* Main scroll ───── */
  .st-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 22px 28px 24px;
    display: flex;
    flex-direction: column;
    gap: 26px;
  }
  .st-section {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .section-row {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .section-actions {
    display: flex;
    gap: 8px;
    margin-left: auto;
  }

  /* Pool card ───── */
  .pools {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .pool {
    background: var(--bg-1);
    border: 1px solid var(--border);
    font-family: var(--font-mono);
    transition: border-color 0.12s;
  }
  .pool.open { border-color: var(--border-bright); }
  .pool.degraded { border-left: 2px solid var(--warn); }
  .pool.crit { border-left: 2px solid var(--crit); }

  .pool-head {
    display: grid;
    grid-template-columns: 24px 1fr 220px 80px 18px 18px 24px;
    gap: 16px;
    align-items: center;
    padding: 12px 16px;
    cursor: pointer;
    user-select: none;
  }
  .pool-head:hover { background: var(--bg-2); }

  .pool-head-icon {
    color: var(--accent);
    font-size: 14px;
    text-align: center;
  }
  .pool-head-icon.sm { font-size: 11px; }

  .pool-ident {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .pool-name {
    font-size: 13px;
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 0.3px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .pool-meta {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.3px;
  }

  .pool-bar-wrap { min-width: 0; }
  .pool-size {
    font-size: 11px;
    color: var(--fg);
    text-align: right;
    font-feature-settings: "tnum";
  }
  .pool-status {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .pool-chev {
    color: var(--fg-mute);
    font-size: 14px;
    transition: transform 0.15s;
    text-align: center;
  }
  .pool-chev.rot { transform: rotate(90deg); color: var(--accent); }

  .pool-kebab {
    width: 24px;
    height: 24px;
    background: transparent;
    border: none;
    color: var(--fg-mute);
    cursor: pointer;
    font-size: 14px;
    font-family: var(--font-mono);
    transition: color 0.12s;
  }
  .pool-kebab:hover { color: var(--accent); }

  /* Kebab button · ahora con estado active cuando se abre la toolbar */
  .pool-kebab.active {
    color: var(--accent);
    background: var(--bg-2);
  }

  /* Toolbar inline de acciones · aparece bajo el pool-head cuando se pulsa kebab */
  .pool-actions-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    padding: 10px 16px;
    background: var(--bg-2);
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    font-family: var(--font-mono);
    animation: pab-in 0.15s ease-out;
  }
  @keyframes pab-in {
    from { opacity: 0; max-height: 0; padding-top: 0; padding-bottom: 0; }
    to   { opacity: 1; max-height: 60px; padding-top: 10px; padding-bottom: 10px; }
  }

  .pa-btn {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 6px 10px;
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    font-family: inherit;
    font-size: 10px;
    letter-spacing: 0.3px;
    cursor: pointer;
    transition: all 0.1s;
    clip-path: polygon(
      0 0, calc(100% - 5px) 0, 100% 5px,
      100% 100%, 5px 100%, 0 calc(100% - 5px)
    );
  }
  .pa-btn:not(:disabled):hover {
    border-color: var(--accent);
    color: var(--accent);
    background: var(--bg-1);
  }
  .pa-btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }
  .pa-btn.danger:not(:disabled):hover {
    border-color: var(--crit);
    color: var(--crit);
    background: rgba(255,90,90,0.04);
  }
  .pa-num {
    color: var(--fg-faint);
    font-size: 9px;
    min-width: 22px;
  }
  .pa-tag {
    color: var(--fg-faint);
    font-size: 8px;
    letter-spacing: 0.8px;
    text-transform: uppercase;
    margin-left: 2px;
  }
  .pa-sep {
    width: 1px;
    height: 18px;
    background: var(--border-bright);
    margin: 0 6px;
  }

  /* Pool body ───── */
  .pool-body {
    border-top: 1px solid var(--border);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    background: var(--bg);
  }

  .pool-info-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 1px;
    background: var(--border);
    border: 1px solid var(--border);
  }
  .pig-col {
    background: var(--bg-1);
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .pig-label {
    font-size: 9px;
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1.2px;
  }
  .pig-value {
    font-size: 12px;
    color: var(--fg);
    font-weight: 600;
    font-feature-settings: "tnum";
    display: flex;
    align-items: center;
    gap: 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pig-value.mono { font-family: var(--font-mono); }
  .pig-value.sm { font-size: 10px; }
  .pig-value.warn { color: var(--warn); }
  .pig-value.crit { color: var(--crit); }

  /* Disk table ───── */
  .pool-disks { }
  .pd-head {
    font-size: 10px;
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1.3px;
    margin-bottom: 8px;
    padding: 0 2px;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .pd-head .todo {
    font-size: 9px;
    text-transform: none;
    letter-spacing: 0.3px;
  }

  .disk-table {
    display: flex;
    flex-direction: column;
    gap: 1px;
    background: var(--border);
    border: 1px solid var(--border);
  }
  .disk-thead, .disk-row {
    display: grid;
    gap: 10px;
    padding: 8px 12px;
    background: var(--bg-1);
    align-items: center;
  }

  /* Grids por variante de tabla — selectores directos y explícitos */

  /* 6 col · Discos dentro de pool expandido (D1 icon, modelo, dev, cap, rol, SMART) */
  .disk-table.cols-6-pool .disk-thead,
  .disk-table.cols-6-pool .disk-row {
    grid-template-columns: 40px 1fr 110px 80px 80px 140px;
  }

  /* 5 col · Discos asignados / libres / USB (dev, modelo, cap, pool-o-tipo, estado) */
  .disk-table.cols-5-disk .disk-thead,
  .disk-table.cols-5-disk .disk-row {
    grid-template-columns: 130px 1fr 100px 120px 130px;
  }

  /* 5 col · Scrub (pool, tipo, tamaño, last scrub, acción) */
  .disk-table.cols-5-scrub .disk-thead,
  .disk-table.cols-5-scrub .disk-row {
    grid-template-columns: 1fr 80px 100px 140px 160px;
  }

  /* 4 col · Snapshots (nombre, usado, creado, acciones) */
  .disk-table.cols-4-snap .disk-thead,
  .disk-table.cols-4-snap .disk-row {
    grid-template-columns: 1fr 90px 160px 90px;
  }

  /* 6 col · SMART (dev, modelo, cap, pool, SMART, notas) */
  .disk-table.cols-6-smart .disk-thead,
  .disk-table.cols-6-smart .disk-row {
    grid-template-columns: 130px 1fr 90px 100px 130px 1fr;
  }

  .disk-thead {
    font-size: 9px;
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1.2px;
    background: var(--bg-2);
  }
  .disk-row {
    font-size: 11px;
    color: var(--fg);
    font-feature-settings: "tnum";
  }
  .disk-idx {
    color: var(--accent);
    font-weight: 700;
    font-size: 11px;
  }
  .disk-cell {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .disk-cell.mono { font-family: var(--font-mono); }

  /* Snapshots list ───── */
  .snap-list {
    display: flex;
    flex-direction: column;
    gap: 1px;
    background: var(--border);
    border: 1px solid var(--border);
  }
  .snap-row {
    padding: 6px 12px;
    background: var(--bg-1);
    display: flex;
    align-items: center;
    gap: 14px;
    font-size: 10px;
  }
  .snap-more {
    padding: 6px 12px;
    background: var(--bg-2);
    font-size: 10px;
    text-align: center;
  }

  .snap-block {
    margin-bottom: 20px;
  }
  .snap-block-head {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
    margin-bottom: 10px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  /* Restorable cards ───── */
  .restorable-list {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .restorable-card {
    background: var(--bg-1);
    border: 1px solid var(--warn);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    box-shadow: 0 0 10px rgba(255,184,0,0.05);
    font-family: var(--font-mono);
    clip-path: polygon(
      0 0, 100% 0, 100% calc(100% - 10px),
      calc(100% - 10px) 100%, 0 100%
    );
  }
  .rc-head {
    display: grid;
    grid-template-columns: 24px 1fr auto;
    gap: 14px;
    align-items: center;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }
  .rc-icon {
    color: var(--warn);
    font-size: 16px;
  }
  .rc-ident { display: flex; flex-direction: column; gap: 3px; }
  .rc-name {
    font-size: 14px;
    color: var(--fg);
    font-weight: 700;
    letter-spacing: 0.5px;
  }
  .rc-meta {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.3px;
  }
  .rc-size { text-align: right; }
  .rc-size-val {
    font-size: 14px;
    color: var(--fg);
    font-weight: 600;
    font-feature-settings: "tnum";
  }
  .rc-size-sub {
    font-size: 9px;
    color: var(--fg-dim);
    text-transform: uppercase;
    letter-spacing: 1px;
    display: flex;
    align-items: center;
    gap: 5px;
    justify-content: flex-end;
    margin-top: 2px;
  }

  .rc-features {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .rc-feat {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--fg-dim);
  }
  .rc-feat.ok { color: var(--fg); }
  .rc-feat.mute { color: var(--fg-mute); font-size: 10px; }

  .rc-actions {
    display: flex;
    gap: 8px;
    padding-top: 10px;
    border-top: 1px solid var(--border);
  }

  /* Alerts ───── */
  .alerts-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .alert-row {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-left: 2px solid var(--fg-mute);
    font-family: var(--font-mono);
  }
  .alert-row.warn { border-left-color: var(--warn); background: rgba(255,184,0,0.04); }
  .alert-row.crit { border-left-color: var(--crit); background: rgba(255,90,90,0.04); }
  .alert-body {
    display: flex;
    flex-direction: column;
    gap: 3px;
    flex: 1;
    min-width: 0;
  }
  .alert-msg {
    font-size: 11px;
    color: var(--fg);
    letter-spacing: 0.3px;
  }
  .alert-meta {
    font-size: 9px;
    color: var(--fg-mute);
  }

  /* Hint box ───── */
  .hint-box {
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-left: 2px solid var(--info);
    padding: 10px 14px;
    font-family: var(--font-sans);
    font-size: 11px;
    color: var(--fg-dim);
    line-height: 1.5;
    margin-bottom: 14px;
  }
  .hint-box b { color: var(--fg); font-weight: 600; }

  .todo-note {
    margin-top: 14px;
    padding: 8px 12px;
    background: rgba(255,184,0,0.04);
    border: 1px dashed var(--warn);
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--warn);
  }
  .todo-note b { letter-spacing: 1px; }

  /* Messages ───── */
  .msg {
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 8px 12px;
    background: rgba(0, 255, 159, 0.04);
    border-left: 2px solid var(--accent);
    color: var(--accent);
    margin-top: 10px;
  }
  .msg.error {
    background: rgba(255, 90, 90, 0.04);
    border-left-color: var(--crit);
    color: var(--crit);
  }

  /* Utility ───── */
  .mono { font-family: var(--font-mono); }
  .sm { font-size: 10px; }
  .tc-accent { color: var(--accent); }
  .tc-warn { color: var(--warn); }
  .tc-crit { color: var(--crit); }
  .tc-mute { color: var(--fg-mute); }
  .tc-dim { color: var(--fg-dim); }
  .k { color: var(--fg-faint); }
  .v { color: var(--fg-dim); font-feature-settings: "tnum"; }
  .sep { color: var(--fg-faint); }
</style>
