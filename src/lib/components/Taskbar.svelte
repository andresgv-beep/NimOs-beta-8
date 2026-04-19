<script>
  /**
   * Taskbar · Barra de tareas NimOS v3
   * ───────────────────────────────────
   * - Zona izquierda: launcher + apps ancladas + apps abiertas no ancladas
   * - Zona central:   vacío (deja respirar el wallpaper)
   * - Zona derecha:   systray con Transferencias, Notificaciones, reloj, power
   *
   * Simplificaciones vs Beta 7:
   *   - Solo posición bottom (no top/left)
   *   - Solo modo classic (no dock)
   *   - Sin drag-to-reorder de iconos (Beta 9)
   */
  import { onMount, onDestroy } from 'svelte';
  import { pinnedApps, setPref, prefs } from '$lib/stores/theme.js';
  import {
    windowList, openWindow, focusWindow,
    restoreWindow, minimizeWindow, closeWindow
  } from '$lib/stores/windows.js';
  import { logout } from '$lib/stores/auth.js';
  import { APP_META } from '$lib/apps.js';
  import { unreadCount } from '$lib/stores/notifications.js';
  import { activeTasks } from '$lib/stores/uploadTasks.js';
  import Launcher from './Launcher.svelte';
  import NotificationPanel from './NotificationPanel.svelte';
  import TransferPanel from './TransferPanel.svelte';

  let showLauncher = false;
  let showNotif = false;
  let showTransfers = false;

  // ─── Clock ───
  let now = new Date();
  let clockInterval;

  function tick() { now = new Date(); }

  onMount(() => {
    clockInterval = setInterval(tick, 1000);
    return () => clearInterval(clockInterval);
  });
  onDestroy(() => {
    if (clockInterval) clearInterval(clockInterval);
  });

  $: hh = String(now.getHours()).padStart(2, '0');
  $: mm = String(now.getMinutes()).padStart(2, '0');
  $: ss = String(now.getSeconds()).padStart(2, '0');
  $: dd = String(now.getDate()).padStart(2, '0');
  $: MON = now.toLocaleDateString('es-ES', { month: 'short' }).toUpperCase().replace('.', '');
  $: DOW = now.toLocaleDateString('es-ES', { weekday: 'short' }).toUpperCase().replace('.', '');

  // ─── Context menu (pin/unpin) ───
  let ctxMenu = null;

  function openCtxMenu(e, appId, win = null) {
    e.preventDefault();
    e.stopPropagation();
    ctxMenu = {
      appId,
      win,
      x: Math.min(e.clientX, window.innerWidth - 220),
      y: Math.max(8, e.clientY - 140),
    };
  }
  function closeCtxMenu() { ctxMenu = null; }
  function isPinned(appId) { return $pinnedApps.includes(appId); }
  function togglePin(appId) {
    if (isPinned(appId)) setPref('pinnedApps', $pinnedApps.filter(id => id !== appId));
    else setPref('pinnedApps', [...$pinnedApps, appId]);
    closeCtxMenu();
  }

  // ─── App launch ───
  function handleAppClick(appId) {
    const meta = APP_META[appId];
    const existing = $windowList.find(w => w.appId === appId);
    if (existing) {
      if (existing.minimized) restoreWindow(existing.id);
      else focusWindow(existing.id);
    } else {
      openWindow(appId, { width: meta?.width || 800, height: meta?.height || 520 });
    }
  }
  function toggleMinimize(win) {
    if (win.minimized) restoreWindow(win.id);
    else minimizeWindow(win.id);
  }

  function isIconUrl(icon) { return icon && (icon.startsWith('/') || icon.startsWith('http')); }

  // ─── Apps open not pinned ───
  $: openUnpinned = $windowList.filter(w => !$pinnedApps.includes(w.appId));

  // ─── Transfers activity ───
  $: transferCount = $activeTasks.length;
</script>

<Launcher bind:visible={showLauncher} />
<NotificationPanel bind:visible={showNotif} />
<TransferPanel bind:visible={showTransfers} />

