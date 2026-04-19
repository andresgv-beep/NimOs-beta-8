<script>
  /**
   * Status hero card — shows system/pool health status.
   * status: "ok" | "warn" | "crit"
   * title: "El sistema funciona correctamente"
   * subtitle: "1 volumen · 3 discos · sin incidencias"
   * stats: [{ label: "Volúmenes", value: "1" }]
   */
  export let status = 'ok';
  export let title = '';
  export let subtitle = '';
  export let stats = [];

  const icons = {
    ok: 'M5 12l5 5L19 7',
    warn: 'M12 3.5L21.5 20L2.5 20Z M12 10v4.5 M12 17.2v0',
    crit: 'M12 3a9 9 0 110 18 9 9 0 010-18z M12 7.5v5.5 M12 16v0',
  };
</script>

<div class="status-hero">
  <div class="status-icon {status}">
    {#if status === 'ok'}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="5 12 10 17 19 7"/></svg>
    {:else if status === 'warn'}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3.5L21.5 20L2.5 20Z"/><line x1="12" y1="10" x2="12" y2="14.5"/><line x1="12" y1="17.2" x2="12" y2="17.2"/></svg>
    {:else}
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><line x1="12" y1="7.5" x2="12" y2="13"/><line x1="12" y1="16" x2="12" y2="16"/></svg>
    {/if}
  </div>
  <div class="status-text">
    <div class="status-title {status}">{title}</div>
    {#if subtitle}<div class="status-sub">{subtitle}</div>{/if}
  </div>
  {#if stats.length > 0}
    <div class="status-stats">
      {#each stats as s}
        <div class="stat">
          <div class="stat-label">{s.label}</div>
          <div class="stat-value">{s.value}</div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .status-hero {
    display: flex;
    align-items: center;
    gap: 20px;
  }

  .status-icon {
    width: 60px; height: 60px;
    border-radius: 50%;
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
    color: #fff;
  }
  .status-icon svg { width: 38px; height: 38px; }
  .status-icon.ok   { background: var(--c-ok); }
  .status-icon.warn { background: var(--c-warn); }
  .status-icon.crit { background: var(--c-crit); }

  .status-text { flex: 1; min-width: 0; }
  .status-title {
    font-size: 22px; font-weight: 700;
    letter-spacing: -0.4px; line-height: 1.2;
  }
  .status-title.ok   { color: var(--c-ok); }
  .status-title.warn { color: var(--c-warn); }
  .status-title.crit { color: var(--c-crit); }
  .status-sub {
    font-size: 13px;
    color: var(--text-secondary);
    margin-top: 5px;
  }

  .status-stats {
    display: flex; gap: 36px;
    padding-left: 32px;
    border-left: 1px solid var(--glass-border);
  }
  .stat {
    display: flex; flex-direction: column;
    gap: 6px; align-items: center;
  }
  .stat-label {
    font-size: 10px; color: var(--text-muted);
    text-transform: uppercase; letter-spacing: 1.2px;
  }
  .stat-value {
    font-family: var(--font-mono);
    font-size: 28px; font-weight: 600;
    color: var(--text-primary);
    line-height: 1; letter-spacing: -0.5px;
  }
</style>
