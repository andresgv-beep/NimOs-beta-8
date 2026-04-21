<script>
  /**
   * DestroyPoolWizard · Wizard to permanently destroy a ZFS/BTRFS pool
   * ─────────────────────────────────────────────────────────────────
   * Unlike ExportPoolWizard (which does a reversible export), this one
   * permanently destroys the pool and releases its disks. Has 3 mandatory
   * guards in sequence before allowing destruction:
   *
   *   1. Check dependent services (GET /api/services/dependencies?pool=X)
   *      If any running → block, redirect to NimHealth to stop them
   *   2. Check pool is exported (not mounted)
   *      If mounted → block, offer to export it first (calls /api/storage/pool/export)
   *   3. Final confirmation · user must type the pool name
   *
   * Then POST /api/storage/pool/destroy { name } → pool gone, disks free
   *
   * Usage:
   *   <DestroyPoolWizard poolName="data3" on:done on:cancel />
   */
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { token } from '$lib/stores/auth.js';
  import { openWindow } from '$lib/stores/windows.js';
  import WizardModal from '$lib/ui/WizardModal.svelte';
  import LED from '$lib/ui/LED.svelte';

  export let poolName = '';
  /** Si el pool ya está exportado (llamado desde Restaurar en el futuro), saltamos paso 2 */
  export let alreadyExported = false;

  const dispatch = createEventDispatcher();

  // Pasos:
  //   1 = detectando (carga inicial)
  //   2 = servicios activos (si los hay)
  //   3 = desmontar pool (si está montado)
  //   4 = confirmación final
  let step = 1;
  let loading = true;
  let deps = [];                   // servicios activos
  let poolMounted = !alreadyExported; // al empezar asumimos montado (porque viene de Discos)
  let pollInterval = null;
  let confirmInput = '';
  let processing = false;
  let errorMsg = '';
  let exporting = false;           // estado del desmontaje inline

  // ─── Derived ───
  $: allStopped = deps.length === 0 || deps.every(d => d.status === 'stopped' || d.status === 'exited');
  $: canAdvance =
      step === 1 ? false
    : step === 2 ? allStopped
    : step === 3 ? !poolMounted && !exporting
    : step === 4 ? confirmInput.trim() === poolName && !processing
    : false;

  // ─── Title dinámico del próximo botón ───
  $: nextLabel =
      step === 4 ? 'Destruir pool'
    : 'Continuar →';

  $: nextVariant = step === 4 ? 'danger' : 'primary';

  // ─── Total steps (dinámico según si hay servicios o pool montado) ───
  // Simplificamos: siempre mostramos 4 pasos visuales, aunque algunos se salten.
  const TOTAL_STEPS = 4;

  // ─── Fetch dependencies ───
  async function fetchDeps() {
    try {
      const res = await fetch(`/api/services/dependencies?pool=${encodeURIComponent(poolName)}`, {
        headers: { 'Authorization': `Bearer ${$token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      deps = data.dependencies || [];
      return true;
    } catch (err) {
      console.error('fetchDeps error:', err);
      errorMsg = 'No se pudo consultar dependencias del pool.';
      return false;
    }
  }

  // ─── Check if pool is mounted ───
  async function checkPoolMounted() {
    try {
      const res = await fetch('/api/storage/pools', {
        headers: { 'Authorization': `Bearer ${$token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const pools = data.pools || data || [];
      poolMounted = pools.some(p => p.name === poolName);
      return true;
    } catch (err) {
      console.error('checkPoolMounted error:', err);
      return false;
    }
  }

  // ─── Handlers ───
  function handleNext() {
    if (step === 2) {
      // Pasar de servicios a desmontaje
      stopPolling();
      if (poolMounted) {
        step = 3;
      } else {
        step = 4;
      }
      return;
    }
    if (step === 3) {
      // Pool ya no está montado, pasar a confirmación final
      step = 4;
      return;
    }
    if (step === 4) {
      submitDestroy();
      return;
    }
  }

  function handleBack() {
    if (step === 4) {
      step = poolMounted ? 3 : (deps.length > 0 ? 2 : 1);
      if (step === 2) startPolling();
      return;
    }
    if (step === 3) {
      step = deps.length > 0 ? 2 : 1;
      if (step === 2) startPolling();
      return;
    }
  }

  function handleCancel() {
    stopPolling();
    dispatch('cancel');
  }

  function openNimHealth() {
    openWindow('nimhealth');
  }

  // ─── Polling paso 2 ───
  function startPolling() {
    stopPolling();
    pollInterval = setInterval(fetchDeps, 3000);
  }
  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  // ─── Desmontar inline (paso 3) ───
  async function handleExportInline() {
    if (exporting) return;
    exporting = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/storage/pool/export', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${$token}`,
        },
        body: JSON.stringify({ name: poolName }),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        // Por si acaso · el backend comprueba servicios también
        if (data.error === 'services_active') {
          errorMsg = `Aparecieron servicios activos: ${(data.services || []).join(', ')}. Vuelve al paso 2.`;
          await fetchDeps();
          step = 2;
          startPolling();
        } else {
          errorMsg = data.error || `Error ${res.status}`;
        }
        exporting = false;
        return;
      }
      // Éxito del desmontaje
      poolMounted = false;
      exporting = false;
    } catch (err) {
      console.error('export inline error:', err);
      errorMsg = err.message || 'Error al desmontar';
      exporting = false;
    }
  }

  // ─── Destroy real (paso 4) ───
  async function submitDestroy() {
    processing = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/storage/pool/destroy', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${$token}`,
        },
        body: JSON.stringify({ name: poolName }),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        if (data.error === 'services_active') {
          errorMsg = `Servicios se activaron: ${(data.services || []).join(', ')}. Reinicia el wizard.`;
          // Retrocedemos a servicios
          await fetchDeps();
          step = 2;
          startPolling();
        } else if (data.error && /mount|active/i.test(data.error)) {
          errorMsg = 'El pool sigue montado. Desmóntalo en el paso anterior.';
          await checkPoolMounted();
          step = 3;
        } else {
          errorMsg = data.error || `Error ${res.status}`;
        }
        processing = false;
        return;
      }
      processing = false;
      dispatch('done');
    } catch (err) {
      console.error('destroy error:', err);
      errorMsg = err.message || 'Error al destruir pool';
      processing = false;
    }
  }

  // ─── Lifecycle ───
  onMount(async () => {
    await fetchDeps();
    loading = false;

    // Flujo inicial: si hay servicios → paso 2; si no hay pero pool montado → paso 3;
    // si ni servicios ni montado → paso 4
    if (deps.length > 0) {
      step = 2;
      startPolling();
    } else if (poolMounted) {
      step = 3;
    } else {
      step = 4;
    }
  });

  onDestroy(stopPolling);

  // ─── UI helpers ───
  function statusLabel(s) {
    if (s === 'running')  return 'running';
    if (s === 'stopped' || s === 'exited')  return 'stopped';
    if (s === 'starting') return 'starting';
    if (s === 'stopping') return 'stopping';
    return s || 'unknown';
  }
  function statusLedVariant(s) {
    if (s === 'running')  return 'ok';
    if (s === 'starting' || s === 'stopping') return 'warn';
    return 'off';
  }
