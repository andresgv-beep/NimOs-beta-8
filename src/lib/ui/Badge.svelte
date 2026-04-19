<script>
  /** 
   * Status badge with dot indicator.
   * status: "ok" | "warn" | "crit" | "info"
   */
  export let status = 'ok';
</script>

<span class="badge {status}">
  <span class="dot"></span>
  <slot />
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 11px;
    font-weight: 500;
    padding: 5px 11px;
    border-radius: 14px;
    white-space: nowrap;
  }
  .dot {
    width: 6px; height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .ok   { background: var(--c-ok-dim);   color: var(--c-ok);   border: 1px solid var(--c-ok-border); }
  .ok   .dot { background: var(--c-ok); }
  .warn { background: var(--c-warn-dim); color: var(--c-warn); border: 1px solid var(--c-warn-border); }
  .warn .dot { background: var(--c-warn); }
  .crit { background: var(--c-crit-dim); color: var(--c-crit); border: 1px solid var(--c-crit-border); }
  .crit .dot { background: var(--c-crit); }
  .info { background: var(--c-info-dim); color: var(--c-info); border: 1px solid rgba(59,130,246,0.30); }
  .info .dot { background: var(--c-info); }
</style>
