<script>
  /**
   * CreatePoolWizard · Wizard to create a new storage pool
   * ─────────────────────────────────────────────────────────
   * 4 pasos: tipo → discos → nombre → confirmación.
   *
   * Filosofía: solo layouts seguros recomendados. El usuario NO elige layout,
   * se calcula automáticamente según tipo y número de discos seleccionados.
   *   ZFS:   1 disk → single, 2 → mirror, 3 → raidz1, 4+ → raidz2
   *   BTRFS: 1 disk → single, 2 → raid1, 3 → raid1, 4+ → raid10
   *
   * Backend:
   *   POST /api/storage/pool { type, name, vdevType|profile, disks: [paths] }
   *
   * Validación de nombre idéntica a la del backend:
   *   ^[a-zA-Z0-9-]{1,32}$ + reserved list
   *
   * Usage:
   *   <CreatePoolWizard
   *     capabilities={{ zfs: true, btrfs: true }}
   *     eligibleDisks={disks.eligible || []}
   *     on:done
   *     on:cancel
   *   />
   */
  import { createEventDispatcher } from 'svelte';
  import { jsonHdrs } from '$lib/stores/auth.js';
  import WizardModal from '$lib/ui/WizardModal.svelte';
  import LED from '$lib/ui/LED.svelte';
  import Badge from '$lib/ui/Badge.svelte';

  export let capabilities = { zfs: false, btrfs: false };
  export let eligibleDisks = [];

  const dispatch = createEventDispatcher();

  // ─── State ───
  let step = 1;                // 1 = tipo · 2 = discos · 3 = nombre · 4 = confirmar
  let fsType = '';             // 'zfs' | 'btrfs'
  let selectedDisks = new Set(); // paths de discos seleccionados
  let poolName = '';
  let nameError = '';
  let confirmInput = '';
  let processing = false;
  let errorMsg = '';

  // Nombres reservados (espejo exacto del backend)
  const RESERVED_NAMES_ZFS   = ['system', 'config', 'temp', 'swap', 'root', 'boot', 'rpool'];
  const RESERVED_NAMES_BTRFS = ['system', 'config', 'temp', 'swap', 'root', 'boot'];
  $: reservedNames = fsType === 'zfs' ? RESERVED_NAMES_ZFS : RESERVED_NAMES_BTRFS;

  // ─── Derived ───

  // Default fsType cuando el usuario aterriza en paso 1
  $: if (step === 1 && !fsType) {
    if (capabilities.zfs) fsType = 'zfs';
    else if (capabilities.btrfs) fsType = 'btrfs';
  }

  $: diskCount = selectedDisks.size;

  // Calcular layout seguro según tipo + número de discos
  $: layout = computeLayout(fsType, diskCount);

  // Capacidad útil estimada (en bytes)
  // ZFS mirror: size * 1 (el menor) | raidz1: (n-1) * size_menor | raidz2: (n-2) * size_menor
  // BTRFS raid1: total / 2 | raid10: total / 2 | single: suma de todos
  $: selectedDisksArr = eligibleDisks.filter(d => selectedDisks.has(d.path || `/dev/${d.name}`));
  $: usableCapacity = computeUsableCapacity(fsType, layout, selectedDisksArr);

  // ¿El nombre es válido?
  $: {
    nameError = '';
    if (poolName.length > 0) {
      if (poolName.length > 32) {
        nameError = 'Máximo 32 caracteres.';
      } else if (!/^[a-zA-Z0-9-]+$/.test(poolName)) {
        nameError = 'Solo letras, números y guiones.';
      } else if (reservedNames.includes(poolName.toLowerCase())) {
        nameError = `"${poolName}" es un nombre reservado.`;
      }
    }
  }

  $: canAdvance = processing ? false
                : step === 1 ? (fsType === 'zfs' && capabilities.zfs) || (fsType === 'btrfs' && capabilities.btrfs)
                : step === 2 ? diskCount >= 1
                : step === 3 ? poolName.length > 0 && nameError === ''
                : step === 4 ? confirmInput === 'CREAR'
                : false;

  $: nextLabel = step === 4 ? (processing ? 'Creando...' : 'Crear pool') : 'Continuar →';
  $: nextVariant = step === 4 ? 'primary' : 'primary';

  // ─── Layout computation ───
  function computeLayout(fs, n) {
    if (!fs || n < 1) return { id: '', label: '—', redundancy: 'none', desc: '' };
    if (fs === 'zfs') {
      if (n === 1) return { id: 'single',  label: 'Single',        redundancy: 'none',   desc: 'Sin redundancia · toda la capacidad disponible' };
      if (n === 2) return { id: 'mirror',  label: 'Mirror (RAID1)', redundancy: 'n-1',    desc: 'Tolera fallo de 1 disco · capacidad = disco menor' };
      if (n === 3) return { id: 'raidz1',  label: 'RAIDZ1',        redundancy: '1 parity', desc: 'Tolera fallo de 1 disco · capacidad = (n-1) × disco menor' };
      return                { id: 'raidz2',  label: 'RAIDZ2',        redundancy: '2 parity', desc: 'Tolera fallo de 2 discos · capacidad = (n-2) × disco menor' };
    }
    // btrfs
    if (n === 1) return { id: 'single', label: 'Single',       redundancy: 'none', desc: 'Sin redundancia · toda la capacidad disponible' };
    if (n === 2) return { id: 'raid1',  label: 'RAID1',        redundancy: 'n-1',  desc: 'Duplica cada bloque · capacidad = total / 2' };
    if (n === 3) return { id: 'raid1',  label: 'RAID1',        redundancy: 'n-1',  desc: 'BTRFS distribuye copias entre discos · capacidad ~ total / 2' };
    return                { id: 'raid10', label: 'RAID10',       redundancy: 'n-1',  desc: 'Stripe + mirror · capacidad = total / 2 · mejor rendimiento' };
  }

  function computeUsableCapacity(fs, lay, disks) {
    if (disks.length === 0) return 0;
    const sizes = disks.map(d => d.size || 0).filter(s => s > 0);
    if (sizes.length === 0) return 0;
    const smallest = Math.min(...sizes);
    const total = sizes.reduce((a, b) => a + b, 0);
    const n = sizes.length;

    if (lay.id === 'single' || lay.id === 'stripe') return total;
    if (lay.id === 'mirror') return smallest;
    if (lay.id === 'raidz1') return smallest * (n - 1);
    if (lay.id === 'raidz2') return smallest * (n - 2);
    if (lay.id === 'raid1')  return Math.floor(total / 2);
    if (lay.id === 'raid10') return Math.floor(total / 2);
    return total;
  }

  // ─── Handlers ───
  function selectFsType(t) {
    if (t === 'zfs' && !capabilities.zfs) return;
    if (t === 'btrfs' && !capabilities.btrfs) return;
    fsType = t;
  }

  function toggleDisk(path) {
    if (selectedDisks.has(path)) selectedDisks.delete(path);
    else selectedDisks.add(path);
    selectedDisks = selectedDisks; // trigger reactivity
  }

  function handleNext() {
    if (step === 4) {
      submitCreate();
      return;
    }
    step += 1;
    errorMsg = '';
  }

  function handleBack() {
    if (step > 1) {
      step -= 1;
      errorMsg = '';
    }
  }

  function handleCancel() {
    if (processing) return;
    dispatch('cancel');
  }

  // ─── Create real ───
  async function submitCreate() {
    processing = true;
    errorMsg = '';

    const body = {
      type: fsType,
      name: poolName,
      disks: Array.from(selectedDisks),
    };
    if (fsType === 'zfs') {
      body.vdevType = layout.id;
    } else {
      body.profile = layout.id;
    }

    try {
      const res = await fetch('/api/storage/pool', {
        method: 'POST',
        headers: jsonHdrs(),
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        errorMsg = data.error || `Error ${res.status}`;
        processing = false;
        return;
      }
      processing = false;
      dispatch('done', { poolName });
    } catch (err) {
      console.error('create pool error:', err);
      errorMsg = err.message || 'Error al crear el pool';
      processing = false;
    }
  }

  // ─── Helpers ───
  function fmtBytes(b) {
    if (!b || b === 0) return '0 B';
    if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB';
    if (b >= 1e9)  return (b / 1e9).toFixed(1)  + ' GB';
    if (b >= 1e6)  return (b / 1e6).toFixed(0)  + ' MB';
    return b + ' B';
  }

  function diskPath(d) {
    return d.path || `/dev/${d.name}`;
  }

  // Detectar si hay tamaños distintos entre los discos seleccionados
  $: hasMixedSizes = (() => {
    if (selectedDisksArr.length < 2) return false;
    const sizes = selectedDisksArr.map(d => d.size || 0);
    const min = Math.min(...sizes);
    const max = Math.max(...sizes);
    // Consideramos "mezclados" si difieren más del 5%
    return max > 0 && (max - min) / max > 0.05;
  })();
</script>

<WizardModal
  open={true}
  title="Crear pool"
  tag={fsType ? fsType.toUpperCase() : ''}
  tagColor="accent"
  currentStep={step}
  totalSteps={4}
  {canAdvance}
  canGoBack={step > 1 && !processing}
  {nextLabel}
  {nextVariant}
  cancelLabel={processing ? 'Procesando...' : 'Cancelar'}
  on:next={handleNext}
  on:back={handleBack}
  on:cancel={handleCancel}
>

  <!-- PASO 1 · Tipo de pool -->
  {#if step === 1}
    <div class="pretitle">PASO 1 · SISTEMA DE ARCHIVOS</div>
    <div class="h">¿ZFS o BTRFS?</div>
    <div class="desc">
      Los dos son sistemas modernos con snapshots e integridad de datos.
      Elige según tus necesidades.
    </div>

    <div class="fs-options">
      <button
        class="fs-card"
        class:selected={fsType === 'zfs'}
        class:disabled={!capabilities.zfs}
        on:click={() => selectFsType('zfs')}
        disabled={!capabilities.zfs}
      >
        <div class="fs-head">
          <div class="fs-name">ZFS</div>
          {#if !capabilities.zfs}
            <Badge size="sm" variant="warn">no disponible</Badge>
          {:else if fsType === 'zfs'}
            <LED size={7} variant="ok" />
          {/if}
        </div>
        <div class="fs-desc">
          Más maduro y robusto. Snapshots instantáneos, replicación,
          compresión automática. Ideal para datos críticos.
        </div>
        <div class="fs-tags">
          <span class="fs-tag">RAIDZ1/Z2</span>
          <span class="fs-tag">ARC cache</span>
          <span class="fs-tag">dedup</span>
        </div>
      </button>

      <button
        class="fs-card"
        class:selected={fsType === 'btrfs'}
        class:disabled={!capabilities.btrfs}
        on:click={() => selectFsType('btrfs')}
        disabled={!capabilities.btrfs}
      >
        <div class="fs-head">
          <div class="fs-name">BTRFS</div>
          {#if !capabilities.btrfs}
            <Badge size="sm" variant="warn">no disponible</Badge>
          {:else if fsType === 'btrfs'}
            <LED size={7} variant="ok" />
          {/if}
        </div>
        <div class="fs-desc">
          Más flexible. Permite añadir discos de uno en uno, mezclar
          tamaños y cambiar el perfil RAID en caliente.
        </div>
        <div class="fs-tags">
          <span class="fs-tag">RAID1/10</span>
          <span class="fs-tag">balance</span>
          <span class="fs-tag">resize</span>
        </div>
      </button>
    </div>
  {/if}

  <!-- PASO 2 · Selección de discos -->
  {#if step === 2}
    <div class="pretitle">PASO 2 · DISCOS</div>
    <div class="h">Selecciona los discos del pool</div>
    <div class="desc">
      Los datos existentes en estos discos se <b>borrarán</b> al crear el pool.
      {#if fsType === 'zfs'}
        ZFS usará el tamaño del <b>disco menor</b> si mezclas capacidades.
      {:else}
        BTRFS puede mezclar capacidades sin desperdiciar espacio.
      {/if}
    </div>

    {#if eligibleDisks.length === 0}
      <div class="no-disks">
        No hay discos libres elegibles. Ve a la vista Discos y formatea
        algún disco primero.
      </div>
    {:else}
      <div class="disk-select-list">
        {#each eligibleDisks as d}
          {@const path = diskPath(d)}
          <button
            class="disk-select-row"
            class:selected={selectedDisks.has(path)}
            on:click={() => toggleDisk(path)}
          >
            <div class="ds-check">
              {#if selectedDisks.has(path)}✓{/if}
            </div>
            <div class="ds-info">
              <div class="ds-path mono">{path}</div>
              <div class="ds-model">{d.model || '—'}</div>
            </div>
            <div class="ds-size">{d.sizeH || fmtBytes(d.size)}</div>
            <Badge size="sm" variant={d.rotational ? 'default' : 'info'}>
              {d.rotational ? 'HDD' : 'SSD'}
            </Badge>
          </button>
        {/each}
      </div>
    {/if}

    <!-- Layout recomendado al vuelo -->
    {#if diskCount > 0}
      <div class="layout-preview">
        <div class="lp-head">
          <span class="lp-label">Layout recomendado</span>
          <span class="lp-name">{layout.label}</span>
        </div>
        <div class="lp-desc">{layout.desc}</div>
        <div class="lp-cap">
          <span class="lp-cap-label">Capacidad útil estimada:</span>
          <span class="lp-cap-val">{fmtBytes(usableCapacity)}</span>
        </div>
        {#if hasMixedSizes && fsType === 'zfs'}
          <div class="lp-warn">
            ⚠ Los discos tienen tamaños distintos. ZFS usará el del disco
            menor y desaprovechará el resto.
          </div>
        {/if}
      </div>
    {/if}
  {/if}

  <!-- PASO 3 · Nombre -->
  {#if step === 3}
    <div class="pretitle">PASO 3 · NOMBRE</div>
    <div class="h">Dale un nombre al pool</div>
    <div class="desc">
      Este nombre se usará en la ruta de montaje (<span class="mono">/nimbus/pools/{poolName || 'nombre'}</span>)
      y en los shares. Elige algo corto y descriptivo.
    </div>

    <div class="name-input-row">
      <input
        class="name-input mono"
        class:err={nameError !== ''}
        class:ok={poolName.length > 0 && nameError === ''}
        type="text"
        bind:value={poolName}
        placeholder="ej: datos, media, backup"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
        spellcheck="false"
        maxlength="32"
      />
    </div>

    <div class="name-hint">
      <span class:err={nameError !== ''}>
        {#if nameError}
          {nameError}
        {:else if poolName.length === 0}
          Máximo 32 caracteres · letras, números y guiones · sin espacios
        {:else}
          ✓ Nombre válido
        {/if}
      </span>
    </div>

    <!-- Resumen del pool -->
    <div class="summary-box">
      <div class="summary-row">
        <span class="summary-label">Sistema</span>
        <span class="summary-val">{fsType.toUpperCase()}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">Layout</span>
        <span class="summary-val">{layout.label}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">Discos</span>
        <span class="summary-val">{diskCount}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">Capacidad útil</span>
        <span class="summary-val">{fmtBytes(usableCapacity)}</span>
      </div>
    </div>
  {/if}

  <!-- PASO 4 · Confirmación -->
  {#if step === 4}
    <div class="pretitle">PASO 4 · CONFIRMACIÓN</div>
    <div class="h">Última comprobación</div>
    <div class="desc">
      Vas a crear el pool <b class="mono">{poolName}</b> con
      {diskCount} disco{diskCount === 1 ? '' : 's'} en layout <b>{layout.label}</b>.
    </div>

    <ul class="bullets">
      <li>Los datos existentes en los discos se <b>borrarán</b></li>
      <li>El pool se montará en <span class="mono">/nimbus/pools/{poolName}</span></li>
      {#if fsType === 'zfs'}
        <li>El zpool interno se llamará <span class="mono">nimos-{poolName}</span></li>
      {/if}
      <li>Podrás gestionar shares, snapshots y apps desde NimOS</li>
    </ul>

    <div class="disks-preview">
      <div class="dp-head">Discos incluidos:</div>
      {#each selectedDisksArr as d}
        <div class="dp-row mono">
          <span>{diskPath(d)}</span>
          <span class="tc-mute">· {d.model || '—'} · {d.sizeH || fmtBytes(d.size)}</span>
        </div>
      {/each}
    </div>

    <div class="confirm-label">Escribe <b>CREAR</b> para confirmar:</div>
    <input
      class="confirm-input"
      class:ok={confirmInput === 'CREAR'}
      type="text"
      bind:value={confirmInput}
      placeholder="CREAR"
      autocomplete="off"
      autocorrect="off"
      autocapitalize="off"
      spellcheck="false"
      disabled={processing}
    />

    {#if errorMsg}
      <div class="err-box">{errorMsg}</div>
    {/if}
  {/if}

</WizardModal>

<style>
  .pretitle {
    font-size: 9px;
    color: var(--fg-faint);
    letter-spacing: 2px;
    text-transform: uppercase;
    font-family: var(--font-mono);
  }
  .h {
    font-size: 15px;
    color: var(--fg);
    letter-spacing: 0.4px;
    font-family: var(--font-sans, inherit);
    font-weight: 500;
    line-height: 1.3;
  }
  .desc {
    font-size: 12px;
    color: var(--fg-dim);
    line-height: 1.6;
    font-family: var(--font-sans, inherit);
  }
  .desc :global(b) { color: var(--accent); font-weight: 600; }
  .mono { font-family: var(--font-mono); }
  .tc-mute { color: var(--fg-mute); }

  /* Paso 1 · Filesystem cards */
  .fs-options {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .fs-card {
    background: var(--bg);
    border: 1px solid var(--border);
    padding: 14px 14px 12px;
    text-align: left;
    cursor: pointer;
    font-family: inherit;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: border-color 0.15s, background 0.15s;
  }
  .fs-card:hover:not(.disabled) {
    border-color: var(--accent);
  }
  .fs-card.selected {
    border-color: var(--accent);
    background: rgba(255, 130, 0, 0.04);
  }
  .fs-card.disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .fs-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .fs-name {
    font-size: 16px;
    color: var(--fg);
    font-weight: 700;
    font-family: var(--font-mono);
    letter-spacing: 1px;
  }
  .fs-desc {
    font-size: 11px;
    color: var(--fg-dim);
    line-height: 1.5;
    font-family: var(--font-sans, inherit);
  }
  .fs-tags {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .fs-tag {
    font-size: 9px;
    padding: 2px 6px;
    background: var(--bg-1);
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    font-family: var(--font-mono);
    border: 1px solid var(--border);
  }

  /* Paso 2 · Disk selection */
  .no-disks {
    padding: 14px;
    background: rgba(255, 184, 0, 0.05);
    border-left: 3px solid var(--warn);
    font-size: 12px;
    color: var(--fg-dim);
    font-family: var(--font-sans, inherit);
    line-height: 1.5;
  }
  .disk-select-list {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border);
  }
  .disk-select-row {
    display: grid;
    grid-template-columns: 22px 1fr auto auto;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg);
    border: none;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    text-align: left;
    font-family: inherit;
    transition: background 0.1s;
  }
  .disk-select-row:last-child { border-bottom: none; }
  .disk-select-row:hover { background: var(--bg-1); }
  .disk-select-row.selected {
    background: rgba(255, 130, 0, 0.04);
  }
  .ds-check {
    width: 16px;
    height: 16px;
    border: 1px solid var(--border-bright);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent);
    font-size: 12px;
    font-weight: 700;
    font-family: var(--font-mono);
  }
  .disk-select-row.selected .ds-check {
    border-color: var(--accent);
  }
  .ds-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .ds-path {
    font-size: 12px;
    color: var(--fg);
  }
  .ds-model {
    font-size: 10px;
    color: var(--fg-mute);
    font-family: var(--font-mono);
  }
  .ds-size {
    font-size: 12px;
    color: var(--fg);
    font-family: var(--font-mono);
  }

  /* Layout preview (paso 2) */
  .layout-preview {
    padding: 12px 14px;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-left: 3px solid var(--accent);
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 6px;
  }
  .lp-head {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .lp-label {
    font-size: 9px;
    color: var(--fg-faint);
    letter-spacing: 1.5px;
    text-transform: uppercase;
    font-family: var(--font-mono);
  }
  .lp-name {
    font-size: 14px;
    color: var(--accent);
    font-weight: 600;
    font-family: var(--font-mono);
    letter-spacing: 0.5px;
  }
  .lp-desc {
    font-size: 11px;
    color: var(--fg-dim);
    line-height: 1.5;
  }
  .lp-cap {
    display: flex;
    gap: 6px;
    align-items: baseline;
    padding-top: 6px;
    border-top: 1px solid var(--border);
    margin-top: 2px;
  }
  .lp-cap-label {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    text-transform: uppercase;
    font-family: var(--font-mono);
  }
  .lp-cap-val {
    font-size: 14px;
    color: var(--fg);
    font-weight: 700;
    font-family: var(--font-mono);
  }
  .lp-warn {
    font-size: 10px;
    color: var(--warn);
    padding: 6px 8px;
    background: rgba(255, 184, 0, 0.05);
    border-left: 2px solid var(--warn);
    font-family: var(--font-mono);
    letter-spacing: 0.3px;
    line-height: 1.4;
  }

  /* Paso 3 · Name */
  .name-input-row { display: flex; }
  .name-input {
    flex: 1;
    padding: 10px 14px;
    background: var(--bg);
    border: 1px solid var(--border-bright);
    color: var(--fg);
    font-size: 14px;
    letter-spacing: 1px;
    outline: none;
    transition: border-color 0.15s, color 0.15s;
  }
  .name-input:focus { border-color: var(--accent); }
  .name-input.ok    { border-color: var(--ok, #00d97e); color: var(--ok, #00d97e); }
  .name-input.err   { border-color: var(--crit); }
  .name-hint {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.3px;
    font-family: var(--font-mono);
  }
  .name-hint .err { color: var(--crit); }

  .summary-box {
    display: flex;
    flex-direction: column;
    background: var(--bg);
    border: 1px solid var(--border);
    margin-top: 4px;
  }
  .summary-row {
    display: grid;
    grid-template-columns: 130px 1fr;
    padding: 8px 14px;
    border-bottom: 1px solid var(--border);
    font-size: 11px;
  }
  .summary-row:last-child { border-bottom: none; }
  .summary-label {
    color: var(--fg-faint);
    font-size: 9px;
    letter-spacing: 1px;
    text-transform: uppercase;
    font-family: var(--font-mono);
  }
  .summary-val {
    color: var(--fg);
    font-family: var(--font-mono);
  }

  /* Paso 4 · Bullets + confirm */
  .bullets {
    list-style: none;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0;
  }
  .bullets li {
    font-size: 12px;
    color: var(--fg-dim);
    padding-left: 18px;
    position: relative;
    line-height: 1.5;
    font-family: var(--font-sans, inherit);
  }
  .bullets li::before {
    content: '›';
    position: absolute;
    left: 4px;
    color: var(--accent);
    font-weight: 700;
  }
  .bullets li :global(b) {
    color: var(--fg);
    font-weight: 600;
  }

  .disks-preview {
    background: var(--bg);
    border: 1px solid var(--border);
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .dp-head {
    font-size: 9px;
    color: var(--fg-faint);
    letter-spacing: 1.5px;
    text-transform: uppercase;
    font-family: var(--font-mono);
    margin-bottom: 4px;
  }
  .dp-row {
    font-size: 11px;
    color: var(--fg);
    display: flex;
    gap: 4px;
  }

  .confirm-label {
    font-size: 10px;
    color: var(--fg-dim);
    letter-spacing: 1px;
    text-transform: uppercase;
    font-family: var(--font-mono);
    margin-top: 4px;
  }
  .confirm-label :global(b) {
    color: var(--accent);
    font-weight: 700;
    font-size: 11px;
  }
  .confirm-input {
    width: 100%;
    padding: 10px 14px;
    background: var(--bg);
    border: 1px solid var(--border-bright);
    color: var(--fg);
    font-family: var(--font-mono);
    font-size: 13px;
    letter-spacing: 2px;
    outline: none;
    transition: border-color 0.15s, color 0.15s;
  }
  .confirm-input:focus { border-color: var(--accent); }
  .confirm-input.ok    { border-color: var(--ok, #00d97e); color: var(--ok, #00d97e); }
  .confirm-input:disabled { opacity: 0.5; cursor: not-allowed; }

  .err-box {
    padding: 10px 12px;
    background: rgba(255, 90, 90, 0.08);
    border-left: 3px solid var(--crit);
    font-size: 11px;
    color: var(--crit);
    font-family: var(--font-mono);
    letter-spacing: 0.3px;
    line-height: 1.5;
  }
</style>