</script>

<WizardModal
  open={true}
  title="Destruir pool"
  tag={poolName}
  tagColor="danger"
  currentStep={step === 1 ? 1 : step}
  totalSteps={TOTAL_STEPS}
  canAdvance={canAdvance}
  canGoBack={(step === 3 && deps.length > 0) || (step === 4 && (poolMounted || deps.length > 0))}
  nextLabel={nextLabel}
  nextVariant={nextVariant}
  on:next={handleNext}
  on:back={handleBack}
  on:cancel={handleCancel}
>

  <!-- PASO 1 · Detección inicial -->
  {#if step === 1}
    <div class="pretitle">DETECCIÓN</div>
    <div class="h">Verificando estado del pool...</div>
    <div class="desc">
      Antes de destruir el pool, NimOS comprueba servicios activos y si el pool está montado.
    </div>
    <div class="recheck">
      <span class="spin">⟳</span>
      <span>Consultando daemon...</span>
    </div>
    {#if errorMsg}<div class="err">{errorMsg}</div>{/if}
  {/if}

  <!-- PASO 2 · Servicios dependientes -->
  {#if step === 2}
    <div class="pretitle">
      {#if allStopped}
        SERVICIOS DETENIDOS · {deps.length}
      {:else}
        SERVICIOS ACTIVOS · {deps.filter(d => d.status !== 'stopped' && d.status !== 'exited').length}
      {/if}
    </div>
    <div class="h">
      {#if allStopped}
        Servicios parados · puedes continuar
      {:else}
        Detén los servicios dependientes
      {/if}
    </div>
    <div class="desc">
      {#if allStopped}
        Ningún servicio está usando este pool. Pasamos a comprobar el estado del pool.
      {:else}
        Hay servicios corriendo que dependen de este pool.
        Ve a <b>NimHealth</b> (el gestor central de servicios) para detenerlos.
      {/if}
    </div>

    <div class="svc-list">
      {#each deps as dep}
        <div class="svc-row">
          <LED size={8} variant={statusLedVariant(dep.status)} />
          <div class="svc-name">
            {dep.app || dep.id}
            <span class="svc-tag">@{poolName}</span>
          </div>
          <div class="svc-state state-{dep.status === 'running' ? 'run' : 'stop'}">
            {statusLabel(dep.status)}
          </div>
          {#if dep.status === 'running' || dep.status === 'starting'}
            <button class="svc-action" on:click={openNimHealth}>→ NimHealth</button>
          {:else}
            <div class="svc-action muted">—</div>
          {/if}
        </div>
      {/each}
    </div>

    {#if allStopped}
      <div class="recheck ok">
        <span>✓</span>
        <span>Todos los servicios detenidos</span>
      </div>
    {:else}
      <div class="recheck">
        <span class="spin">⟳</span>
        <span>Re-verificando cada 3s...</span>
      </div>
    {/if}
    {#if errorMsg}<div class="err">{errorMsg}</div>{/if}
  {/if}

  <!-- PASO 3 · Desmontar pool -->
  {#if step === 3}
    <div class="pretitle">DESMONTAJE REQUERIDO</div>
    <div class="h">
      {#if poolMounted}
        El pool está montado · debe desmontarse primero
      {:else}
        Pool desmontado · listo para destruir
      {/if}
    </div>
    <div class="desc">
      {#if poolMounted}
        Un pool montado no puede destruirse directamente. Desmóntalo desde aquí para preparar la destrucción.
        Tras desmontar, los datos seguirían intactos en los discos hasta la destrucción final del paso 4.
      {:else}
        El pool <b>{poolName}</b> ya está desmontado. Puedes proceder a la destrucción definitiva.
      {/if}
    </div>

    <div class="export-panel" class:done={!poolMounted}>
      <div class="export-info">
        <div class="export-label">Pool</div>
        <div class="export-value">
          <b>{poolName}</b>
          {#if poolMounted}
            <span class="badge-mounted">MONTADO</span>
          {:else}
            <span class="badge-exported">DESMONTADO</span>
          {/if}
        </div>
      </div>
      {#if poolMounted}
        <button class="export-btn" on:click={handleExportInline} disabled={exporting}>
          {#if exporting}
            <span class="spin">⟳</span> Desmontando...
          {:else}
            Desmontar pool
          {/if}
        </button>
      {:else}
        <div class="export-ok">✓ LISTO</div>
      {/if}
    </div>

    {#if errorMsg}<div class="err">{errorMsg}</div>{/if}
  {/if}

  <!-- PASO 4 · Confirmación final -->
  {#if step === 4}
    <div class="pretitle">CONFIRMACIÓN · DESTRUCCIÓN FINAL</div>
    <div class="h">Última comprobación antes de destruir el pool</div>

    <ul class="bullets">
      <li>Se <b>destruirá definitivamente</b> el pool <b>{poolName}</b></li>
      <li>Todos los <b>datos se perderán</b> · esta acción <b>no se puede deshacer</b></li>
      <li>Los discos del pool quedarán <b>libres</b> para nuevos usos</li>
      <li>La configuración del pool se elimina de NimOS</li>
    </ul>

    <div class="confirm-label">
      Escribe el nombre del pool <b>{poolName}</b> para confirmar:
    </div>
    <input
      class="confirm-input"
      class:ok={confirmInput.trim() === poolName}
      type="text"
      bind:value={confirmInput}
      placeholder={poolName}
      autocomplete="off"
      autocorrect="off"
      autocapitalize="off"
      spellcheck="false"
      disabled={processing}
    />

    {#if errorMsg}<div class="err">{errorMsg}</div>{/if}
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

  .recheck {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    font-family: var(--font-mono);
    margin-top: 4px;
  }
  .recheck.ok { color: var(--ok, #00d97e); }
  .recheck .spin {
    display: inline-block;
    animation: spin 1s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* Service list */
  .svc-list {
    background: var(--bg);
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
  }
  .svc-row {
    display: grid;
    grid-template-columns: 14px 1fr auto auto;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .svc-row:last-child { border-bottom: none; }
  .svc-name { color: var(--fg); letter-spacing: 0.3px; }
  .svc-tag { color: var(--fg-faint); margin-left: 4px; font-size: 10px; }
  .svc-state {
    font-size: 9px;
    letter-spacing: 1.5px;
    text-transform: uppercase;
  }
  .svc-state.state-run  { color: var(--crit); }
  .svc-state.state-stop { color: var(--fg-mute); }

  .svc-action {
    font-size: 9px;
    letter-spacing: 1px;
    text-transform: uppercase;
    padding: 4px 10px;
    background: transparent;
    border: 1px solid var(--border-bright);
    color: var(--fg-dim);
    cursor: pointer;
    font-family: inherit;
    transition: all 0.12s;
    clip-path: polygon(
      0 0, calc(100% - 4px) 0, 100% 4px,
      100% 100%, 4px 100%, 0 calc(100% - 4px)
    );
  }
  .svc-action:hover:not(.muted) {
    border-color: var(--accent);
    color: var(--accent);
  }
  .svc-action.muted {
    opacity: 0.35;
    cursor: default;
    padding: 4px 10px;
    display: inline-block;
  }

  /* Export panel paso 3 */
  .export-panel {
    background: var(--bg);
    border: 1px solid var(--border);
    padding: 14px 18px;
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 16px;
    font-family: var(--font-mono);
  }
  .export-panel.done {
    border-color: var(--ok, #00d97e);
    background: rgba(0, 217, 126, 0.03);
  }
  .export-label {
    font-size: 9px;
    color: var(--fg-mute);
    letter-spacing: 1.5px;
    text-transform: uppercase;
  }
  .export-value {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 4px;
    font-size: 13px;
    color: var(--fg);
  }
  .badge-mounted {
    font-size: 9px;
    padding: 2px 8px;
    border: 1px solid var(--warn);
    color: var(--warn);
    letter-spacing: 1px;
  }
  .badge-exported {
    font-size: 9px;
    padding: 2px 8px;
    border: 1px solid var(--ok, #00d97e);
    color: var(--ok, #00d97e);
    letter-spacing: 1px;
  }
  .export-btn {
    padding: 8px 16px;
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 1.5px;
    text-transform: uppercase;
    background: var(--bg-2);
    border: 1px solid var(--warn);
    color: var(--warn);
    cursor: pointer;
    transition: all 0.12s;
    clip-path: polygon(
      0 0, calc(100% - 5px) 0, 100% 5px,
      100% 100%, 5px 100%, 0 calc(100% - 5px)
    );
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .export-btn:hover:not(:disabled) {
    background: rgba(255, 184, 0, 0.1);
  }
  .export-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .export-btn .spin {
    display: inline-block;
    animation: spin 1s linear infinite;
  }
  .export-ok {
    font-size: 11px;
    letter-spacing: 2px;
    color: var(--ok, #00d97e);
    font-family: var(--font-mono);
    font-weight: 700;
  }

  /* Bullets */
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
    color: var(--crit);
    font-weight: 700;
  }
  .bullets li :global(b) {
    color: var(--fg);
    font-weight: 600;
  }

  /* Confirm input */
  .confirm-label {
    font-size: 10px;
    color: var(--fg-dim);
    letter-spacing: 1px;
    text-transform: uppercase;
    font-family: var(--font-mono);
    margin-top: 4px;
  }
  .confirm-label :global(b) {
    color: var(--crit);
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

  .err {
    padding: 10px 12px;
    background: rgba(255, 90, 90, 0.08);
    border-left: 3px solid var(--crit);
    font-size: 11px;
    color: var(--crit);
    font-family: var(--font-mono);
    letter-spacing: 0.3px;
  }
</style>
