<script>
  /**
   * Compact disk chip — se usa dentro de PoolCard y en listas de discos.
   *
   * Props:
   *   disk: { name, model, size, smart?:{ temperature, powerOnHours } }
   *   status: 'ok' | 'warn' | 'crit' | 'unknown'  — deriva de SMART
   *
   * El chip resalta sutilmente el borde si el estado es warn/crit
   * (para que al escanear visualmente salte el problemático).
   */
  export let disk;
  export let status = 'ok';

  function fmtH(h) {
    if (!h) return '—';
    return h >= 1000 ? (h / 1000).toFixed(1) + 'k h' : h + ' h';
  }

  $: statusSymbol = status === 'crit' ? '● Crítico' : status === 'warn' ? '● Atención' : status === 'ok' ? '● OK' : '● —';
</script>

<div class="disk-chip {status}">
  <div class="icon">
    <svg viewBox="0 0 24 24" fill="currentColor">
      <path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/>
      <path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/>
    </svg>
  </div>
  <div class="info">
    <div class="name">{disk.model || disk.name}</div>
    <div class="meta">
      /dev/{disk.name}
      {#if disk.size}&nbsp;·&nbsp;{disk.size}{/if}
      {#if disk.smart?.temperature}&nbsp;·&nbsp;{disk.smart.temperature}°C{/if}
      {#if disk.smart?.powerOnHours}&nbsp;·&nbsp;{fmtH(disk.smart.powerOnHours)}{/if}
    </div>
  </div>
  <span class="state {status}">{statusSymbol}</span>
</div>

<style>
  .disk-chip {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 12px;
    background: var(--bg-elev-2);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    font-size: 11px;
    transition: border-color 0.15s;
  }
  .disk-chip.warn { border-color: var(--c-warn-border); background: rgba(245,158,11,0.04); }
  .disk-chip.crit { border-color: var(--c-crit-border); background: rgba(239,68,68,0.05); }

  .icon {
    width: 22px; height: 22px;
    border-radius: 5px;
    background: var(--bg-app);
    color: var(--text-secondary);
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }
  .disk-chip.warn .icon { color: var(--c-warn); }
  .disk-chip.crit .icon { color: var(--c-crit); }
  .icon svg { width: 13px; height: 13px; }

  .info {
    flex: 1;
    min-width: 0;
  }
  .name {
    color: var(--text-primary);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .meta {
    font-family: var(--font-mono);
    color: var(--text-muted);
    font-size: 10px;
    margin-top: 1px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .state {
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 500;
    flex-shrink: 0;
  }
  .state.ok   { color: var(--c-ok); }
  .state.warn { color: var(--c-warn); }
  .state.crit { color: var(--c-crit); }
  .state.unknown { color: var(--text-muted); }
</style>
