<script>
  import { onMount, tick } from 'svelte';
  import { closeWindow, focusWindow, minimizeWindow, maximizeWindow, updateWindowPos, getWindowPos } from '$lib/stores/windows.js';
  import { APP_META } from '$lib/apps.js';

  export let win;

  $: meta = APP_META[win.appId] || { name: win.appId, icon: '📦' };

  let x = 0, y = 0, w = 800, h = 520;

  onMount(async () => {
    await tick();
    const p = getWindowPos(win.id);
    x = p.x; y = p.y; w = p.width; h = p.height;
  });

  // ── Drag ──
  let dragging = false;
  let dragOffset = { x: 0, y: 0 };

  function getZoom() {
    return parseFloat(document.documentElement.style.zoom) || 1;
  }

  function onTitleMouseDown(e) {
    if (e.target.closest('.wf-btn')) return;
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
    y = Math.max(0, e.clientX / z - dragOffset.y);
    updateWindowPos(win.id, { x, y });
  }

  function onDragEnd() {
    dragging = false;
    window.removeEventListener('mousemove', onDrag);
    window.removeEventListener('mouseup', onDragEnd);
  }

  // ── Resize ──
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

  // ── Maximize ──
  function doMaximize() {
    maximizeWindow(win.id);
    tick().then(() => {
      const p = getWindowPos(win.id);
      x = p.x; y = p.y; w = p.width; h = p.height;
    });
  }

  $: compact = win.appId === 'transfermanager';
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="wf"
  class:maximized={win.maximized}
  class:dragging
  style="z-index:{win.zIndex}; left:{x}px; top:{y}px; width:{w}px; height:{h}px;"
  on:mousedown={() => focusWindow(win.id)}
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="wf-drag" on:mousedown={onTitleMouseDown}></div>

  {#if compact}
    <div class="wf-dots wf-dots-right">
      <button class="wf-btn" on:click={() => closeWindow(win.id)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" width="10" height="10">
          <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </div>
  {:else}
    <div class="wf-dots">
      <button class="wf-btn wf-close" on:click={() => closeWindow(win.id)}><i></i></button>
      <button class="wf-btn wf-min" on:click={() => minimizeWindow(win.id)}><i></i></button>
      <button class="wf-btn wf-max" on:click={doMaximize}><i></i></button>
    </div>
  {/if}

  <div class="wf-body">
    {#if win.isWebApp && win.webAppPort}
      {#await import('$lib/apps/WebApp.svelte') then module}
        <svelte:component this={module.default} appId={win.appId} port={win.webAppPort} name={win.webAppName} />
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
    {:else if win.appId === 'mediaplayer'}
      {#await import('$lib/apps/MediaPlayer.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimbackup'}
      {#await import('$lib/apps/NimBackup.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'texteditor'}
      {#await import('$lib/apps/Notes.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'nimhealth'}
      {#await import('$lib/apps/NimHealth.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else if win.appId === 'transfermanager'}
      {#await import('$lib/apps/TransferManager.svelte') then module}
        <svelte:component this={module.default} />
      {/await}
    {:else}
      <div class="wf-placeholder">
        <span style="font-size:48px">{meta.icon}</span>
        <p>{meta.name}</p>
        <small>Coming soon</small>
      </div>
    {/if}
  </div>

  {#if !win.maximized}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="wf-resize" on:mousedown={onResizeMouseDown}></div>
  {/if}
</div>

<style>
  .wf {
    position: fixed;
    border-radius: 12px;
    overflow: hidden;
    border: 1px solid var(--border-hi, rgba(255,255,255,0.10));
    box-shadow: 0 32px 100px rgba(0,0,0,0.7), inset 0 0.5px 0 rgba(255,255,255,0.04);
    display: flex; flex-direction: column;
    background: var(--bg-app, #09090b);
    animation: wfIn 0.38s cubic-bezier(0.16,1,0.3,1) both;
  }
  .wf.dragging { user-select: none; }
  .wf.maximized {
    border-radius: 0 !important; border: none !important;
    box-shadow: none !important;
    left: 0 !important; top: 0 !important;
    width: 100vw !important;
    height: calc(100vh - var(--taskbar-height, 48px)) !important;
  }
  @keyframes wfIn {
    from { opacity: 0; transform: scale(0.96) translateY(10px); }
    to   { opacity: 1; transform: scale(1) translateY(0); }
  }

  .wf-drag {
    position: absolute; top: 0; left: 0; right: 0;
    height: 38px; z-index: 1;
    cursor: default; user-select: none;
  }

  .wf-dots {
    position: absolute; top: 14px; left: 14px;
    display: flex; gap: 7px; z-index: 10;
    opacity: 0.45; transition: opacity 0.2s;
  }
  .wf-dots-right { left: auto; right: 12px; }
  .wf:hover .wf-dots { opacity: 1; }

  .wf-btn {
    width: 12px; height: 12px;
    border: none; background: none; padding: 0;
    cursor: pointer; display: flex; align-items: center; justify-content: center;
    color: rgba(255,255,255,0.55);
  }
  .wf-btn i {
    width: 12px; height: 12px; border-radius: 50%;
    display: block; transition: box-shadow 0.15s;
  }
  .wf-close i { background: #ff5f57; }
  .wf-min i   { background: #febc2e; }
  .wf-max i   { background: #28c840; }
  .wf-close:hover i { box-shadow: 0 0 6px rgba(255,95,87,0.5); }
  .wf-min:hover i   { box-shadow: 0 0 6px rgba(254,188,46,0.5); }
  .wf-max:hover i   { box-shadow: 0 0 6px rgba(40,200,64,0.5); }

  .wf-body { flex: 1; overflow: hidden; }

  .wf-placeholder {
    width: 100%; height: 100%;
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    gap: 8px; color: var(--text-3, rgba(255,255,255,0.25));
    background: var(--bg-panel, #111114);
  }
  .wf-placeholder p { font-size: 14px; font-weight: 500; }
  .wf-placeholder small { font-size: 11px; }

  .wf-resize {
    position: absolute; bottom: 0; right: 0;
    width: 16px; height: 16px;
    cursor: nwse-resize; z-index: 10;
  }
</style>