<!-- Context menu click outside -->
{#if ctxMenu}
  <div class="ctx-overlay" on:click={closeCtxMenu} role="presentation"></div>
  <div class="ctx-menu" style="left:{ctxMenu.x}px; top:{ctxMenu.y}px">
    <div class="ctx-item" on:click={() => togglePin(ctxMenu.appId)} role="button" tabindex="0">
      <span class="ctx-ic">◆</span>
      <span>{isPinned(ctxMenu.appId) ? 'Desanclar del taskbar' : 'Anclar al taskbar'}</span>
    </div>
    {#if ctxMenu.win}
      <div class="ctx-sep"></div>
      <div class="ctx-item" on:click={() => { closeWindow(ctxMenu.win.id); closeCtxMenu(); }} role="button" tabindex="0">
        <span class="ctx-ic">×</span>
        <span>Cerrar ventana</span>
      </div>
    {/if}
  </div>
{/if}

<div class="taskbar">

  <!-- ═══ IZQUIERDA ═══ -->
  <div class="tb-left">

    <!-- Launcher button -->
    <button
      class="tb-launcher-btn"
      on:click={() => showLauncher = !showLauncher}
      class:active={showLauncher}
      title="Apps"
    >
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
        <path fill="currentColor" fill-rule="evenodd" d="M0 0h4v4H0V0zm0 6h4v4H0V6zm0 6h4v4H0v-4zM6 0h4v4H6V0zm0 6h4v4H6V6zm0 6h4v4H6v-4zm6-12h4v4h-4V0zm0 6h4v4h-4V6zm0 6h4v4h-4v-4z"/>
      </svg>
    </button>

    <div class="tb-sep"></div>

    <!-- Pinned apps -->
    <div class="app-row">
      {#each $pinnedApps as appId}
        {@const meta = APP_META[appId]}
        {#if meta}
          {@const existing = $windowList.find(w => w.appId === appId)}
          {@const isOpen = !!existing}
          {@const isMin  = existing?.minimized}
          {@const isFocused = isOpen && !isMin && existing?.zIndex === Math.max(...$windowList.map(w => w.zIndex))}
          <button
            class="tb-app"
            class:open={isOpen}
            class:minimized={isMin}
            class:focused={isFocused}
            title={meta.name}
            on:click={() => handleAppClick(appId)}
            on:contextmenu={(e) => openCtxMenu(e, appId, existing)}
          >
            {#if isIconUrl(meta.icon)}
              <img
                src={meta.icon}
                alt={meta.name}
                class="tb-icon-img"
                on:error={(e) => e.target.style.opacity = '0'}
              />
            {:else}
              <span class="tb-emoji">{meta.fallback || meta.icon || '📦'}</span>
            {/if}
          </button>
        {/if}
      {/each}
    </div>

    <!-- Open but not pinned -->
    {#if openUnpinned.length > 0}
      <div class="tb-sep"></div>
      <div class="app-row">
        {#each openUnpinned as win}
          {@const meta = APP_META[win.appId]}
          {@const isFocused = !win.minimized && win.zIndex === Math.max(...$windowList.map(w => w.zIndex))}
          <button
            class="tb-app open"
            class:minimized={win.minimized}
            class:focused={isFocused}
            title={meta?.name || win.appId}
            on:click={() => toggleMinimize(win)}
            on:contextmenu={(e) => openCtxMenu(e, win.appId, win)}
          >
            {#if isIconUrl(meta?.icon)}
              <img
                src={meta.icon}
                alt={meta?.name}
                class="tb-icon-img"
                on:error={(e) => e.target.style.opacity = '0'}
              />
            {:else}
              <span class="tb-emoji">{meta?.fallback || '📦'}</span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}

  </div>

  <!-- ═══ CENTRO (vacío, respira) ═══ -->
  <div class="tb-center"></div>

  <!-- ═══ DERECHA (systray) ═══ -->
  <div class="tb-right">

    <!-- Transferencias -->
    <button
      class="tb-tray"
      class:active={showTransfers}
      on:click={() => { showTransfers = !showTransfers; showNotif = false; }}
      title="Transferencias"
    >
      <span class="tray-ic">⇅</span>
      {#if transferCount > 0}
        <span class="tray-badge info">{transferCount}</span>
      {/if}
    </button>

    <!-- Notificaciones -->
    <button
      class="tb-tray"
      class:active={showNotif}
      on:click={() => { showNotif = !showNotif; showTransfers = false; }}
      title="Notificaciones"
    >
      <span class="tray-ic">◉</span>
      {#if $unreadCount > 0}
        <span class="tray-badge">{$unreadCount}</span>
      {/if}
    </button>

    <!-- Reloj -->
    <div class="tb-clock" title={now.toLocaleString('es-ES')}>
      <span class="clock-time">{$prefs.clock24 ? `${hh}:${mm}:${ss}` : now.toLocaleTimeString('es-ES', { hour12: true })}</span>
      <span class="clock-date">{dd} {MON} · {DOW}</span>
    </div>

    <!-- Power -->
    <button class="tb-power" on:click={logout} title="Cerrar sesión">⏻</button>

  </div>

</div>

<style>
  .taskbar {
    position: fixed;
    left: 0; right: 0; bottom: 0;
    height: var(--taskbar-height);
    background: rgba(10, 10, 10, 0.88);
    backdrop-filter: blur(20px) saturate(130%);
    -webkit-backdrop-filter: blur(20px) saturate(130%);
    border-top: 1px solid var(--border-bright);
    display: flex;
    align-items: stretch;
    padding: 0 12px;
    gap: 8px;
    z-index: 9000;
    font-family: var(--font-mono);
  }
  /* Línea de acento arriba */
  .taskbar::before {
    content: '';
    position: absolute;
    top: -1px;
    left: 0; right: 0;
    height: 1px;
    background: linear-gradient(
      to right,
      transparent 0%,
      var(--accent-glow) 15%,
      var(--accent-glow) 85%,
      transparent 100%
    );
    opacity: 0.6;
  }

  .tb-left, .tb-right {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 0;
  }
  .tb-center { flex: 1; }

  .tb-sep {
    width: 1px;
    align-self: center;
    height: 24px;
    background: var(--border);
    margin: 0 4px;
  }

  .app-row {
    display: flex;
    gap: 4px;
  }

  /* ─── Launcher button ─── */
  .tb-launcher-btn {
    width: 40px;
    height: 40px;
    background: var(--bg);
    color: var(--accent);
    border: 1px solid var(--accent);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.12s;
    /* Mismo patrón de bevel que ventanas y popovers: solo inferior-derecha */
    clip-path: polygon(
      0 0,
      100% 0,
      100% calc(100% - 10px),
      calc(100% - 10px) 100%,
      0 100%
    );
  }
  .tb-launcher-btn:hover {
    background: var(--accent-dim);
    box-shadow: 0 0 8px var(--accent-glow);
  }
  .tb-launcher-btn.active {
    background: var(--accent-dim);
    box-shadow: 0 0 10px var(--accent-glow);
  }

  /* ─── App icon ─── */
  .tb-app {
    position: relative;
    width: 40px;
    height: 40px;
    background: transparent;
    border: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: transform 0.1s;
    padding: 2px;
  }
  .tb-app:hover { transform: translateY(-2px); }

  .tb-icon-img {
    width: 32px;
    height: 32px;
    object-fit: contain;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.5));
  }
  .tb-emoji {
    font-size: 24px;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.5));
  }

  /* Línea verde bajo apps abiertas */
  .tb-app.open::after {
    content: '';
    position: absolute;
    bottom: 0;
    left: 50%;
    transform: translateX(-50%);
    width: 18px;
    height: 2px;
    background: var(--accent);
    box-shadow: 0 0 4px var(--accent-glow);
  }
  .tb-app.focused::after {
    width: 28px;
  }
  .tb-app.minimized::after {
    opacity: 0.5;
    width: 10px;
  }

  /* ─── Systray (tb-tray) ─── */
  .tb-tray {
    position: relative;
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(20, 20, 20, 0.5);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    font-size: 15px;
    cursor: pointer;
    transition: all 0.1s;
    clip-path: polygon(
      0 0, calc(100% - 5px) 0, 100% 5px,
      100% 100%, 5px 100%, 0 calc(100% - 5px)
    );
  }
  .tb-tray:hover,
  .tb-tray.active {
    border-color: var(--accent);
    color: var(--accent);
  }
  .tray-ic {
    line-height: 1;
  }

  .tray-badge {
    position: absolute;
    top: 2px;
    right: 2px;
    min-width: 14px;
    height: 14px;
    padding: 0 4px;
    background: var(--crit);
    color: var(--fg);
    font-family: var(--font-mono);
    font-size: 8px;
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 0 4px rgba(255, 90, 90, 0.5);
    clip-path: polygon(
      0 0, calc(100% - 3px) 0, 100% 3px,
      100% 100%, 3px 100%, 0 calc(100% - 3px)
    );
  }
  .tray-badge.info {
    background: var(--info);
    box-shadow: 0 0 4px rgba(77, 184, 255, 0.5);
  }

  /* ─── Clock ─── */
  .tb-clock {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    padding: 0 10px;
    line-height: 1.1;
    cursor: pointer;
  }
  .clock-time {
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 1px;
    font-feature-settings: "tnum";
  }
  .clock-date {
    font-size: 8.5px;
    color: var(--fg-mute);
    letter-spacing: 1px;
    margin-top: 1px;
  }

  /* ─── Power ─── */
  .tb-power {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(20, 20, 20, 0.5);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.1s;
    clip-path: polygon(
      0 0, calc(100% - 5px) 0, 100% 5px,
      100% 100%, 5px 100%, 0 calc(100% - 5px)
    );
  }
  .tb-power:hover {
    border-color: var(--crit);
    color: var(--crit);
  }

  /* ─── Context menu ─── */
  .ctx-overlay {
    position: fixed;
    inset: 0;
    z-index: 9500;
  }
  .ctx-menu {
    position: fixed;
    min-width: 210px;
    background: var(--bg-1);
    border: 1px solid var(--accent);
    box-shadow: 4px 4px 0 rgba(0, 0, 0, 0.8);
    z-index: 9510;
    font-family: var(--font-mono);
    font-size: 11px;
    clip-path: polygon(
      0 0, calc(100% - 6px) 0, 100% 6px,
      100% 100%, 6px 100%, 0 calc(100% - 6px)
    );
  }
  .ctx-item {
    padding: 8px 14px;
    color: var(--fg);
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    transition: all 0.08s;
  }
  .ctx-item:hover {
    background: var(--bg-2);
    color: var(--accent);
  }
  .ctx-ic {
    color: var(--fg-mute);
    width: 14px;
    text-align: center;
  }
  .ctx-item:hover .ctx-ic { color: var(--accent); }
  .ctx-sep {
    height: 1px;
    background: var(--border);
    margin: 2px 0;
  }
</style>
