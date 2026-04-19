<script>
  /**
   * PoolActivity — tira de actividad/progreso para acciones en curso de un pool.
   *
   * Dos variantes visuales según la card que lo contiene:
   *   - size="full"    → strip completo con icono, texto, barra, %
   *   - size="compact" → versión condensada para cards del grid A2
   *
   * Props:
   *   activity: objeto con una de estas formas
   *     { action: 'scrub',    state: 'running',   progress: 42, speed: '123M/s', timeRemaining: '01:23:45' }
   *     { action: 'snapshot', state: 'running' }                      ← indeterminada
   *     { action: 'snapshot', state: 'completed', ok: true }          ← éxito
   *     { action: 'scrub',    state: 'completed', ok: false, message: 'Pool ocupado' }
   *     { action: 'export',   state: 'running' }
   *
   *   action: 'snapshot' | 'scrub' | 'export' | 'resilver' | 'destroy'
   *   state:  'running' | 'completed'
   *   progress?: 0-100  (si no, barra indeterminada)
   *   ok?: boolean      (solo en completed)
   *   message?: string  (opcional, personaliza texto)
   *   speed?: string    (opcional, muestra debajo en full)
   *   timeRemaining?: string (opcional, muestra debajo en full)
   *
   *   size: 'full' | 'compact'  (default 'full')
   *
   * Si activity es null/undefined no renderiza nada.
   */
  export let activity = null;
  export let size = 'full';

  // Etiquetas por acción + estado
  const LABELS = {
    scrub:     { running: 'Verificando integridad', completed_ok: 'Verificación completada', completed_err: 'Error en verificación' },
    snapshot:  { running: 'Creando snapshot',       completed_ok: 'Snapshot creado',          completed_err: 'Error al crear snapshot' },
    export:    { running: 'Desmontando volumen',    completed_ok: 'Volumen desmontado',       completed_err: 'Error al desmontar' },
    resilver:  { running: 'Reconstruyendo datos',   completed_ok: 'Reconstrucción completada', completed_err: 'Error en reconstrucción' },
    destroy:   { running: 'Destruyendo volumen',    completed_ok: 'Volumen destruido',        completed_err: 'Error al destruir' },
    addDisk:   { running: 'Añadiendo disco',        completed_ok: 'Disco añadido',            completed_err: 'Error al añadir disco' },
  };

  $: label = (() => {
    if (!activity) return '';
    if (activity.message) return activity.message;
    const def = LABELS[activity.action] || {};
    if (activity.state === 'running') return def.running || 'Procesando';
    if (activity.state === 'completed') {
      return activity.ok ? (def.completed_ok || 'Completado') : (def.completed_err || 'Error');
    }
    return '';
  })();

  $: isRunning = activity?.state === 'running';
  $: isCompletedOk = activity?.state === 'completed' && activity?.ok;
  $: isCompletedErr = activity?.state === 'completed' && activity?.ok === false;
  $: hasProgress = typeof activity?.progress === 'number' && isRunning;
</script>

