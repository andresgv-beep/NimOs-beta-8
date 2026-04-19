<script>
  /** 
   * Standard button. 
   * variant: "default" | "primary" | "danger"
   * size: "default" | "sm"
   */
  export let variant = 'default';
  export let size = 'default';
  export let disabled = false;
</script>

<button
  class="btn {variant} {size}"
  {disabled}
  on:click
>
  <slot />
</button>

<style>
  .btn {
    font-family: var(--font-sans);
    font-size: 13px;
    font-weight: 500;
    padding: 11px 16px;
    border-radius: var(--radius-md);
    cursor: pointer;
    border: 1px solid var(--glass-border);
    background: var(--bg-elev-2);
    color: var(--text-primary);
    text-align: left;
    transition: all 0.15s;
    display: inline-flex;
    align-items: center;
    gap: 10px;
  }
  .btn.sm {
    font-size: 12px;
    padding: 9px 16px;
    border-radius: 8px;
    gap: 8px;
  }
  .btn:hover { background: var(--bg-elev-1); }
  .btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .btn.primary {
    background: var(--accent);
    border-color: var(--accent);
    color: #fff;
  }
  .btn.primary:hover { filter: brightness(1.1); }

  .btn.danger {
    background: transparent;
    border: 1px solid var(--c-crit-border);
    color: var(--c-crit);
  }
  .btn.danger:hover { background: var(--c-crit-dim); }
</style>
