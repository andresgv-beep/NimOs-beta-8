<script>
  /**
   * PoolCard — tarjeta de pool con dos variantes de densidad.
   *
   * Props:
   *   pool:      objeto pool tal y como devuelve /api/storage/status
   *   fileStats: objeto { video, audio, image, document, other } con bytes por tipo
   *              (puede venir como null/undefined si aún no se ha cargado)
   *   variant:   'expanded' | 'compact'  — densidad de la tarjeta
   *
   * Eventos (forward al padre, que maneja acciones contra la API):
   *   on:snapshot   — crear snapshot del pool
   *   on:scrub      — iniciar verificación
   *   on:addDisk    — añadir disco al pool
   *   on:menu       — abrir menú kebab con todas las acciones
   *   on:click      — click sobre el cuerpo de la card (navegar a detalle)
   *
   * El componente NO decide su ancho — el padre lo controla con el contenedor.
   * Regla NimOS: componentes de contenido rico no imponen su tamaño.
   */
  import { createEventDispatcher } from 'svelte';
  import Badge from './Badge.svelte';
  import Button from './Button.svelte';
  import Donut from './Donut.svelte';
  import DiskChip from './DiskChip.svelte';
  import PoolActivity from './PoolActivity.svelte';

  const dispatch = createEventDispatcher();

  export let pool;
  export let fileStats = null;
  export let variant = 'expanded';
  export let activity = null; // { action, state, progress?, speed?, timeRemaining?, ok?, message? }

  // ── Categorías visuales de archivos (coherentes con StorageApp) ──
  const CATEGORIES = [
    { key: 'video',    label: 'Vídeo',      color: '#3b82f6' },
    { key: 'audio',    label: 'Audio',      color: '#f59e0b' },
    { key: 'image',    label: 'Imágenes',   color: '#10b981' },
    { key: 'document', label: 'Documentos', color: '#8b5cf6' },
    { key: 'other',    label: 'Otros',      color: '#64748b' },
  ];

  // ── Helpers de formato ──
  function fmtBytes(b) {
    if (!b || b === 0) return '0 B';
    if (b >= 1099511627776) return (b / 1099511627776).toFixed(1) + ' TB';
    if (b >= 1073741824)    return (b / 1073741824).toFixed(1) + ' GB';
    if (b >= 1048576)       return Math.round(b / 1048576) + ' MB';
    if (b >= 1024)          return Math.round(b / 1024) + ' KB';
    return b + ' B';
  }

  function vdevLabel(t) {
    return { raidz1:'RAIDZ1', raidz2:'RAIDZ2', raidz3:'RAIDZ3',
             mirror:'Espejo', single:'Simple', stripe:'Stripe' }[t] || t || '';
  }

  function poolStatus(p) {
    const ph = p.poolHealth;
    if (ph?.status === 'critical' || p.health === 'FAULTED') return 'crit';
    if (ph?.status === 'degraded' || p.health === 'DEGRADED') return 'warn';
    if (p.disks?.some(d => d.smartStatus === 'critical')) return 'warn';
    return 'ok';
  }

  function diskStatus(d) {
    return d.smartStatus === 'critical' ? 'crit'
         : d.smartStatus === 'warning'  ? 'warn'
         : d.smartStatus === 'ok'       ? 'ok'
         : 'unknown';
  }

  // ── Derivados ──
  $: status = poolStatus(pool);
  $: statusLabel = status === 'crit' ? 'Crítico'
                 : status === 'warn' ? 'Aviso'
                 :                     'Sano';

  $: subtitle = (() => {
    let s = pool.type?.toUpperCase() || '';
    const vl = vdevLabel(pool.vdevType);
    if (vl) s += ' · ' + vl;
    const n = pool.disks?.length || 0;
    s += ' · ' + n + ' disco' + (n !== 1 ? 's' : '');
    return s;
  })();

  $: usagePct = pool.usagePercent || 0;

  // Calcular used y free a partir de size (bytes) y usagePct.
  // pool.totalFormatted viene listo del backend; used/free se derivan.
  $: usedBytes = pool.size ? Math.round(pool.size * usagePct / 100) : 0;
  $: freeBytes = pool.size ? pool.size - usedBytes : 0;
  $: usedFmt = pool.size ? fmtBytes(usedBytes) : '—';
  $: freeFmt = pool.size ? fmtBytes(freeBytes) : '—';
  $: totalFmt = pool.totalFormatted || (pool.size ? fmtBytes(pool.size) : '—');

  // Donut: segmentos por tipo de contenido
  $: donutSegments = fileStats
    ? CATEGORIES
        .filter(c => (fileStats[c.key] || 0) > 0)
        .map(c => ({ color: c.color, value: fileStats[c.key] }))
    : [];

  // Distribución con valores para leyenda
  $: distribution = fileStats
    ? CATEGORIES
        .filter(c => (fileStats[c.key] || 0) > 0)
        .map(c => ({ ...c, value: fileStats[c.key] }))
    : [];

  $: hasWarning = status === 'warn' || status === 'crit';
  $: warningDisks = (pool.disks || []).filter(d => {
    const s = diskStatus(d);
    return s === 'warn' || s === 'crit';
  });
