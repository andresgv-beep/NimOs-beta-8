<script>
  /**
   * DestroyPoolWizard · Wizard to permanently destroy a ZFS/BTRFS pool
   * ─────────────────────────────────────────────────────────────────
   * Destroys the pool and releases its disks. Two mandatory guards:
   *
   *   1. Check dependent services (GET /api/services/dependencies?pool=X)
   *      If any running → block, redirect to NimHealth to stop them
   *   2. Final confirmation · user must type the pool name
   *
   * Then POST /api/storage/pool/destroy { name } → pool gone, disks free
   *
   * Note: the backend destroy operates on the mounted pool config; it handles
   * unmount internally as part of zpool destroy / btrfs wipefs. We don't need
   * a separate unmount step.
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

  const dispatch = createEventDispatcher();

  // Pasos:
  //   1 = detectando (carga inicial)
  //   2 = servicios activos (si los hay)
  //   3 = confirmación final
  let step = 1;
  let loading = true;
  let deps = [];
  let pollInterval = null;
  let confirmInput = '';
  let processing = false;
  let errorMsg = '';

  // ─── Derived ───
  $: allStopped = deps.length === 0 || deps.every(d => d.status === 'stopped' || d.status === 'exited');
  $: canAdvance =
      step === 1 ? false
    : step === 2 ? allStopped
    : step === 3 ? confirmInput.trim() === poolName && !processing
    : false;

  $: nextLabel = step === 3 ? 'Destruir pool' : 'Continuar →';
  $: nextVariant = step === 3 ? 'danger' : 'primary';

  const TOTAL_STEPS = 3;

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

  // ─── Handlers ───
  function handleNext() {
    if (step === 2) {
      stopPolling();
      step = 3;
      return;
    }
    if (step === 3) {
      submitDestroy();
      return;
    }
  }

  function handleBack() {
    if (step === 3 && deps.length > 0) {
      step = 2;
      startPolling();
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

  // ─── Destroy real (paso 3) ───
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
        // Si aparecieron servicios mientras tanto, volver al paso 2
        if (typeof data.error === 'string' && /Active services/i.test(data.error)) {
          errorMsg = 'Aparecieron servicios activos. Deténlos y reintenta.';
          await fetchDeps();
          step = 2;
          startPolling();
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
    if (deps.length > 0) {
      step = 2;
      startPolling();
    } else {
      step = 3;
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
  canGoBack={step === 3 && deps.length > 0}
  nextLabel={nextLabel}
  nextVariant={nextVariant}
  on:next={handleNext}
  on:back={handleBack}
  on:cancel={handleCancel}
>

  <!-- PASO 1 · Detección -->
  {#if step === 1}
    <div class="pretitle">DETECCIÓN</div>
    <div class="h">Verificando servicios dependientes...</div>
    <div class="desc">
      Antes de destruir el pool, NimOS comprueba qué servicios están usándolo activamente.
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
        Ningún servicio está usando este pool. Puedes proceder a la destrucción.
      {:else}
        Hay servicios corriendo que dependen de este pool.
        Ve a <b>NimHealth</b> para detenerlos.
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

  <!-- PASO 3 · Confirmación final -->
  {#if step === 3}
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

