<script>
  /**
   * Segmented control para alternar vista.
   * Usa el mismo lenguaje que .sb-item.active de AppShell (accent-dim bg + accent color).
   *
   * Props:
   *   value: 'expanded' | 'compact'  — bindable
   *
   * Emite change cuando el usuario cambia manualmente.
   */
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let value = 'expanded';

  function pick(v) {
    if (value === v) return;
    value = v;
    dispatch('change', v);
  }
</script>

<div class="view-toggle" role="group" aria-label="Cambiar vista">
  <button
    class="vt-btn"
    class:active={value === 'expanded'}
    title="Vista detallada"
    on:click={() => pick('expanded')}
  >
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="4" width="18" height="5" rx="1.5"/>
      <rect x="3" y="11" width="18" height="5" rx="1.5"/>
      <rect x="3" y="18" width="18" height="3" rx="1" opacity="0.5"/>
    </svg>
  </button>
  <button
    class="vt-btn"
    class:active={value === 'compact'}
    title="Vista compacta"
    on:click={() => pick('compact')}
  >
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="3" width="8" height="8" rx="1.5"/>
      <rect x="13" y="3" width="8" height="8" rx="1.5"/>
      <rect x="3" y="13" width="8" height="8" rx="1.5"/>
      <rect x="13" y="13" width="8" height="8" rx="1.5"/>
    </svg>
  </button>
</div>

<style>
  .view-toggle {
    display: inline-flex;
    gap: 2px;
    padding: 2px;
    background: var(--bg-elev-2);
    border: 1px solid var(--glass-border);
    border-radius: var(--radius-md);
  }
  .vt-btn {
    width: 30px; height: 26px;
    border: none;
    background: transparent;
    padding: 0;
    color: var(--text-muted);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    transition: all 0.12s;
  }
  .vt-btn:hover:not(.active) {
    color: var(--text-primary);
    background: rgba(255,255,255,0.04);
  }
  .vt-btn.active {
    background: var(--accent-dim);
    color: var(--accent);
  }
  .vt-btn svg {
    width: 14px;
    height: 14px;
  }
</style>
