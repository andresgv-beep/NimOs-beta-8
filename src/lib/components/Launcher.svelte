<script>
  /**
   * Launcher · Popover de apps
   * ────────────────────────────
   * Sale encima del botón launcher del taskbar.
   * Muestra grid de apps agrupadas por categoría con tabs de filtro y búsqueda.
   */
  import { APP_META, listAllApps } from '$lib/apps.js';
  import { openWindow, windowList } from '$lib/stores/windows.js';
  import { getToken } from '$lib/stores/auth.js';
  import Badge from '$lib/ui/Badge.svelte';
  import KeyBind from '$lib/ui/KeyBind.svelte';

  export let visible = false;

  let filter = 'all'; // 'all' | 'system' | 'utilities' | 'docker'
  let searchTerm = '';
  let dockerApps = [];
  let allowedApps = null; // null = loading, 'all' = admin, string[] = user-specific
  let searchEl;

  $: if (visible) {
    filter = 'all';
    searchTerm = '';
    loadDockerApps();
    loadMyApps();
    setTimeout(() => searchEl?.focus(), 50);
  }

  async function loadMyApps() {
    try {
      const res = await fetch('/api/my-apps', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      const data = await res.json();
      allowedApps = data.apps;
    } catch {
      allowedApps = 'all';
    }
  }

  async function loadDockerApps() {
    try {
      const res = await fetch('/api/docker/installed-apps', {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      });
      const data = await res.json();
      if (data.apps && Array.isArray(data.apps)) {
        dockerApps = data.apps.map(app => ({
          id: app.id,
          name: app.name,
          icon: app.icon || '📦',
          fallback: '📦',
          port: app.port,
          isWebApp: true,
          external: app.external || false,
          category: 'docker',
          running: app.running || false,
        }));
      }
    } catch {}
  }

  function canAccess(appId) {
    if (allowedApps === 'all') return true;
    if (Array.isArray(allowedApps)) return allowedApps.includes(appId);
    return true;
  }

  $: systemApps = listAllApps().map(a => ({ ...a, isSystem: true }));

  $: allApps = (() => {
    const seen = new Set();
    return [...systemApps, ...dockerApps].filter(app => {
      if (seen.has(app.id)) return false;
      if (app.hidden) return false;
      if (!canAccess(app.id)) return false;
      seen.add(app.id);
      return true;
    });
  })();

  $: filteredApps = (() => {
    let list = allApps;

    if (filter !== 'all') {
      list = list.filter(a => a.category === filter);
    }

    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      list = list.filter(a =>
        a.name.toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q)
      );
    }

    return list;
  })();

  $: systemCount    = allApps.filter(a => a.category === 'system').length;
  $: utilitiesCount = allApps.filter(a => a.category === 'utilities').length;
  $: dockerCount    = allApps.filter(a => a.category === 'docker').length;
  $: openAppIds     = new Set($windowList.map(w => w.appId));

  // Groups when filter is 'all'
  $: groups = filter === 'all' && !searchTerm
    ? [
        { label: 'Sistema',    apps: filteredApps.filter(a => a.category === 'system') },
        { label: 'Utilidades', apps: filteredApps.filter(a => a.category === 'utilities') },
        { label: 'Docker',     apps: filteredApps.filter(a => a.category === 'docker') },
      ].filter(g => g.apps.length > 0)
    : [{ label: '', apps: filteredApps }];

  function launch(app) {
    visible = false;
    if (app.isWebApp) {
      if (app.external) {
        window.open(`${window.location.protocol}//${window.location.hostname}:${app.port}`, '_blank');
        return;
      }
      openWindow(app.id, { width: 1100, height: 700 }, {
        isWebApp: true,
        port: app.port,
        appName: app.name,
      });
    } else {
      const meta = APP_META[app.id];
      openWindow(app.id, { width: meta?.width || 800, height: meta?.height || 520 });
    }
  }

  function isIconUrl(icon) {
    return icon && (icon.startsWith('http') || icon.startsWith('/'));
  }

  function handleKeydown(e) {
    if (!visible) return;
    if (e.key === 'Escape') {
      visible = false;
    } else if (e.key === 'Enter' && filteredApps.length > 0) {
      launch(filteredApps[0]);
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if visible}
  <div class="overlay" on:click={() => visible = false} role="presentation"></div>

  <div class="launcher" on:click|stopPropagation role="presentation">
    <div class="lch-inner">

    <!-- Header -->
    <div class="lch-header">
      <div class="lch-title">
        <span class="lch-title-ic">▦</span>
        <span>Apps</span>
        <span class="lch-counter">
          · <span class="c">{allApps.length}</span> instaladas
          {#if openAppIds.size > 0}
            · <span class="c">{openAppIds.size}</span> abiertas
          {/if}
        </span>
      </div>

      <div class="lch-search">
        <span class="lch-search-ic">⌕</span>
        <input
          bind:this={searchEl}
          bind:value={searchTerm}
          placeholder="Buscar app..."
          autocomplete="off"
        />
        <span class="lch-search-key">/</span>
      </div>
    </div>

    <!-- Filter tabs -->
    <div class="lch-tabs">
      <button class="lch-tab" class:active={filter === 'all'} on:click={() => filter = 'all'}>
        <span>Todas</span>
        <span class="tab-n">{allApps.length}</span>
      </button>
      <button class="lch-tab" class:active={filter === 'system'} on:click={() => filter = 'system'}>
        <span>Sistema</span>
        <span class="tab-n">{systemCount}</span>
      </button>
      <button class="lch-tab" class:active={filter === 'utilities'} on:click={() => filter = 'utilities'}>
        <span>Utilidades</span>
        <span class="tab-n">{utilitiesCount}</span>
      </button>
      {#if dockerCount > 0}
        <button class="lch-tab" class:active={filter === 'docker'} on:click={() => filter = 'docker'}>
          <span>Docker</span>
          <span class="tab-n">{dockerCount}</span>
        </button>
      {/if}
    </div>

    <!-- Body -->
    <div class="lch-body">
      {#each groups as group}
        {#if group.label}
          <div class="lch-section-head">
            <span>{group.label}</span>
            <span class="c">· {group.apps.length}</span>
          </div>
        {/if}

        <div class="lch-grid">
          {#each group.apps as app}
            <div
              class="lch-app"
              class:running={openAppIds.has(app.id)}
              on:click={() => launch(app)}
              on:keydown={(e) => e.key === 'Enter' && launch(app)}
              role="button"
              tabindex="0"
              title={app.name}
            >
              <div class="lch-icon">
                {#if isIconUrl(app.icon)}
                  <img src={app.icon} alt={app.name} on:error={(e) => e.target.style.display = 'none'} />
                {:else}
                  <span class="lch-emoji">{app.fallback || app.icon || '📦'}</span>
                {/if}
              </div>
              <div class="lch-app-label">{app.name}</div>
            </div>
          {/each}
        </div>
      {/each}

      {#if filteredApps.length === 0}
        <div class="empty">
          <div class="empty-ic">◌</div>
          <div class="empty-msg">Sin resultados para "<b>{searchTerm}</b>"</div>
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <div class="lch-footer">
      <span class="keyhint"><KeyBind key="↑↓←→" /> <span>navegar</span></span>
      <span class="keyhint"><KeyBind key="↵" /> <span>abrir</span></span>
      <span class="keyhint"><KeyBind key="ESC" /> <span>cerrar</span></span>
    </div>

    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.25);
    backdrop-filter: blur(3px);
    z-index: 9100;
  }

  .launcher {
    position: fixed;
    left: 16px;
    bottom: calc(var(--taskbar-height) + 12px);
    width: 720px;
    max-height: 75vh;
    background: var(--border-bright);   /* color del marco */
    padding: 1px;                         /* grosor del borde */
    box-shadow: 0 0 24px rgba(0, 255, 159, 0.06);
    /* Bevel 14px en esquina inferior-derecha como la ventana */
    clip-path: polygon(
      0 0,
      100% 0,
      100% calc(100% - 14px),
      calc(100% - 14px) 100%,
      0 100%
    );
    font-family: var(--font-mono);
    z-index: 9200;
    display: flex;
    flex-direction: column;
    animation: lch-in 0.18s cubic-bezier(0.16, 1, 0.3, 1) both;
  }

  .lch-inner {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--glass-bg);
    backdrop-filter: blur(24px) saturate(140%);
    -webkit-backdrop-filter: blur(24px) saturate(140%);
    /* Mismo clip-path 1px más pequeño para que la línea del marco quede visible */
    clip-path: polygon(
      0 0,
      100% 0,
      100% calc(100% - 13px),
      calc(100% - 13px) 100%,
      0 100%
    );
  }

  @keyframes lch-in {
    from { opacity: 0; transform: translateY(10px) scale(0.98); }
    to   { opacity: 1; transform: translateY(0) scale(1); }
  }

  /* Header */
  .lch-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px 10px 18px;
    border-bottom: 1px solid var(--border);
    background: rgba(20, 20, 20, 0.4);
    flex-shrink: 0;
  }
  .lch-title {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 10px;
    color: var(--fg);
    letter-spacing: 2px;
    text-transform: uppercase;
    font-weight: 600;
    flex-shrink: 0;
  }
  .lch-title-ic {
    width: 18px; height: 18px;
    border: 1px solid var(--accent);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    font-weight: 700;
    clip-path: polygon(0 0, calc(100% - 3px) 0, 100% 3px, 100% 100%, 3px 100%, 0 calc(100% - 3px));
  }
  .lch-counter {
    color: var(--fg-dim);
    font-size: 9px;
    letter-spacing: 1px;
    font-weight: 400;
    text-transform: none;
  }
  .lch-counter .c { color: var(--accent); font-weight: 600; }

  .lch-search {
    flex: 1;
    height: 26px;
    border: 1px solid var(--border);
    background: rgba(10, 10, 10, 0.6);
    padding: 0 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    clip-path: polygon(0 0, calc(100% - 5px) 0, 100% 5px, 100% 100%, 5px 100%, 0 calc(100% - 5px));
  }
  .lch-search:focus-within { border-color: var(--accent); }
  .lch-search-ic { color: var(--fg-mute); font-size: 11px; }
  .lch-search input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: var(--fg);
    font-family: inherit;
    font-size: 10px;
    letter-spacing: 0.5px;
  }
  .lch-search input::placeholder { color: var(--fg-mute); }
  .lch-search-key {
    font-size: 9px;
    color: var(--fg-faint);
    border: 1px solid var(--border);
    padding: 0 4px;
    letter-spacing: 0.5px;
  }

  /* Tabs */
  .lch-tabs {
    display: flex;
    gap: 4px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    background: rgba(15, 15, 15, 0.3);
    flex-shrink: 0;
  }
  .lch-tab {
    background: transparent;
    border: 1px solid transparent;
    color: var(--fg-dim);
    font-family: inherit;
    font-size: 9px;
    font-weight: 600;
    letter-spacing: 1px;
    text-transform: uppercase;
    padding: 4px 10px;
    cursor: pointer;
    transition: all 0.1s;
    display: flex;
    align-items: center;
    gap: 6px;
    clip-path: polygon(0 0, calc(100% - 4px) 0, 100% 4px, 100% 100%, 4px 100%, 0 calc(100% - 4px));
  }
  .lch-tab:hover { color: var(--fg); background: rgba(255, 255, 255, 0.03); }
  .lch-tab.active {
    color: var(--accent);
    border-color: var(--accent);
    background: rgba(0, 255, 159, 0.06);
  }
  .tab-n {
    font-size: 8px;
    color: var(--fg-faint);
    font-feature-settings: "tnum";
  }
  .lch-tab.active .tab-n { color: var(--accent); }

  /* Body */
  .lch-body {
    padding: 18px 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    overflow-y: auto;
    flex: 1;
  }

  .lch-section-head {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 9px;
    color: var(--fg-mute);
    letter-spacing: 1.8px;
    text-transform: uppercase;
    padding: 0 4px 4px;
  }
  .lch-section-head::before { content: '──'; color: var(--fg-faint); }
  .lch-section-head::after {
    content: '';
    flex: 1;
    height: 1px;
    background: linear-gradient(to right, var(--border) 0%, var(--border) 70%, transparent 100%);
  }
  .lch-section-head .c { color: var(--accent); font-weight: 600; }

  .lch-grid {
    display: grid;
    grid-template-columns: repeat(6, 1fr);
    gap: 6px;
  }

  .lch-app {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 14px 8px 10px;
    cursor: pointer;
    border: 1px solid transparent;
    transition: all 0.1s;
    clip-path: polygon(0 0, calc(100% - 5px) 0, 100% 5px, 100% 100%, 5px 100%, 0 calc(100% - 5px));
  }
  .lch-app:hover {
    border-color: var(--border-bright);
    background: rgba(255, 255, 255, 0.025);
  }
  .lch-app:hover .lch-app-label { color: var(--accent); }

  .lch-icon {
    width: 52px;
    height: 52px;
    display: flex;
    align-items: center;
    justify-content: center;
    filter: drop-shadow(0 3px 6px rgba(0, 0, 0, 0.4));
    transition: transform 0.15s;
  }
  .lch-app:hover .lch-icon { transform: translateY(-2px) scale(1.04); }
  .lch-icon img {
    width: 48px;
    height: 48px;
    object-fit: contain;
  }
  .lch-emoji {
    font-size: 36px;
    line-height: 1;
  }

  .lch-app-label {
    font-family: var(--font-mono);
    font-size: 9.5px;
    color: var(--fg-dim);
    letter-spacing: 0.4px;
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
    transition: color 0.1s;
  }

  /* Running indicator */
  .lch-app.running::after {
    content: '';
    position: absolute;
    bottom: 4px;
    left: 50%;
    transform: translateX(-50%);
    width: 14px;
    height: 2px;
    background: var(--accent);
    box-shadow: 0 0 3px var(--accent-glow);
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 40px 20px;
    color: var(--fg-mute);
  }
  .empty-ic {
    width: 44px; height: 44px;
    border: 1px solid var(--border-bright);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 18px;
    clip-path: polygon(0 0, calc(100% - 5px) 0, 100% 5px, 100% 100%, 5px 100%, 0 calc(100% - 5px));
  }
  .empty-msg {
    font-size: 11px;
    letter-spacing: 0.5px;
  }
  .empty-msg b { color: var(--fg-dim); }

  /* Footer */
  .lch-footer {
    display: flex;
    align-items: center;
    padding: 8px 14px;
    border-top: 1px solid var(--border);
    background: rgba(20, 20, 20, 0.5);
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--fg-mute);
    letter-spacing: 0.6px;
    gap: 14px;
    flex-shrink: 0;
  }
  .keyhint {
    display: flex;
    align-items: center;
    gap: 5px;
  }
</style>
