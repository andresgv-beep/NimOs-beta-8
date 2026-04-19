<script>
  import { user } from '$lib/stores/auth.js';
  import StoragePanel from '$lib/apps/StoragePanel.svelte';

  let activeTab = 'resumen';

  const sidebarItems = [
    { id: 'resumen',   label: 'Resumen',                icon: 'resumen'  },
    { id: 'disks',     label: 'Discos',                  icon: 'disk'     },
    { id: 'snapshots', label: 'Puntos de restauración',  icon: 'snapshot' },
  ];
  const maintItems = [
    { id: 'health',    label: 'Salud',                   icon: 'health'   },
    { id: 'restore',   label: 'Restaurar volumen',       icon: 'restore'  },
  ];

  $: userName = $user?.username || 'User';
  $: userRole = $user?.role     || 'user';

  const tabLabel = { resumen:'Resumen', detalle:'Gestionar', disks:'Discos', pools:'Crear volumen', health:'Salud', restore:'Restaurar volumen', snapshots:'Puntos de restauración' };
</script>

<div class="app">
  <!-- ── SIDEBAR ── -->
  <aside class="sidebar">
    <div class="sb-header">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <ellipse cx="12" cy="5" rx="9" ry="3"/>
        <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
        <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/>
      </svg>
      <span class="sb-title">Almacenamiento</span>
    </div>

    <div class="sb-search">⌕ Buscar…</div>

    <div class="sb-label">Almacenamiento</div>
    {#each sidebarItems as item}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="sb-item" class:active={activeTab === item.id} on:click={() => activeTab = item.id}>
        <svg class="sb-icon" viewBox="0 0 24 24">
          {#if item.icon === 'resumen'}
            <rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>
          {:else if item.icon === 'disk'}
            <circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="2.5"/>
          {:else if item.icon === 'snapshot'}
            <polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>
          {/if}
        </svg>
        {item.label}
      </div>
    {/each}

    <div class="sb-sep"></div>
    <div class="sb-label">Mantenimiento</div>
    {#each maintItems as item}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="sb-item" class:active={activeTab === item.id} on:click={() => activeTab = item.id}>
        <svg class="sb-icon" viewBox="0 0 24 24">
          {#if item.icon === 'health'}
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          {:else if item.icon === 'restore'}
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
          {/if}
        </svg>
        {item.label}
      </div>
    {/each}

    <div class="sb-footer">
      <div class="sb-user">
        <div class="sb-avatar">{userName[0].toUpperCase()}</div>
        <div>
          <div class="sb-user-name">{userName}</div>
          <div class="sb-user-role">{userRole}</div>
        </div>
      </div>
    </div>
  </aside>

  <!-- ── MAIN ── -->
  <div class="main">
    <div class="topbar">
      <span class="tb-title">Almacenamiento</span>
      <span class="tb-sep"></span>
      <span class="tb-sub">{tabLabel[activeTab] || ''}</span>
    </div>

    <div class="content">
      <StoragePanel bind:activeTab />
    </div>

    <div class="statusbar">
      <div class="status-dot"></div>
      <span>NimOS v7.0-beta</span>
    </div>
  </div>
</div>

<style>
  /* ═══════════════════════════════════
     StorageApp — flat layout
     Sidebar (--bg-sidebar) | Main (--bg-panel)
     No inner-wrap. No double background.
     ═══════════════════════════════════ */
  .app {
    width: 100%; height: 100%;
    display: flex; overflow: hidden;
    font-family: var(--font);
    color: var(--text-1);
  }

  /* ── Sidebar ── */
  .sidebar {
    width: 200px; flex-shrink: 0;
    display: flex; flex-direction: column;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border);
    padding: 44px 8px 10px;
  }
  .sb-header {
    display: flex; align-items: center; gap: 9px;
    padding: 4px 10px 16px;
  }
  .sb-header svg { color: var(--text-0); flex-shrink: 0; }
  .sb-title { font-size: 14px; font-weight: 800; color: var(--text-0); letter-spacing: -0.02em; }

  .sb-search {
    display: flex; align-items: center; gap: 6px;
    padding: 6px 10px; border-radius: 6px;
    border: 1px solid var(--border); background: var(--bg-panel);
    font-size: 11px; color: var(--text-3);
    margin-bottom: 12px; cursor: text;
  }
  .sb-label {
    font-size: 9.5px; font-weight: 700; color: var(--text-3);
    letter-spacing: 0.08em; text-transform: uppercase;
    padding: 0 10px 4px; margin-top: 4px;
  }
  .sb-item {
    display: flex; align-items: center; gap: 9px;
    padding: 7px 10px; border-radius: 6px;
    font-size: 12.5px; font-weight: 500; color: var(--text-2);
    cursor: pointer; transition: all 0.15s;
    border: 1px solid transparent; margin-bottom: 1px;
  }
  .sb-item:hover { background: rgba(255,255,255,0.04); color: var(--text-1); }
  .sb-item.active {
    background: var(--accent-dim);
    color: var(--accent);
    border-color: rgba(255,255,255,0.04);
  }
  .sb-icon {
    width: 15px; height: 15px; flex-shrink: 0;
    stroke: currentColor; fill: none;
    stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round;
    opacity: 0.5;
  }
  .sb-item.active .sb-icon { opacity: 1; }
  .sb-sep { height: 1px; background: var(--border); margin: 8px 10px; }

  .sb-footer { margin-top: auto; border-top: 1px solid var(--border); padding-top: 10px; }
  .sb-user { display: flex; align-items: center; gap: 10px; padding: 8px 10px; }
  .sb-avatar {
    width: 30px; height: 30px; border-radius: 8px; flex-shrink: 0;
    background: var(--accent);
    display: flex; align-items: center; justify-content: center;
    font-size: 12px; font-weight: 700; color: white;
  }
  .sb-user-name { font-size: 12px; font-weight: 600; color: var(--text-0); }
  .sb-user-role { font-size: 9.5px; color: var(--text-3); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 600; }

  /* ── Main ── */
  .main {
    flex: 1; display: flex; flex-direction: column;
    background: var(--bg-panel);
    overflow: hidden;
  }
  .topbar {
    display: flex; align-items: center; gap: 10px;
    padding: 12px 20px;
    background: var(--bg-bar);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .tb-title { font-size: 13px; font-weight: 700; color: var(--text-0); }
  .tb-sep { width: 1px; height: 14px; background: var(--border-hi); }
  .tb-sub { font-size: 11.5px; color: var(--text-3); font-weight: 500; }

  .content {
    flex: 1; overflow: hidden;
    display: flex; flex-direction: column;
  }

  .statusbar {
    display: flex; align-items: center; gap: 14px;
    padding: 6px 20px;
    border-top: 1px solid var(--border);
    background: var(--bg-bar);
    font-size: 10.5px; font-family: var(--mono);
    color: var(--text-3); flex-shrink: 0;
  }
  .status-dot {
    width: 4px; height: 4px; border-radius: 50%;
    background: var(--green);
    box-shadow: 0 0 4px rgba(34,197,94,0.5);
  }
</style>