{#if activity}
  <div class="pool-activity {size}" class:running={isRunning} class:ok={isCompletedOk} class:err={isCompletedErr}>

    <!-- Icono / spinner -->
    <div class="pa-icon">
      {#if isRunning}
        {#if hasProgress}
          <!-- Círculo con progreso real -->
          <svg viewBox="0 0 24 24" class="pa-prog-ring">
            <circle class="ring-bg" cx="12" cy="12" r="9" fill="none" stroke-width="2.5"/>
            <circle class="ring-fg" cx="12" cy="12" r="9" fill="none" stroke-width="2.5"
              stroke-dasharray="{2 * Math.PI * 9}"
              stroke-dashoffset="{2 * Math.PI * 9 * (1 - activity.progress / 100)}"
              transform="rotate(-90 12 12)"
              stroke-linecap="round" />
          </svg>
        {:else}
          <!-- Spinner indeterminado -->
          <svg viewBox="0 0 24 24" class="pa-spinner">
            <circle cx="12" cy="12" r="9" fill="none" stroke-width="2.5" stroke-linecap="round"
                    stroke-dasharray="14 42" />
          </svg>
        {/if}
      {:else if isCompletedOk}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="5 13 10 18 19 7"/>
        </svg>
      {:else if isCompletedErr}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round">
          <line x1="6" y1="6" x2="18" y2="18"/>
          <line x1="18" y1="6" x2="6" y2="18"/>
        </svg>
      {/if}
    </div>

    <!-- Texto -->
    <div class="pa-text">
      <div class="pa-label">
        {label}{#if isRunning && hasProgress && size === 'full'}<span class="pa-pct"> · {activity.progress}%</span>{/if}
      </div>
      {#if size === 'full' && isRunning && (activity.speed || activity.timeRemaining)}
        <div class="pa-meta">
          {#if activity.speed}{activity.speed}{/if}
          {#if activity.speed && activity.timeRemaining}&nbsp;·&nbsp;{/if}
          {#if activity.timeRemaining}≈ {activity.timeRemaining} restantes{/if}
        </div>
      {/if}
    </div>

    <!-- Porcentaje compact -->
    {#if size === 'compact' && isRunning && hasProgress}
      <div class="pa-pct-compact">{activity.progress}%</div>
    {/if}

    <!-- Barra de progreso (solo en running) -->
    {#if isRunning}
      <div class="pa-bar">
        {#if hasProgress}
          <div class="pa-bar-fill" style="width:{activity.progress}%"></div>
        {:else}
          <div class="pa-bar-indet"></div>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .pool-activity {
    display: grid;
    align-items: center;
    gap: 10px 12px;
    padding: 10px 14px;
    border-radius: var(--radius-md);
    background: var(--bg-elev-2);
    border: 1px solid var(--glass-border);
    font-size: 12px;
    transition: all 0.18s;
  }

  /* Layout full: [icon] [text+meta] . [bar full width row 2] */
  .pool-activity.full {
    grid-template-columns: 28px 1fr;
    grid-template-rows: auto auto;
  }
  .pool-activity.full .pa-icon  { grid-row: 1 / 3; }
  .pool-activity.full .pa-text  { grid-column: 2; }
  .pool-activity.full .pa-bar   { grid-column: 2; }

  /* Layout compact: [icon] [text] [pct] / [bar full row 2] */
  .pool-activity.compact {
    grid-template-columns: 22px 1fr auto;
    gap: 8px 10px;
    padding: 8px 12px;
  }
  .pool-activity.compact .pa-icon       { grid-row: 1 / 3; }
  .pool-activity.compact .pa-text       { grid-column: 2; }
  .pool-activity.compact .pa-pct-compact{ grid-column: 3; grid-row: 1; }
  .pool-activity.compact .pa-bar        { grid-column: 1 / -1; grid-row: 2; }

  /* Color por estado */
  .pool-activity.running { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 30%, transparent); }
  .pool-activity.ok      { color: var(--c-ok);   border-color: var(--c-ok-border);   background: var(--c-ok-dim); }
  .pool-activity.err     { color: var(--c-crit); border-color: var(--c-crit-border); background: var(--c-crit-dim); }

  /* Icono */
  .pa-icon {
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }
  .pa-icon svg { width: 20px; height: 20px; display: block; }
  .pool-activity.compact .pa-icon svg { width: 16px; height: 16px; }

  /* Spinner indeterminado */
  .pa-spinner {
    animation: pa-rot 1.2s linear infinite;
  }
  .pa-spinner circle {
    stroke: currentColor;
  }
  @keyframes pa-rot {
    to { transform: rotate(360deg); }
  }

  /* Anillo de progreso */
  .pa-prog-ring .ring-bg { stroke: rgba(255,255,255,0.08); }
  .pa-prog-ring .ring-fg {
    stroke: currentColor;
    transition: stroke-dashoffset 0.4s ease;
  }

  /* Texto */
  .pa-text { min-width: 0; }
  .pa-label {
    color: var(--text-primary);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pa-pct {
    color: currentColor;
    font-family: var(--font-mono);
    font-weight: 500;
  }
  .pa-meta {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: 10.5px;
    margin-top: 2px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .pa-pct-compact {
    color: currentColor;
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
  }

  /* Barra */
  .pa-bar {
    height: 3px;
    background: rgba(255,255,255,0.06);
    border-radius: 2px;
    overflow: hidden;
    position: relative;
  }
  .pa-bar-fill {
    height: 100%;
    background: currentColor;
    opacity: 0.85;
    border-radius: 2px;
    transition: width 0.4s ease;
  }
  .pa-bar-indet {
    position: absolute;
    top: 0; left: -40%;
    width: 40%;
    height: 100%;
    background: linear-gradient(90deg, transparent, currentColor 50%, transparent);
    opacity: 0.55;
    animation: pa-indet 1.4s ease-in-out infinite;
  }
  @keyframes pa-indet {
    0%   { left: -40%; }
    100% { left: 100%; }
  }

  /* Los estados completados no tienen barra, compactamos el layout */
  .pool-activity.ok.full,
  .pool-activity.err.full {
    grid-template-rows: auto;
  }
  .pool-activity.ok.full .pa-icon,
  .pool-activity.err.full .pa-icon {
    grid-row: 1;
  }
  .pool-activity.ok.compact,
  .pool-activity.err.compact {
    grid-template-rows: auto;
  }
  .pool-activity.ok.compact .pa-icon,
  .pool-activity.err.compact .pa-icon {
    grid-row: 1;
  }
</style>
