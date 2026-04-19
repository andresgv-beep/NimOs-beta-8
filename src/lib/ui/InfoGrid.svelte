<script>
  /** 
   * Key-value info grid.
   * items: [{ key: "Nombre", value: "volume", mono?: true, sub?: "subtext" }]
   */
  export let items = [];
</script>

<div class="info-grid">
  {#each items as item}
    <div class="info-k">{item.key}</div>
    <div class="info-v" class:mono={item.mono}>
      {item.value}
      {#if item.sub}<span class="sub">{item.sub}</span>{/if}
    </div>
  {/each}
</div>

<style>
  .info-grid {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 13px 16px;
    font-size: 13px;
  }
  .info-k { color: var(--text-secondary); }
  .info-v {
    color: var(--text-primary);
    font-weight: 500;
    text-align: right;
  }
  .info-v.mono {
    font-family: var(--font-mono);
    font-weight: 400;
  }
  .sub {
    display: block;
    font-size: 11px;
    color: var(--text-muted);
    font-weight: 400;
    margin-top: 2px;
  }
</style>
