<script>
  /**
   * WindowFrame · Marco de ventana draggable y resizable
   * ──────────────────────────────────────────────────────
   * Envuelve cada app abierta, maneja drag, resize, maximize.
   * El chrome de titlebar lo pone AppShell por dentro — WindowFrame
   * solo es el contenedor flotante con bordes retro.
   *
   * Basado en el de Beta 7 pero adaptado a estética v3:
   *   - border-radius: 0
   *   - shadow hard 4px 4px + glow verde tenue
   *   - border 1px bright
   *   - sin border-radius en maximized
   */
  import { onMount, tick, setContext } from 'svelte';
  import {
    closeWindow, focusWindow, minimizeWindow, maximizeWindow,
    updateWindowPos, getWindowPos
  } from '$lib/stores/windows.js';
  import { APP_META } from '$lib/apps.js';

  export let win;

  $: meta = APP_META[win.appId] || { name: win.appId, fallback: '📦' };

  // Expose window controls vía context a AppShell
  setContext('windowControls', {
    close:    () => closeWindow(win.id),
    minimize: () => minimizeWindow(win.id),
    maximize: () => doMaximize(),
    getWin:   () => win,
  });

  let x = 0, y = 0, w = 800, h = 520;

  onMount(async () => {
    await tick();
    const p = getWindowPos(win.id);
    x = p.x; y = p.y; w = p.width; h = p.height;
  });

  // ─── Drag ───
  let dragging = false;
  let dragOffset = { x: 0, y: 0 };

  function getZoom() {
    return parseFloat(document.documentElement.style.zoom) || 1;
  }

  function onTitleMouseDown(e) {
    if (e.target.closest('.wc-btn')) return;
    if (e.target.closest('.tb-actions button')) return;
    if (win.maximized) return;
    focusWindow(win.id);
    dragging = true;
    const z = getZoom();
    dragOffset = { x: e.clientX / z - x, y: e.clientY / z - y };
    window.addEventListener('mousemove', onDrag);
    window.addEventListener('mouseup', onDragEnd);
  }

  function onDrag(e) {
    if (!dragging) return;
    const z = getZoom();
    x = e.clientX / z - dragOffset.x;
    y = Math.max(0, e.clientY / z - dragOffset.y);
    updateWindowPos(win.id, { x, y });
  }

  function onDragEnd() {
    dragging = false;
    window.removeEventListener('mousemove', onDrag);
    window.removeEventListener('mouseup', onDragEnd);
  }

  // ─── Resize ───
  let resizing = false;
  let resizeStart = { mx: 0, my: 0, w: 0, h: 0 };

  function onResizeMouseDown(e) {
    if (win.maximized) return;
    e.stopPropagation();
    resizing = true;
    const z = getZoom();
    resizeStart = { mx: e.clientX / z, my: e.clientY / z, w, h };
    window.addEventListener('mousemove', onResize);
    window.addEventListener('mouseup', onResizeEnd);
  }

  function onResize(e) {
    if (!resizing) return;
    const z = getZoom();
    w = Math.max(400, resizeStart.w + (e.clientX / z - resizeStart.mx));
    h = Math.max(300, resizeStart.h + (e.clientY / z - resizeStart.my));
    updateWindowPos(win.id, { width: w, height: h });
  }

  function onResizeEnd() {
    resizing = false;
    window.removeEventListener('mousemove', onResize);
    window.removeEventListener('mouseup', onResizeEnd);
  }

  // ─── Maximize ───
  function doMaximize() {
    maximizeWindow(win.id);
    tick().then(() => {
      const p = getWindowPos(win.id);
      x = p.x; y = p.y; w = p.width; h = p.height;
    });
  }
</script>

<div
  class="window"
  class:maximized={win.maximized}
  class:dragging
  style="z-index:{win.zIndex}; left:{x}px; top:{y}px; width:{w}px; height:{h}px;"
  on:mousedown={() => focusWindow(win.id)}
  role="application"
