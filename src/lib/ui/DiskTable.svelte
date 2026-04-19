<script>
  import Badge from './Badge.svelte';

  /**
   * Disk table with SMART badges.
   * disks: [{ name, model, device, capacity, temp, hours, role?, status }]
   * status: "ok" | "warn" | "crit"
   * on:select — dispatches when a disk row is clicked
   */
  export let disks = [];
  export let showRole = false;
  export let columns = ['model', 'device', 'capacity', 'temp', 'hours'];

  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  const statusLabel = { ok: 'Sano', warn: 'Atención', crit: 'Crítico' };

  const colLabel = {
    model: 'Modelo', device: 'Dispositivo', capacity: 'Capacidad',
    temp: 'Temp', hours: 'Horas', role: 'Rol', status: 'Estado'
  };
</script>

<div class="disk-table">
  <div class="disk-head" class:with-role={showRole}>
    <div></div>
    {#each columns as col}
      <div>{colLabel[col] || col}</div>
    {/each}
    {#if showRole}<div>Rol</div>{/if}
    <div>Estado</div>
  </div>

  {#each disks as disk}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="disk-row" class:with-role={showRole} on:click={() => dispatch('select', disk)}>
      <div class="disk-icon">
        <svg viewBox="0 0 24 24" fill="currentColor"><path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/><path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/></svg>
      </div>
      {#each columns as col}
        <div class={col === 'model' ? 'disk-name' : 'disk-cell'}>
          {disk[col] || '—'}
        </div>
      {/each}
      {#if showRole}
        <div><span class="role-pill">{disk.role || 'data'}</span></div>
      {/if}
      <div>
        <Badge status={disk.status || 'ok'}>{statusLabel[disk.status] || 'Sano'}</Badge>
      </div>
    </div>
  {/each}
</div>

<style>
  .disk-table { width: 100%; }

  .disk-head {
    display: grid;
    grid-template-columns: 28px 1.4fr 1.2fr 0.7fr 0.6fr 0.7fr 110px;
    gap: 14px;
    padding: 0 12px 8px;
    font-size: 10px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.8px;
  }
  .disk-head.with-role {
    grid-template-columns: 28px 1.4fr 1.0fr 0.6fr 0.5fr 0.6fr 0.5fr 110px;
  }

  .disk-row {
    display: grid;
    grid-template-columns: 28px 1.4fr 1.2fr 0.7fr 0.6fr 0.7fr 110px;
    align-items: center;
    gap: 14px;
    padding: 9px 12px;
    border-radius: 8px;
    font-size: 12px;
    cursor: pointer;
    transition: background 0.15s;
  }
  .disk-row.with-role {
    grid-template-columns: 28px 1.4fr 1.0fr 0.6fr 0.5fr 0.6fr 0.5fr 110px;
  }
  .disk-row + .disk-row { margin-top: 2px; }
  .disk-row:hover { background: var(--bg-elev-2); }

  .disk-icon {
    width: 26px; height: 26px;
    border-radius: 6px;
    background: var(--bg-elev-2);
    display: flex; align-items: center; justify-content: center;
    color: var(--text-secondary);
  }
  .disk-icon svg { width: 16px; height: 16px; display: block; }
  .disk-row:hover .disk-icon { color: var(--text-primary); }

  .disk-name {
    font-weight: 500;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .disk-cell {
    font-family: var(--font-mono);
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .role-pill {
    display: inline-block;
    font-size: 10px;
    padding: 3px 8px;
    border-radius: 5px;
    background: var(--bg-elev-2);
    color: var(--text-secondary);
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
</style>
