<script>
  /** Glass card container. Use card-label inside for section headers. */
  export let padding = '22px 24px';
</script>

<div class="card" style="padding:{padding}">
  <slot />
</div>

<style>
  .card {
    background: var(--glass-bg);
    border: 1px solid var(--glass-border);
    border-radius: var(--radius-lg);
  }
</style>
