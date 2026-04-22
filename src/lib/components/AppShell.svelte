<script>
  /**
   * AppShell · Envoltorio estándar de apps NimOS
   * ──────────────────────────────────────────────
   * Provee el chrome común: titlebar con path, sidebar con secciones, footer.
   *
   * Uso:
   *   <AppShell
   *     appId="nimhealth"
   *     title="NimHealth"
   *     headerIcon="♥"
   *     sections={[
   *       { label: 'Monitor', items: [
   *         { id: 'task', label: 'Task Manager', keyHint: 'T' },
   *         { id: 'system', label: 'Sistema', keyHint: 'S' },
   *       ]},
   *     ]}
   *     bind:active
   *     pathSegments={['health', 'task-manager']}
   *   >
   *     <svelte:fragment slot="page-header">
   *       <b>Remote Access</b>
   *       <span class="ph-desc">· exposición y acceso remoto</span>
   *     </svelte:fragment>
   *
   *     <svelte:fragment slot="toolbar">
   *       [toolbar custom]
   *     </svelte:fragment>
   *
   *     [contenido principal de la app]
   *
   *     <svelte:fragment slot="footer">
   *       <span class="k">items:</span> <span class="v">13</span>
   *     </svelte:fragment>
   *   </AppShell>
   */
  import { getContext } from 'svelte';
  import { user } from '$lib/stores/auth.js';
  import LED from '$lib/ui/LED.svelte';
  import KeyBind from '$lib/ui/KeyBind.svelte';
  import Badge from '$lib/ui/Badge.svelte';

  export let appId = '';
  export let title = '';
  export let headerIcon = '◆';
  export let sections = [];
  export let active = '';
  /** pathSegments: segmentos del path tras el host, ej ['health','task-manager'] */
  export let pathSegments = [];
  /** Footer interno que muestra daemon status + versión */
  export let showDaemonStatus = true;

  const wc = getContext('windowControls');

  $: hostname = typeof window !== 'undefined' ? (window.location.hostname || 'nimos') : 'nimos';
  $: userName = $user?.username || 'user';

  function handleItem(itemId) {
    active = itemId;
  }
</script>

