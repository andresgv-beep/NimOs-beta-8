<script>
  /**
   * AppIcon · Icono de app limpio (glass v4)
   * ──────────────────────────────────────────
   * Solo renderiza el icono (PNG/SVG) con fallback si falla.
   * Sin brackets HUD retro — estética glass limpia.
   *
   * Uso:
   *   <AppIcon src="/icons/storage.png" alt="Storage" />
   *   <AppIcon src="/icons/network.png" alt="Network" size="sm" />
   *   <AppIcon src="/icons/nimhealth.png" alt="NimHealth" size="lg" active />
   *
   * Tamaños:
   *   xs ·  32px · Tablas / filas densas
   *   sm ·  40px · Taskbar
   *   md ·  52px · Launcher
   *   lg ·  80px · Launcher grande
   */
  export let src = '';
  export let alt = '';
  /** xs | sm | md | lg */
  export let size = 'md';
  /** Estado activo (app abierta/seleccionada) · se mantiene por compat, efecto visual sutil */
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
</div>

<style>
  .app-icon-frame {
    position: relative;
    display: inline-block;
    transition: transform 0.1s;
    flex-shrink: 0;
  }

  .app-icon-img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    display: block;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.4));
  }

  .app-icon-fallback {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--font-sans);
    font-weight: 600;
    color: var(--accent);
    background: rgba(255, 255, 255, 0.06);
    border-radius: 8px;
    text-transform: uppercase;
  }

  /* ─── TAMAÑOS ─── */
  .size-xs { width: 32px; height: 32px; }
  .size-xs .app-icon-fallback { font-size: 12px; border-radius: 6px; }

  .size-sm { width: 36px; height: 36px; }
  .size-sm .app-icon-fallback { font-size: 14px; }

  .size-md { width: 52px; height: 52px; }
  .size-md .app-icon-fallback { font-size: 22px; border-radius: 10px; }

  .size-lg { width: 80px; height: 80px; }
  .size-lg .app-icon-fallback { font-size: 28px; border-radius: 12px; }
</style>