>
  <!-- Drag zone invisible en la titlebar -->
  <div
    class="drag-zone"
    on:mousedown={onTitleMouseDown}
    role="presentation"
  ></div>

  <!-- App content — el .content ocupa toda la ventana, incluyendo titlebar -->
  <div class="content">
    {#if win.isWebApp && win.webAppPort}
      {#await import('$lib/apps/WebApp.svelte') then module}
        <svelte:component
          this={module.default}
          appId={win.appId}
          port={win.webAppPort}
          name={win.webAppName}
        />
      {/await}
    {:else if win.appId === 'files'}
      {#await import('$lib/apps/FileManager.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimsettings'}
      {#await import('$lib/apps/Settings.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'storage'}
      {#await import('$lib/apps/StorageApp.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'network'}
      {#await import('$lib/apps/NetworkApp.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimtorrent'}
      {#await import('$lib/apps/NimTorrent.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'appstore'}
      {#await import('$lib/apps/AppStore.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimbackup'}
      {#await import('$lib/apps/NimBackup.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'notes'}
      {#await import('$lib/apps/Notes.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimhealth'}
      {#await import('$lib/apps/NimHealth.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimshield'}
      {#await import('$lib/apps/NimShield.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'terminal'}
      {#await import('$lib/apps/Terminal.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else}
      <div class="placeholder">
        <span class="ph-ic">{meta.fallback}</span>
        <p>{meta.name}</p>
        <small>Coming soon</small>
      </div>
    {/if}
  </div>

  {#if !win.maximized}
    <div class="resize-handle" on:mousedown={onResizeMouseDown} role="presentation"></div>
  {/if}
</div>

<style>
  /* ─── Window frame con bevel en esquina inferior-derecha ─────
     Técnica: la .window es el "borde" (color del marco), y el .content
     interno tiene el mismo clip-path con 1px menos.
     Así el marco queda visible incluso en la diagonal biselada. */
  .window {
    position: fixed;
    display: flex;
    flex-direction: column;
    background: var(--border-bright);       /* color del marco */
    padding: 1px;                             /* grosor del borde */
    animation: win-in 0.32s cubic-bezier(0.16, 1, 0.3, 1) both;
    box-shadow: 0 0 24px rgba(0, 255, 159, 0.06);
    /* Bevel 14px en la esquina inferior-derecha (opuesta al sidebar) */
    clip-path: polygon(
      0 0,
      100% 0,
      100% calc(100% - 14px),
      calc(100% - 14px) 100%,
      0 100%
    );
  }
  .window.dragging { user-select: none; }
  .window.maximized {
    background: var(--bg) !important;
    padding: 0 !important;
    box-shadow: none !important;
    clip-path: none !important;
    left: 0 !important;
    top: 0 !important;
    width: calc(100vw / var(--ui-zoom, 1)) !important;
    height: calc((100vh - var(--taskbar-height, 52px)) / var(--ui-zoom, 1)) !important;
  }

  .drag-zone {
    position: absolute;
    top: 0;
    left: 0;
    right: 140px; /* deja espacio para los wc-btn a la derecha */
    height: 32px;
    z-index: 5;
    cursor: default;
    pointer-events: auto;
  }

  .content {
    flex: 1;
    overflow: hidden;
    min-height: 0;
    background: var(--bg);
    /* Mismo clip-path que la .window pero 1px menos: así el marco queda visible */
    clip-path: polygon(
      0 0,
      100% 0,
      100% calc(100% - 13px),
      calc(100% - 13px) 100%,
      0 100%
    );
  }
  .window.maximized .content {
    clip-path: none !important;
  }

  .placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--fg-mute);
    background: var(--bg);
    font-family: var(--font-mono);
  }
  .ph-ic { font-size: 48px; }
  .placeholder p {
    font-size: 13px;
    font-weight: 500;
    color: var(--fg-dim);
    letter-spacing: 1px;
  }
  .placeholder small {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 1.5px;
    text-transform: uppercase;
  }

  .resize-handle {
    position: absolute;
    bottom: 12px;  /* movido hacia dentro por el bevel de 14px */
    right: 12px;
    width: 16px;
    height: 16px;
    cursor: nwse-resize;
    z-index: 10;
  }
  .resize-handle::before {
    content: '';
    position: absolute;
    right: 0;
    bottom: 0;
    width: 8px;
    height: 8px;
    background:
      linear-gradient(135deg, transparent 0 3px, var(--fg-faint) 3px 4px, transparent 4px 6px, var(--fg-faint) 6px 7px, transparent 7px);
  }
</style>