<div class="app-shell">
  <!-- ─── Titlebar ─── -->
  <div class="titlebar">
    <div class="tb-left">
      <span class="tb-logo"></span>
      <span class="tb-path">
        <span class="scheme">nimos://</span><span class="host">{hostname}</span>
        {#each pathSegments as seg, i}
          <span class="sep">/</span>
          {#if i === pathSegments.length - 1}
            <span class="current">{seg}</span>
          {:else}
            <span class="seg">{seg}</span>
          {/if}
        {/each}
      </span>
    </div>
    <div class="tb-right">
      <div class="tb-actions">
        <slot name="titlebar-actions" />
      </div>
      {#if wc}
        <div class="wc-bar">
          <button class="wc-btn" on:click={wc.minimize} title="Minimizar">−</button>
          <button class="wc-btn" on:click={wc.maximize} title="Maximizar">□</button>
          <button class="wc-btn close" on:click={wc.close} title="Cerrar">×</button>
        </div>
      {/if}
    </div>
  </div>

  <!-- ─── App Body ─── -->
  <div class="app-body">

    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sb-header">
        <div class="sb-header-icon">{headerIcon}</div>
        <div class="sb-title">{title}</div>
      </div>

      <div class="sb-scroll">
        {#each sections as section}
          <div class="sb-section">
            <span>{section.label}</span>
          </div>
          {#each section.items as item}
            <div
              class="sb-item"
              class:active={active === item.id}
              on:click={() => handleItem(item.id)}
              on:keydown={(e) => e.key === 'Enter' && handleItem(item.id)}
              role="button"
              tabindex="0"
            >
              <span class="sb-prefix">{active === item.id ? '▸' : '\u00A0'}</span>
              <span class="sb-label">{item.label}</span>
              {#if item.badge !== undefined && item.badge !== null && item.badge !== 0}
                <Badge size="sm" variant={item.badgeVariant || 'default'}>{item.badge}</Badge>
              {/if}
              {#if item.keyHint}
                <KeyBind key={item.keyHint} active={active === item.id} />
              {/if}
            </div>
          {/each}
        {/each}
      </div>

      {#if showDaemonStatus}
        <div class="sb-footer">
          <div class="sb-footer-row">
            <LED size={7} />
            <span class="k">daemon</span>
            <span class="v">running</span>
          </div>
          <div class="sb-footer-row">
            <span class="k">user</span>
            <span class="v">{userName}</span>
          </div>
        </div>
      {/if}
    </aside>

    <!-- Main -->
    <div class="main">
      {#if $$slots['page-header']}
        <div class="page-header">
          <slot name="page-header" />
        </div>
      {/if}
      <slot name="toolbar" />
      <div class="content">
        <slot />
      </div>
      <slot name="footer-raw" />
      {#if $$slots.footer}
        <div class="inner-footer">
          <div class="left">
            <slot name="footer" />
          </div>
          <div class="right">
            <slot name="footer-right" />
          </div>
        </div>
      {/if}
    </div>

  </div>
</div>

<style>
  .app-shell {
    width: 100%;
    height: 100%;
    background: transparent;
    font-family: var(--font-sans);
    color: var(--fg);
    display: flex;
    flex-direction: column;
    min-width: 780px;
    overflow: hidden;
  }

  /* ═══════════════════════════════════════════════════════════
     TITLEBAR · glass sutil con path
     ═══════════════════════════════════════════════════════════ */
  .titlebar {
    height: var(--titlebar-height);
    background: transparent;
    display: flex;
    align-items: center;
    font-family: var(--font-sans);
    font-size: 12px;
    user-select: none;
    flex-shrink: 0;
  }
  .tb-left {
    padding: 0 16px;
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1;
    min-width: 0;
  }
  .tb-logo {
    display: inline-block;
    width: 12px;
    height: 12px;
    background: var(--accent);
    border-radius: 3px;
    flex-shrink: 0;
    box-shadow: 0 0 6px var(--accent-glow);
  }
  .tb-path {
    color: var(--fg-dim);
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.3px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tb-path .scheme  { color: var(--fg-faint); }
  .tb-path .host    { color: var(--fg-mute); font-weight: 500; }
  .tb-path .sep     { color: var(--fg-faint); margin: 0 6px; }
  .tb-path .seg     { color: var(--fg-mute); }
  .tb-path .current { color: var(--fg); font-weight: 500; }

  .tb-right {
    display: flex;
    align-items: center;
  }
  .tb-actions {
    display: flex;
    gap: 6px;
    padding: 0 10px;
  }

  /* Window controls · estilo macOS traffic lights */
  .wc-bar {
    display: flex;
    gap: 8px;
    padding: 0 14px;
  }
  .wc-btn {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.18);
    color: rgba(0, 0, 0, 0);
    border: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 9px;
    font-weight: 700;
    line-height: 1;
    transition: background 0.12s, transform 0.12s, color 0.12s;
    padding: 0;
  }
  .wc-btn:nth-child(1) { background: rgba(255, 189, 46, 0.85); }  /* minimize · amarillo */
  .wc-btn:nth-child(2) { background: rgba(39, 201, 63, 0.85); }   /* maximize · verde */
  .wc-btn.close        { background: rgba(255, 95, 86, 0.85); }   /* close · rojo */
  .wc-bar:hover .wc-btn {
    color: rgba(0, 0, 0, 0.55);
  }
  .wc-btn:hover {
    transform: scale(1.08);
  }
  .wc-btn:active {
    transform: scale(0.94);
  }

  /* ═══════════════════════════════════════════════════════════
     APP BODY · sidebar + main
     ═══════════════════════════════════════════════════════════ */
  .app-body {
    flex: 1;
    display: grid;
    grid-template-columns: var(--sidebar-width) 1fr;
    overflow: hidden;
    min-height: 0;
  }

  /* ─── Sidebar · glass translúcido ─── */
  .sidebar {
    background: var(--side-bg);
    border-right: 1px solid var(--side-border);
    display: flex;
    flex-direction: column;
    font-family: var(--font-sans);
    font-size: 13px;
    overflow: hidden;
  }
  .sb-header {
    padding: 16px 16px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }
  .sb-header-icon {
    width: 26px;
    height: 26px;
    background: var(--accent-dim);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    font-size: 13px;
    border-radius: 6px;
  }
  .sb-title {
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 0.5px;
    text-transform: uppercase;
    font-size: 11px;
  }

  .sb-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 2px 10px 10px;
  }

  .sb-section {
    padding: 14px 6px 6px;
    font-size: 10px;
    color: var(--fg-faint);
    text-transform: uppercase;
    letter-spacing: 1.5px;
    font-weight: 600;
  }

  .sb-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    margin: 1px 0;
    color: var(--fg-dim);
    cursor: pointer;
    border-radius: 6px;
    transition: background 0.12s, color 0.12s;
    font-size: 13px;
    font-weight: 400;
  }
  .sb-item:hover {
    background: var(--side-hover);
    color: var(--fg);
  }
  .sb-item.active {
    background: var(--side-active-bg);
    color: var(--side-active-fg);
  }
  /* Prefijo ▸ oculto · el fondo sólido del activo ya señaliza */
  .sb-prefix {
    display: none;
  }
  .sb-label {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Sidebar footer · daemon status */
  .sb-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--side-border);
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--fg-mute);
    flex-shrink: 0;
    background: transparent;
  }
  .sb-footer-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .sb-footer .k { color: var(--fg-mute); }
  .sb-footer .v { color: var(--fg-dim); margin-left: auto; font-weight: 500; }

  /* ═══════════════════════════════════════════════════════════
     MAIN · área de contenido glass
     ═══════════════════════════════════════════════════════════ */
  .main {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--main-bg);
    min-width: 0;
  }
  .content {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }

  /* Page header opcional · título y descripción debajo del titlebar */
  .page-header {
    padding: 14px 22px;
    background: transparent;
    font-family: var(--font-sans);
    font-size: 14px;
    color: var(--fg);
    letter-spacing: -0.1px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
  }
  .page-header :global(b),
  .page-header :global(strong) {
    color: var(--fg);
    font-weight: 600;
  }
  .page-header :global(.ph-desc),
  .page-header :global(.ph-path) {
    color: var(--fg-mute);
    font-size: 12px;
    font-weight: 400;
    letter-spacing: 0;
  }
  .page-header :global(.ph-right) {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  /* Inner footer (status bar interno de la app) */
  .inner-footer {
    height: 30px;
    background: transparent;
    display: flex;
    align-items: center;
    padding: 0 18px;
    font-family: var(--font-sans);
    font-size: 11px;
    color: var(--fg-mute);
    letter-spacing: 0;
    flex-shrink: 0;
  }
  .inner-footer .left, .inner-footer .right {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .inner-footer .left  { flex: 1; }
  .inner-footer .right { margin-left: auto; }
</style>
