<script>
  import { getContext } from 'svelte';
  import { user } from '$lib/stores/auth.js';

  /**
   * AppShell — Standard NimOS app layout.
   */
  export let title = '';
  export let appIcon = [];
  export let appIconUrl = '';
  export let sections = [];
  export let active = '';
  export let showSearch = false;

  const wc = getContext('windowControls');

  $: userName = $user?.username || 'User';
  $: userRole = $user?.role || 'user';

  $: subtitle = findLabel(active);

  function findLabel(id) {
    for (const s of sections) {
      for (const item of s.items) {
        if (item.id === id) return item.label;
      }
    }
    return '';
  }
</script>

<div class="app-shell">
  <aside class="sidebar">
    <!-- App header -->
    <div class="sb-header">
      {#if appIconUrl}
        <img class="sb-app-img" src={appIconUrl} alt={title} />
      {:else}
        <svg class="sb-app-icon" viewBox="0 0 24 24">
          {#each appIcon as el}
            {#if el.tag === 'path'}<path {...el.attrs}/>
            {:else if el.tag === 'ellipse'}<ellipse {...el.attrs}/>
            {:else if el.tag === 'circle'}<circle {...el.attrs}/>
            {:else if el.tag === 'rect'}<rect {...el.attrs}/>
            {:else if el.tag === 'polyline'}<polyline {...el.attrs}/>
            {:else if el.tag === 'line'}<line {...el.attrs}/>
            {/if}
          {/each}
        </svg>
      {/if}
      <span class="sb-app-title">{title}</span>
    </div>

    {#if showSearch}
      <div class="sb-search">⌕ Buscar…</div>
    {/if}

    {#each sections as section, si}
      <div class="sb-section" style={si > 0 ? 'margin-top:8px' : ''}>{section.label}</div>
      {#each section.items as item}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="sb-item"
          class:active={active === item.id}
          on:click={() => active = item.id}
        >
          {#if item.paths && item.paths.length > 0}
            <svg class="sb-icon" viewBox="0 0 24 24">
              {#each item.paths as el}
                {#if el.tag === 'path'}<path {...el.attrs}/>
                {:else if el.tag === 'circle'}<circle {...el.attrs}/>
                {:else if el.tag === 'rect'}<rect {...el.attrs}/>
                {:else if el.tag === 'polyline'}<polyline {...el.attrs}/>
                {:else if el.tag === 'line'}<line {...el.attrs}/>
                {:else if el.tag === 'ellipse'}<ellipse {...el.attrs}/>
                {/if}
              {/each}
            </svg>
          {/if}
          {item.label}
          {#if item.badge !== undefined && item.badge !== null && item.badge !== 0}
            <span class="sb-badge" class:sb-badge-accent={item.badgeColor === 'accent'} class:sb-badge-ok={item.badgeColor === 'ok'} class:sb-badge-warn={item.badgeColor === 'warn'} class:sb-badge-crit={item.badgeColor === 'crit'}>{item.badge}</span>
          {/if}
        </div>
      {/each}
    {/each}

    <div class="sb-bottom">
      <div class="sb-user-card">
        <div class="sb-avatar">{userName[0].toUpperCase()}</div>
        <div class="sb-user-info">
          <div class="sb-user-name">{userName}</div>
          <div class="sb-user-role">{userRole}</div>
        </div>
      </div>
    </div>
  </aside>

  <div class="main">
    <div class="titlebar">
      <span class="tb-title">{subtitle || title}</span>
      <div class="tb-actions">
        <slot name="titlebar-actions" />
      </div>
      {#if wc}
        <div class="wf-controls">
          <button class="wf-btn" on:click={wc.minimize} title="Minimizar">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="5" y1="12" x2="19" y2="12"/></svg>
          </button>
          <button class="wf-btn" on:click={wc.maximize} title="Maximizar">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
          </button>
          <button class="wf-btn wf-close" on:click={wc.close} title="Cerrar">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
      {/if}
    </div>
    <slot name="toolbar" />
    <div class="content">
      <slot />
    </div>
  </div>
</div>

<style>
  .app-shell {
    width: 100%; height: 100%;
    min-height: 0;
    display: grid;
    grid-template-columns: var(--sidebar-width) 1fr;
    background: var(--bg-elev-1);
    font-family: var(--font-sans);
    color: var(--text-primary);
    min-width: 780px;
  }

  /* ── Sidebar ── */
  .sidebar {
    background: var(--bg-elev-1);
    border-right: 1px solid var(--glass-border);
    padding: 18px 14px;
    display: flex; flex-direction: column; gap: 3px;
    overflow-y: auto;
  }
  .sidebar::-webkit-scrollbar { width: 3px; }
  .sidebar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.06); border-radius: 3px; }

  .sb-header {
    display: flex; align-items: center; gap: 12px;
    padding: 4px 12px 16px;
  }
  .sb-app-icon {
    width: 22px; height: 22px; flex-shrink: 0;
    stroke: var(--text-primary); fill: none;
    stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round;
  }
  .sb-app-img {
    width: 24px; height: 24px; flex-shrink: 0;
    object-fit: contain;
  }
  .sb-app-title {
    font-size: 16px; font-weight: 700;
    color: var(--text-primary); letter-spacing: -0.3px;
  }

  .sb-search {
    display: flex; align-items: center; gap: 6px;
    padding: 8px 12px; border-radius: var(--radius-sm);
    border: 1px solid var(--glass-border); background: var(--bg-elev-2);
    font-size: 12px; color: var(--text-muted);
    margin-bottom: 10px; cursor: text;
  }

  .sb-section {
    font-size: 10px; font-weight: 500; color: var(--text-muted);
    text-transform: uppercase; letter-spacing: 1.2px;
    padding: 14px 12px 6px;
  }
  .sb-section:first-child { padding-top: 4px; }

  .sb-item {
    display: flex; align-items: center; gap: 12px;
    padding: 9px 12px; border-radius: 8px;
    font-size: 13px; font-weight: 500; color: var(--text-secondary);
    cursor: pointer; transition: all 0.15s;
  }
  .sb-item:hover { background: var(--bg-elev-2); color: var(--text-primary); }
  .sb-item.active { background: var(--accent-dim); color: var(--accent); }

  /* Duotone icon system */
  .sb-icon {
    width: 16px; height: 16px; flex-shrink: 0;
    stroke: currentColor; fill: none;
    stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round;
  }
  .sb-item .sb-icon :global([data-fill]) {
    fill: currentColor; opacity: 0.12; stroke: none;
  }
  .sb-item.active .sb-icon :global([data-fill]) { opacity: 0.2; }
  .sb-item .sb-icon :global([data-dot]) {
    fill: currentColor; opacity: 0.45; stroke: none;
  }
  .sb-item.active .sb-icon :global([data-dot]) { opacity: 0.7; }

  /* Badges */
  .sb-badge {
    margin-left: auto;
    padding: 1px 6px; border-radius: 10px;
    font-size: 10px; font-weight: 600;
    font-family: var(--font-mono, monospace);
    background: var(--bg-elev-2); color: var(--text-muted);
    line-height: 1.4;
  }
  .sb-item.active .sb-badge { background: rgba(59,130,246,0.15); color: var(--accent); }
  .sb-badge-accent { background: rgba(59,130,246,0.12) !important; color: var(--accent) !important; }
  .sb-badge-ok { background: var(--c-ok-dim) !important; color: var(--c-ok) !important; }
  .sb-badge-warn { background: var(--c-warn-dim) !important; color: var(--c-warn) !important; }
  .sb-badge-crit { background: var(--c-crit-dim) !important; color: var(--c-crit) !important; }

  /* User card */
  .sb-bottom {
    margin-top: auto;
    border-top: 1px solid var(--glass-border);
    padding-top: 10px;
  }
  .sb-user-card {
    display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  }
  .sb-avatar {
    width: 30px; height: 30px; border-radius: 8px;
    background: var(--accent);
    display: flex; align-items: center; justify-content: center;
    font-size: 12px; font-weight: 700; color: #fff; flex-shrink: 0;
  }
  .sb-user-name { font-size: 12px; font-weight: 600; color: var(--text-primary); }
  .sb-user-role {
    font-size: 9.5px; color: var(--text-muted);
    text-transform: uppercase; letter-spacing: 0.5px; font-weight: 500;
  }

  /* ── Main ── */
  .main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    height: 100%;
    overflow: hidden;
  }

  .titlebar {
    display: flex; align-items: center; gap: 14px;
    padding: 0 4px 0 24px; height: var(--titlebar-height);
    background: var(--bg-elev-1);
    border-bottom: 1px solid var(--glass-border);
    flex-shrink: 0;
  }
  .tb-title {
    font-size: 14px; font-weight: 600; color: var(--text-primary);
    letter-spacing: -0.2px; white-space: nowrap;
  }
  .tb-actions {
    margin-left: auto;
    display: flex; align-items: center; gap: 8px;
    min-width: 0;
  }

  /* Window controls */
  .wf-controls {
    display: flex; align-items: center;
    flex-shrink: 0;
  }
  .wf-btn {
    width: 36px; height: 30px;
    border: none; background: transparent; padding: 0;
    color: var(--text-muted);
    cursor: pointer; display: flex; align-items: center; justify-content: center;
    transition: background 0.12s, color 0.12s;
    border-radius: 6px;
  }
  .wf-btn svg { width: 14px; height: 14px; }
  .wf-btn:hover { background: rgba(255,255,255,0.06); color: var(--text-primary); }
  .wf-close:hover { background: rgba(239,68,68,0.8); color: #fff; }

  .content {
    flex: 1; overflow-y: auto;
    background: var(--bg-app); padding: 24px 28px;
    min-height: 0;
  }
  .content::-webkit-scrollbar {
    width: 10px;
  }
  .content::-webkit-scrollbar-track {
    background: transparent;
  }
  .content::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.10);
    border-radius: 5px;
    border: 2px solid transparent;
    background-clip: padding-box;
  }
  .content::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.22);
    background-clip: padding-box;
  }
  /* Firefox */
  .content {
    scrollbar-width: thin;
    scrollbar-color: rgba(255, 255, 255, 0.14) transparent;
  }
</style>
