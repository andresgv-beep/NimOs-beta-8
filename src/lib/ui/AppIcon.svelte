<script>
  /**
   * AppIcon · Icono de app con marco HUD
   * ──────────────────────────────────────────
   * Envuelve un icono (PNG/SVG) en 4 brackets tipo HUD en las esquinas.
   * Los brackets cambian a color accent en hover o cuando está activo.
   *
   * Uso:
   *   <AppIcon src="/icons/storage.png" alt="Storage" />
   *   <AppIcon src="/icons/network.png" alt="Network" size="sm" />
   *   <AppIcon src="/icons/nimhealth.png" alt="NimHealth" size="lg" active />
   *
   * Tamaños:
   *   sm ·  44px · Taskbar
   *   md ·  68px · default
   *   lg ·  80px · Launcher
   *   xs ·  32px · Tablas / filas densas
   */
  export let src = '';
  export let alt = '';
  /** sm | md | lg | xs */
  export let size = 'md';
  /** Estado activo (app abierta/seleccionada) · brackets accent permanentes */
  export let active = false;
  /** Fallback: letra o glyph para mostrar si src falla o no se provee */
  export let fallback = '';

  let imgFailed = false;
  function handleError() { imgFailed = true; }
</script>

<div class="app-icon-frame size-{size}" class:active>
  {#if src && !imgFailed}
    <img class="app-icon-img" {src} {alt} on:error={handleError} draggable="false" />
  {:else}
    <div class="app-icon-fallback" aria-label={alt}>
      {fallback || (alt ? alt.charAt(0).toUpperCase() : '◆')}
    </div>
  {/if}

  <!-- Brackets HUD en las 4 esquinas -->
  <span class="br tl-h"></span>
  <span class="br tl-v"></span>
  <span class="br tr-h"></span>
  <span class="br tr-v"></span>
  <span class="br bl-h"></span>
  <span class="br bl-v"></span>
  <span class="br br-h"></span>
  <span class="br br-v"></span>
</div>

<style>
  .app-icon-frame {
    position: relative;
    display: inline-block;
    transition: transform 0.1s;
    flex-shrink: 0;
  }
  .app-icon-frame:hover { transform: scale(1.05); }

  .app-icon-img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    display: block;
  }

  .app-icon-fallback {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--font-mono);
    font-weight: 700;
    color: var(--accent);
    background: var(--bg-1);
    text-transform: uppercase;
  }

  /* Brackets base */
  .br {
    position: absolute;
    background: var(--border-bright);
    transition: background 0.12s;
    pointer-events: none;
  }
  .app-icon-frame:hover .br,
  .app-icon-frame.active .br {
    background: var(--accent);
  }

  /* ─── TAMAÑO xs · 32px (tablas, sidebar header) ─── */
  .size-xs { width: 32px; height: 32px; padding: 3px; }
  .size-xs .br { }
  .size-xs .tl-h, .size-xs .tr-h, .size-xs .bl-h, .size-xs .br-h { width: 7px; height: 1.5px; }
  .size-xs .tl-v, .size-xs .tr-v, .size-xs .bl-v, .size-xs .br-v { width: 1.5px; height: 7px; }
  .size-xs .app-icon-fallback { font-size: 11px; }

  /* ─── TAMAÑO sm · 40px · Taskbar (icono interno ~32px) ─── */
  .size-sm { width: 40px; height: 40px; padding: 4px; }
  .size-sm .tl-h, .size-sm .tr-h, .size-sm .bl-h, .size-sm .br-h { width: 9px; height: 2px; }
  .size-sm .tl-v, .size-sm .tr-v, .size-sm .bl-v, .size-sm .br-v { width: 2px; height: 9px; }
  .size-sm .app-icon-fallback { font-size: 13px; }

  /* ─── TAMAÑO md · 52px · Launcher (icono interno ~48px) ─── */
  .size-md { width: 52px; height: 52px; padding: 2px; }
  .size-md .tl-h, .size-md .tr-h, .size-md .bl-h, .size-md .br-h { width: 12px; height: 2px; }
  .size-md .tl-v, .size-md .tr-v, .size-md .bl-v, .size-md .br-v { width: 2px; height: 12px; }
  .size-md .app-icon-fallback { font-size: 18px; }
  .size-md .app-icon-fallback { font-size: 22px; }

  /* ─── TAMAÑO lg · 80px (launcher) ─── */
  .size-lg { width: 80px; height: 80px; padding: 10px; }
  .size-lg .tl-h, .size-lg .tr-h, .size-lg .bl-h, .size-lg .br-h { width: 20px; height: 2px; }
  .size-lg .tl-v, .size-lg .tr-v, .size-lg .bl-v, .size-lg .br-v { width: 2px; height: 20px; }
  .size-lg .app-icon-fallback { font-size: 28px; }

  /* Posicionamiento brackets (común a todos los tamaños) */
  .tl-h { top: 0; left: 0; }
  .tl-v { top: 0; left: 0; }
  .tr-h { top: 0; right: 0; }
  .tr-v { top: 0; right: 0; }
  .bl-h { bottom: 0; left: 0; }
  .bl-v { bottom: 0; left: 0; }
  .br-h { bottom: 0; right: 0; }
  .br-v { bottom: 0; right: 0; }
</style>
