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
  import AppIcon from '$lib/ui/AppIcon.svelte';

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

    <!-- Launcher button · logo NimOS -->
    <button
      class="tb-launcher-btn"
      on:click={() => showLauncher = !showLauncher}
      class:active={showLauncher}
      title="Apps"
    >
      <svg class="nimos-logo" width="24" height="24" viewBox="-15 0 200 185" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="5" y="45" width="80" height="80" rx="16" transform="rotate(-30 45 85)" fill="#e8e8e8"/>
        <rect x="108" y="12" width="60" height="60" rx="10" fill="#e8e8e8"/>
        <rect x="108" y="98" width="60" height="60" rx="10" fill="#e8e8e8"/>
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
              <AppIcon
                src={meta.icon}
                alt={meta.name}
                size="sm"
                fallback={meta.fallback}
                active={isOpen}
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
              <AppIcon
                src={meta.icon}
                alt={meta?.name}
                size="sm"
                fallback={meta?.fallback}
                active={!win.minimized}
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
  /* ═══════════════════════════════════════════════════════════
     TASKBAR · pegado abajo (Windows 11 style) con glass
     ═══════════════════════════════════════════════════════════ */
  .taskbar {
    position: fixed;
    left: 0; right: 0; bottom: 0;
    height: var(--taskbar-height);
    background: var(--window-bg);
    backdrop-filter: var(--glass-blur);
    -webkit-backdrop-filter: var(--glass-blur);
    border-top: 1px solid var(--window-border);
    display: flex;
    align-items: stretch;
    padding: 0 12px;
    gap: 8px;
    z-index: 9000;
    font-family: var(--font-sans);
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
    height: 22px;
    background: var(--border);
    margin: 0 6px;
  }

  .app-row {
    display: flex;
    gap: 4px;
  }

  /* ─── Launcher button · logo NimOS sin marcos ─── */
  .tb-launcher-btn {
    width: 40px;
    height: 40px;
    background: transparent;
    border: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.12s, transform 0.1s;
    border-radius: var(--radius-md);
    padding: 0;
  }
  .tb-launcher-btn:hover {
    background: rgba(255, 255, 255, 0.08);
  }
  .tb-launcher-btn.active {
    background: rgba(255, 255, 255, 0.12);
  }
  .tb-launcher-btn:active {
    transform: scale(0.94);
  }
  .nimos-logo {
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.3));
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
    transition: transform 0.1s, background 0.12s;
    padding: 0;
    border-radius: var(--radius-md);
  }
  .tb-app:hover {
    background: rgba(255, 255, 255, 0.06);
    transform: translateY(-1px);
  }

  .tb-icon-img {
    width: 32px;
    height: 32px;
    object-fit: contain;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.4));
  }
  .tb-emoji {
    font-size: 24px;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.4));
  }

  /* Dot bajo apps abiertas · más sutil que la línea retro */
  .tb-app.open::after {
    content: '';
    position: absolute;
    bottom: -2px;
    left: 50%;
    transform: translateX(-50%);
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent-glow);
  }
  .tb-app.focused::after {
    width: 18px;
    border-radius: 2px;
  }
  .tb-app.minimized::after {
    opacity: 0.4;
    width: 3px;
    height: 3px;
  }

  /* ─── Systray (tb-tray) · glass sutil ─── */
  .tb-tray {
    position: relative;
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--fg-dim);
    font-size: 15px;
    cursor: pointer;
    transition: background 0.12s, color 0.12s;
    border-radius: var(--radius-md);
  }
  .tb-tray:hover {
    background: rgba(255, 255, 255, 0.06);
    color: var(--fg);
  }
  .tb-tray.active {
    background: rgba(255, 255, 255, 0.1);
    color: var(--accent);
  }
  .tray-ic {
    line-height: 1;
  }

  .tray-badge {
    position: absolute;
    top: 2px;
    right: 2px;
    min-width: 15px;
    height: 15px;
    padding: 0 4px;
    background: var(--crit);
    color: #fff;
    font-family: var(--font-sans);
    font-size: 9px;
    font-weight: 600;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    box-shadow: 0 0 4px rgba(248, 113, 113, 0.4);
  }
  .tray-badge.info {
    background: var(--info);
    box-shadow: 0 0 4px rgba(77, 184, 255, 0.4);
  }

  /* ─── Clock ─── */
  .tb-clock {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    padding: 0 10px 0 14px;
    line-height: 1.15;
    cursor: pointer;
  }
  .clock-time {
    font-family: var(--font-sans);
    font-size: 13px;
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 0.2px;
    font-feature-settings: "tnum";
  }
  .clock-date {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    margin-top: 1px;
    font-weight: 400;
  }

  /* ─── Power ─── */
  .tb-power {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--fg-dim);
    font-family: var(--font-sans);
    font-size: 16px;
    cursor: pointer;
    transition: background 0.12s, color 0.12s;
    border-radius: var(--radius-md);
  }
  .tb-power:hover {
    background: rgba(248, 113, 113, 0.1);
    color: var(--crit);
  }

  /* ═══════════════════════════════════════════════════════════
     CONTEXT MENU · glass popover
     ═══════════════════════════════════════════════════════════ */
  .ctx-overlay {
    position: fixed;
    inset: 0;
    z-index: 9500;
  }
  .ctx-menu {
    position: fixed;
    min-width: 210px;
    background: var(--glass-bg-strong);
    backdrop-filter: var(--glass-blur);
    -webkit-backdrop-filter: var(--glass-blur);
    border: 1px solid var(--window-border);
    border-radius: var(--radius-md);
    box-shadow: var(--window-shadow);
    z-index: 9510;
    font-family: var(--font-sans);
    font-size: 13px;
    padding: 4px;
    overflow: hidden;
  }
  .ctx-item {
    padding: 8px 12px;
    color: var(--fg);
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    transition: background 0.08s, color 0.08s;
    border-radius: 5px;
  }
  .ctx-item:hover {
    background: rgba(255, 255, 255, 0.08);
    color: var(--accent);
  }
  .ctx-ic {
    color: var(--fg-mute);
    width: 14px;
    text-align: center;
    font-size: 12px;
  }
  .ctx-item:hover .ctx-ic { color: var(--accent); }
  .ctx-sep {
    height: 1px;
    background: var(--border);
    margin: 4px 2px;
  }
</style>
