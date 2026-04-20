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
    background: var(--bg);
    font-family: var(--font-sans);
    color: var(--fg);
    display: flex;
    flex-direction: column;
    min-width: 780px;
    overflow: hidden;
  }

  /* ─── Titlebar ─── */
  .titlebar {
    height: var(--titlebar-height);
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    font-family: var(--font-mono);
    font-size: 11px;
    user-select: none;
    flex-shrink: 0;
  }
  .tb-left {
    padding: 0 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1;
    min-width: 0;
  }
  .tb-logo {
    display: inline-block;
    width: 14px;
    height: 14px;
    background: var(--accent);
    clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
    flex-shrink: 0;
  }
  .tb-path {
    color: var(--fg);
    letter-spacing: 0.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tb-path .scheme  { color: var(--fg-mute); }
  .tb-path .host    { color: var(--accent); font-weight: 500; }
  .tb-path .sep     { color: var(--fg-faint); margin: 0 6px; }
  .tb-path .seg     { color: var(--fg-dim); }
  .tb-path .current { color: var(--fg); font-weight: 500; }

  .tb-right {
    display: flex;
    align-items: center;
  }
  .tb-actions {
    display: flex;
    gap: 6px;
    padding: 0 8px;
  }

  /* Window controls */
  .wc-bar {
    display: flex;
    gap: 3px;
    padding: 2px 4px 2px 0;
  }
  .wc-btn {
    width: 34px;
    height: 22px;
    background: var(--bg-1);
    color: var(--fg-dim);
    border: 1px solid var(--border-bright);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--font-mono);
    font-size: 11px;
    transition: color 0.12s, background 0.12s, border-color 0.12s;
    clip-path: polygon(
      0 0, calc(100% - 6px) 0, 100% 6px,
      100% 100%, 6px 100%, 0 calc(100% - 6px)
    );
  }
  .wc-btn:hover {
    color: var(--bg);
    background: var(--accent);
    border-color: var(--accent);
  }
  .wc-btn.close:hover {
    color: var(--bg);
    background: var(--crit);
    border-color: var(--crit);
  }

  /* ─── App body ─── */
  .app-body {
    flex: 1;
    display: grid;
    grid-template-columns: var(--sidebar-width) 1fr;
    overflow: hidden;
    min-height: 0;
  }

  /* ─── Sidebar ─── */
  .sidebar {
    background: var(--bg-1);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    font-family: var(--font-mono);
    font-size: 12px;
    overflow: hidden;
  }
  .sb-header {
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }
  .sb-header-icon {
    width: 22px;
    height: 22px;
    border: 1px solid var(--accent);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 11px;
    clip-path: polygon(
      0 0, calc(100% - 4px) 0, 100% 4px,
      100% 100%, 4px 100%, 0 calc(100% - 4px)
    );
  }
  .sb-title {
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 1px;
    text-transform: uppercase;
    font-size: 11px;
  }

  .sb-scroll {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 10px;
  }

  .sb-section {
    padding: 16px 16px 6px;
    font-size: 9px;
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1.8px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .sb-section::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--border);
  }

  .sb-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 16px;
    color: var(--fg-dim);
    cursor: pointer;
    border-left: 2px solid transparent;
    transition: all 0.1s;
    font-size: 11px;
  }
  .sb-item:hover {
    background: var(--bg-2);
    color: var(--fg);
  }
  .sb-item.active {
    background: var(--bg-2);
    color: var(--accent);
    border-left-color: var(--accent);
  }
  .sb-prefix {
    color: var(--fg-faint);
    font-size: 10px;
    width: 10px;
    flex-shrink: 0;
  }
  .sb-item.active .sb-prefix { color: var(--accent); }
  .sb-label {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Sidebar footer */
  .sb-footer {
    padding: 10px 16px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 3px;
    font-size: 9px;
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    flex-shrink: 0;
    background: var(--bg-1);
  }
  .sb-footer-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .sb-footer .k { color: var(--fg-mute); }
  .sb-footer .v { color: var(--fg-dim); margin-left: auto; }

  /* ─── Main ─── */
  .main {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg);
    min-width: 0;
  }
  .content {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }

  /* Page header opcional: título y descripción debajo del titlebar.
     Solo se muestra si la app pasa contenido al slot "page-header". */
  .page-header {
    padding: 14px 22px;
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--fg);
    letter-spacing: 0.3px;
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
    font-size: 11px;
    font-weight: normal;
    letter-spacing: 0.2px;
  }
  .page-header :global(.ph-right) {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .inner-footer {
    height: 26px;
    border-top: 1px solid var(--border);
    background: var(--bg-1);
    display: flex;
    align-items: center;
    padding: 0 14px;
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--fg-mute);
    letter-spacing: 0.8px;
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