</script>

<div
  class="pool-card {variant}"
  class:warn={status === 'warn'}
  class:crit={status === 'crit'}
>
  <!-- ── Cabecera común ── -->
  <div class="pc-head">
    <div class="pc-icon">
      <svg viewBox="0 0 24 24" fill="currentColor">
        <path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/>
        <path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/>
      </svg>
    </div>
    <div class="pc-ident">
      <div class="pc-name">
        <span>{pool.name}</span>
        <Badge status={status}>{statusLabel}</Badge>
      </div>
      <div class="pc-sub">{subtitle}</div>
    </div>
    <button class="pc-kebab" title="Más acciones" on:click|stopPropagation={(e) => dispatch('menu', { pool, event: e })}>
      <svg viewBox="0 0 24 24" fill="currentColor">
        <circle cx="12" cy="5" r="1.8"/>
        <circle cx="12" cy="12" r="1.8"/>
        <circle cx="12" cy="19" r="1.8"/>
      </svg>
    </button>
  </div>

  <!-- Activity strip (solo si hay acción en curso o recién completada) -->
  {#if activity}
    <div class="pc-activity-row">
      <PoolActivity {activity} size={variant === 'expanded' ? 'full' : 'compact'} />
    </div>
  {/if}

  <!-- ══ Variante EXPANDED ══ -->
  {#if variant === 'expanded'}
    <div class="pc-body-x">
      <Donut
        segments={donutSegments}
        center="{usagePct}%"
        label="Usado"
        size={130}
        thickness={13}
      />

      <div class="pc-dist-wrap">
        {#if distribution.length > 0}
          <div class="pc-dist">
            {#each distribution as item}
              <div class="dist-row">
                <span class="dot" style="background:{item.color}"></span>
                <span class="n">{item.label}</span>
                <span class="v">{fmtBytes(item.value)}</span>
              </div>
            {/each}
          </div>
        {:else}
          <div class="state-empty">Sin datos de distribución</div>
        {/if}
        <div class="pc-cap">
          <span><b>{usedFmt}</b> usado de {totalFmt}</span>
          {#if pool.size}<span class="free">{freeFmt} libre</span>{/if}
        </div>
      </div>

      <div class="pc-actions">
        <Button variant="primary" size="sm" on:click={() => dispatch('snapshot', pool)}>Crear snapshot</Button>
        <Button size="sm" on:click={() => dispatch('scrub', pool)}>Scrub ahora</Button>
        <Button size="sm" on:click={() => dispatch('addDisk', pool)}>Añadir disco</Button>
      </div>
    </div>

    {#if pool.disks?.length > 0}
      <div class="pc-disks">
        {#each pool.disks as disk}
          <DiskChip {disk} status={diskStatus(disk)} />
        {/each}
      </div>
    {/if}

  <!-- ══ Variante COMPACT ══ -->
  {:else}
    <div class="pc-body-c">
      <!-- Barra segmentada por tipo -->
      <div class="cap-seg">
        {#if donutSegments.length > 0}
          {@const total = donutSegments.reduce((a,s) => a + s.value, 0)}
          {#each donutSegments as seg}
            <div class="cap-seg-fill" style="width:{(seg.value / total) * usagePct}%;background:{seg.color}"></div>
          {/each}
        {:else if usagePct > 0}
          <div class="cap-seg-fill" style="width:{usagePct}%;background:var(--accent)"></div>
        {/if}
      </div>

      <div class="cap-labels">
        <span class="used"><b>{usedFmt}</b> usados</span>
        <span class="pct">{usagePct}%</span>
        <span class="total">{totalFmt}</span>
      </div>

      {#if distribution.length > 0}
        <div class="pc-dist-inline">
          {#each distribution.slice(0, 4) as item}
            <span class="dist-item">
              <span class="dot" style="background:{item.color}"></span>
              <span class="n">{item.label}</span>
              <span class="v">{fmtBytes(item.value)}</span>
            </span>
          {/each}
        </div>
      {/if}

      {#if hasWarning && warningDisks.length > 0}
        <div class="pc-warn-strip">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
            <line x1="12" y1="9" x2="12" y2="13"/>
            <line x1="12" y1="17" x2="12.01" y2="17"/>
          </svg>
          <span>{warningDisks.length} disco{warningDisks.length > 1 ? 's' : ''} requiere{warningDisks.length > 1 ? 'n' : ''} atención</span>
        </div>
      {/if}

      <div class="pc-actions-c">
        <Button variant="primary" size="sm" on:click={() => dispatch('snapshot', pool)}>Snapshot</Button>
        <Button size="sm" on:click={() => dispatch('scrub', pool)}>Scrub</Button>
      </div>
    </div>
  {/if}
</div>

<style>
  .pool-card {
    background: var(--glass-bg);
    border: 1px solid var(--glass-border);
    border-radius: var(--radius-lg);
    transition: border-color 0.2s;
    min-width: 0;
    overflow: hidden;
  }
  .pool-card.expanded { padding: 24px 26px; }
  .pool-card.compact  { padding: 20px 22px; }
  .pool-card:hover { border-color: rgba(255,255,255,0.12); }
  .pool-card.warn {
    border-color: var(--c-warn-border);
    background: linear-gradient(180deg, rgba(245,158,11,0.04), transparent 30%), var(--glass-bg);
  }
  .pool-card.crit {
    border-color: var(--c-crit-border);
    background: linear-gradient(180deg, rgba(239,68,68,0.05), transparent 30%), var(--glass-bg);
  }

  /* ── Cabecera ── */
  .pc-head {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .pool-card.expanded .pc-head { margin-bottom: 18px; }
  .pool-card.compact  .pc-head { margin-bottom: 14px; }

  /* Activity row (cuando hay activity activa) */
  .pc-activity-row {
    margin-bottom: 16px;
  }
  .pool-card.compact .pc-activity-row {
    margin-bottom: 12px;
  }
  /* Si hay activity el head necesita menos margen bottom */
  .pool-card.expanded .pc-head:has(+ .pc-activity-row) { margin-bottom: 14px; }
  .pool-card.compact  .pc-head:has(+ .pc-activity-row) { margin-bottom: 10px; }

  .pc-icon {
    border-radius: 9px;
    background: var(--bg-elev-2);
    color: var(--text-primary);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .pool-card.expanded .pc-icon { width: 38px; height: 38px; }
  .pool-card.compact  .pc-icon { width: 32px; height: 32px; border-radius: 8px; }
  .pool-card.expanded .pc-icon svg { width: 20px; height: 20px; }
  .pool-card.compact  .pc-icon svg { width: 17px; height: 17px; }

  .pc-ident { flex: 1; min-width: 0; }
  .pc-name {
    font-weight: 700;
    letter-spacing: -0.3px;
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--text-primary);
  }
  .pool-card.expanded .pc-name { font-size: 18px; }
  .pool-card.compact  .pc-name { font-size: 15px; letter-spacing: -0.2px; }

  .pc-sub {
    font-size: 12px;
    color: var(--text-secondary);
    font-family: var(--font-mono);
    margin-top: 3px;
  }
  .pool-card.compact .pc-sub { font-size: 11px; margin-top: 2px; }

  .pc-kebab {
    width: 30px; height: 30px;
    border: none;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.12s;
    flex-shrink: 0;
  }
  .pc-kebab:hover { background: var(--bg-elev-2); color: var(--text-primary); }
  .pc-kebab svg { width: 14px; height: 14px; }

  /* ── Body EXPANDED ── */
  .pc-body-x {
    display: grid;
    grid-template-columns: 130px minmax(0, 1fr) auto;
    gap: 26px;
    align-items: center;
    min-width: 0;
  }

  .pc-dist-wrap { min-width: 0; }
  .pc-dist {
    display: flex;
    flex-direction: column;
    gap: 9px;
  }
  .dist-row {
    display: grid;
    grid-template-columns: 10px 1fr auto;
    gap: 10px;
    align-items: center;
    font-size: 12px;
  }
  .dist-row .dot {
    width: 9px; height: 9px;
    border-radius: 50%;
  }
  .dist-row .n { color: var(--text-secondary); }
  .dist-row .v {
    font-family: var(--font-mono);
    color: var(--text-primary);
    font-size: 11px;
  }

  .pc-cap {
    margin-top: 14px;
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }
  .pc-cap b { color: var(--text-primary); font-weight: 500; }
  .pc-cap .free { color: var(--text-muted); }

  .state-empty {
    font-size: 12px;
    color: var(--text-muted);
    padding: 8px 0;
  }

  .pc-actions {
    display: flex;
    flex-direction: column;
    gap: 7px;
    min-width: 140px;
    max-width: 180px;
  }
  .pc-actions :global(button) {
    justify-content: flex-start;
  }

  /* ── Discos inline (solo expanded) ── */
  .pc-disks {
    margin-top: 18px;
    padding-top: 16px;
    border-top: 1px solid var(--glass-border);
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr));
    gap: 10px;
  }

  /* ── Body COMPACT ── */
  .pc-body-c { display: flex; flex-direction: column; }

  .cap-seg {
    display: flex;
    height: 6px;
    border-radius: 3px;
    overflow: hidden;
    background: var(--bg-elev-2);
  }
  .cap-seg-fill { height: 100%; }

  .cap-labels {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .cap-labels .used { color: var(--text-secondary); }
  .cap-labels .used b { color: var(--text-primary); font-weight: 500; }
  .cap-labels .pct { color: var(--text-primary); font-weight: 500; }
  .cap-labels .total { color: var(--text-muted); }

  .pc-dist-inline {
    margin-top: 12px;
    display: flex;
    flex-wrap: wrap;
    gap: 10px 14px;
    font-size: 11px;
  }
  .dist-item {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .dist-item .dot {
    width: 7px; height: 7px;
    border-radius: 50%;
  }
  .dist-item .n { color: var(--text-secondary); }
  .dist-item .v {
    font-family: var(--font-mono);
    color: var(--text-muted);
  }

  .pc-warn-strip {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--c-warn-dim);
    border: 1px solid var(--c-warn-border);
    border-radius: var(--radius-md);
    font-size: 11px;
    color: var(--c-warn);
    margin-top: 12px;
  }
  .pc-warn-strip svg {
    width: 13px; height: 13px;
    flex-shrink: 0;
  }

  .pc-actions-c {
    display: flex;
    gap: 6px;
    margin-top: 14px;
    padding-top: 12px;
    border-top: 1px solid var(--glass-border);
  }
  .pc-actions-c :global(button) {
    flex: 1;
    justify-content: center;
  }
</style>
