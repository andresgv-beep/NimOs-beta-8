<script>
  import { createEventDispatcher } from 'svelte';
  import { fly } from 'svelte/transition';

  const dispatch = createEventDispatcher();

  export let open = false;
  export let variant = 'default'; // 'default' | 'warning' | 'danger'
  export let title = '';
  export let message = '';
  export let confirmText = 'Confirmar';
  export let cancelText = 'Cancelar';
  export let requireInput = false; // require typing "confirmar"
  export let services = []; // [{ name, status }]
  export let loading = false;
  export let disabled = false; // externally disable confirm button

  let inputValue = '';

  $: hasActiveServices = services.some(s => s.status === 'running' || s.status === 'starting');
  $: inputValid = inputValue.trim().toLowerCase() === 'confirmar';
  $: canConfirm = !disabled && !hasActiveServices && (requireInput ? inputValid : true);

  function onConfirm() {
    if (!canConfirm || loading) return;
    dispatch('confirm');
  }

  function onCancel() {
    inputValue = '';
    dispatch('cancel');
  }

  function onKeydown(e) {
    if (e.key === 'Escape') onCancel();
    if (e.key === 'Enter' && canConfirm && !loading) onConfirm();
  }
</script>

<svelte:window on:keydown={onKeydown} />

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="cd-overlay" on:click|self={onCancel} transition:fly={{ duration: 150 }}>
    <div class="cd {variant}">
      <div class="cd-header">
        <div class="cd-icon">
          {#if variant === 'danger'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/><path d="M10 11v6m4-6v6"/><path d="M9 6V4a1 1 0 011-1h4a1 1 0 011 1v2"/>
            </svg>
          {:else if variant === 'warning'}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/><path d="M12 8v4m0 4h.01"/>
            </svg>
          {/if}
        </div>
        <div class="cd-title">{title}</div>
      </div>

      <div class="cd-body">
        <p class="cd-message">{message}</p>

        {#if services.length > 0}
          <div class="cd-services">
            <div class="cd-services-header">
              <span class="cd-services-label">Servicios activos en este volumen</span>
            </div>
            {#each services as svc}
              <div class="cd-service-row">
                <span class="cd-service-name"><span class="cd-service-dot"></span>{svc.name || svc.appName || svc.appId}</span>
                <span class="cd-service-status">{svc.status === 'running' ? 'activo' : svc.status}</span>
              </div>
            {/each}
          </div>
          {#if hasActiveServices}
            <div class="cd-services-block">
              <span>Detén los servicios antes de continuar</span>
              <button class="cd-btn cd-btn-services" on:click={() => dispatch('openServices')}>
                Gestionar servicios →
              </button>
            </div>
          {/if}
        {/if}
      </div>

      {#if requireInput}
        <div class="cd-confirm-field">
          <p class="cd-confirm-label">Escribe <span>confirmar</span> para continuar</p>
          <input
            class="cd-confirm-input"
            class:valid={inputValid}
            type="text"
            placeholder="confirmar"
            autocomplete="off"
            spellcheck="false"
            bind:value={inputValue}
          />
        </div>
      {/if}

      <div class="cd-actions">
        <button class="cd-btn cd-btn-cancel" on:click={onCancel}>{cancelText}</button>
        <button
          class="cd-btn cd-btn-confirm cd-btn-{variant}"
          disabled={!canConfirm || loading}
          on:click={onConfirm}
        >
          {loading ? 'Procesando...' : confirmText}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .cd-overlay {
    position: fixed; inset: 0; z-index: 10000;
    background: rgba(0,0,0,0.55);
    display: flex; align-items: center; justify-content: center;
  }

  .cd {
    width: 440px; max-width: 90vw;
    background: var(--bg-elev-1);
    border: 1px solid var(--glass-border);
    border-radius: 14px;
    padding: 28px 28px 24px;
    box-shadow: 0 24px 64px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.04) inset;
    animation: cdIn 220ms cubic-bezier(0.34, 1.2, 0.64, 1) both;
  }
  @keyframes cdIn { from { opacity:0; transform:scale(0.92) translateY(6px); } to { opacity:1; transform:none; } }

  .cd.danger  { border-top: 2px solid var(--c-crit); }
  .cd.warning { border-top: 2px solid var(--c-warn); }

  .cd-header { display:flex; align-items:center; gap:12px; margin-bottom:12px; }
  .cd-icon {
    width:36px; height:36px; border-radius:10px;
    display:flex; align-items:center; justify-content:center; flex-shrink:0;
  }
  .cd-icon svg { width:18px; height:18px; stroke-linecap:round; stroke-linejoin:round; }
  .cd.danger  .cd-icon { background:var(--c-crit-dim); color:var(--c-crit); }
  .cd.warning .cd-icon { background:var(--c-warn-dim); color:var(--c-warn); }
  .cd.default .cd-icon { background:var(--accent-dim); color:var(--accent); }

  .cd-title { font-size:15px; font-weight:600; color:var(--text-primary); line-height:1.3; }

  .cd-body { padding-left:48px; }
  .cd-message { font-size:13px; color:var(--text-secondary); line-height:1.6; }

  .cd-services {
    margin-top:10px; border-radius:9px;
    border:1px solid var(--glass-border);
    background:var(--bg-elev-2); overflow:hidden;
  }
  .cd-services-header {
    display:flex; align-items:center; justify-content:space-between;
    padding:8px 12px; border-bottom:1px solid var(--glass-border);
  }
  .cd-services-label {
    font-size:10px; font-weight:600; letter-spacing:.07em;
    text-transform:uppercase; color:var(--text-muted);
  }
  .cd-service-row {
    display:flex; align-items:center; justify-content:space-between;
    padding:8px 12px; font-size:12px;
  }
  .cd-service-row + .cd-service-row { border-top:1px solid var(--glass-border); }
  .cd-service-name { display:flex; align-items:center; gap:7px; color:var(--text-primary); }
  .cd-service-dot {
    width:6px; height:6px; border-radius:50%; flex-shrink:0;
    background:var(--c-ok); box-shadow:0 0 5px rgba(16,185,129,0.5);
  }
  .cd-service-status { font-size:11px; color:var(--text-muted); }

  .cd-services-block {
    display:flex; align-items:center; justify-content:space-between; gap:10px;
    margin-top:10px; padding:10px 12px; border-radius:8px;
    background:var(--c-warn-dim); border:1px solid var(--c-warn-border);
  }
  .cd-services-block span {
    font-size:11px; color:var(--c-warn); font-weight:500;
  }
  .cd-btn-services {
    padding:5px 12px; border-radius:7px; font-size:11px; font-weight:600;
    cursor:pointer; border:1px solid var(--glass-border);
    background:var(--bg-elev-2); color:var(--text-primary);
    font-family:inherit; transition:all .12s; white-space:nowrap;
  }
  .cd-btn-services:hover { background:var(--bg-elev-1); }

  .cd-confirm-field { margin-top:14px; padding-left:48px; }
  .cd-confirm-label { font-size:11px; color:var(--text-secondary); margin-bottom:6px; line-height:1.5; }
  .cd-confirm-label span {
    font-family:var(--font-mono, 'IBM Plex Mono', monospace); font-size:11px;
    padding:1px 6px; border-radius:4px;
  }
  .cd.danger  .cd-confirm-label span { background:var(--c-crit-dim); color:var(--c-crit); }
  .cd.warning .cd-confirm-label span { background:var(--c-warn-dim); color:var(--c-warn); }
  .cd.default .cd-confirm-label span { background:var(--accent-dim); color:var(--accent); }

  .cd-confirm-input {
    width:100%; background:var(--bg-app);
    border:1px solid var(--glass-border); border-radius:8px;
    padding:9px 12px; font-size:13px;
    font-family:var(--font-mono, 'IBM Plex Mono', monospace); color:var(--text-primary);
    outline:none; transition:border-color .15s, box-shadow .15s;
  }
  .cd-confirm-input::placeholder { color:var(--text-muted); }
  .cd-confirm-input:focus { border-color:rgba(255,255,255,0.15); box-shadow:0 0 0 3px rgba(255,255,255,0.03); }
  .cd.danger  .cd-confirm-input.valid { border-color:var(--c-ok); box-shadow:0 0 0 3px rgba(16,185,129,0.10); }
  .cd.warning .cd-confirm-input.valid { border-color:var(--c-ok); box-shadow:0 0 0 3px rgba(16,185,129,0.10); }

  .cd-actions { display:flex; justify-content:flex-end; gap:8px; margin-top:20px; }

  .cd-btn {
    padding:9px 20px; border-radius:9px; font-size:13px; font-weight:500;
    cursor:pointer; border:1px solid transparent; transition:all .12s;
    outline:none; font-family:inherit;
  }
  .cd-btn:disabled { opacity:.4; cursor:not-allowed; }

  .cd-btn-cancel {
    background:var(--bg-elev-2); border-color:var(--glass-border);
    color:var(--text-secondary);
  }
  .cd-btn-cancel:hover { background:var(--bg-elev-1); color:var(--text-primary); }

  .cd-btn-default { background:var(--accent); color:#fff; }
  .cd-btn-default:hover:not(:disabled) { filter:brightness(1.1); }

  .cd-btn-danger { background:var(--c-crit); color:#fff; }
  .cd-btn-danger:hover:not(:disabled) { filter:brightness(1.1); }

  .cd-btn-warning { background:var(--c-warn); color:#000; font-weight:600; }
  .cd-btn-warning:hover:not(:disabled) { filter:brightness(1.1); }
</style>
