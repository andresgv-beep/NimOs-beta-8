<script>
  /**
   * Donut chart — reutilizable para uso de pool, RAM, carpetas, etc.
   *
   * Props:
   *   segments: [{ color: string, value: number }]
   *   center:   string | number  — valor principal mostrado en el centro
   *   label:    string           — etiqueta pequeña debajo del valor
   *   size:     number (px)      — diámetro total (default 130)
   *   thickness: number (px)     — grosor del anillo (default 13)
   *
   * Los segments se dibujan en el orden dado. Si la suma de values es 0
   * solo se muestra el anillo de fondo vacío.
   */
  export let segments = [];
  export let center = '';
  export let label = '';
  export let size = 130;
  export let thickness = 13;

  $: radius = (size - thickness) / 2;
  $: cx = size / 2;
  $: cy = size / 2;
  $: circumference = 2 * Math.PI * radius;

  $: total = segments.reduce((acc, s) => acc + (s.value || 0), 0);

  $: computedSegs = (() => {
    if (total <= 0) return [];
    let offset = 0;
    return segments
      .filter(s => (s.value || 0) > 0)
      .map(s => {
        const len = (s.value / total) * circumference;
        const seg = { color: s.color, dash: `${len} ${circumference}`, off: -offset };
        offset += len;
        return seg;
      });
  })();
</script>

<div class="donut" style="width:{size}px;height:{size}px">
  <svg viewBox="0 0 {size} {size}" style="transform:rotate(-90deg)">
    <circle cx={cx} cy={cy} r={radius} fill="none" stroke="var(--bg-elev-2)" stroke-width={thickness}/>
    {#each computedSegs as seg}
      <circle cx={cx} cy={cy} r={radius} fill="none"
        stroke={seg.color} stroke-width={thickness}
        stroke-dasharray={seg.dash} stroke-dashoffset={seg.off} />
    {/each}
  </svg>
  <div class="center">
    {#if center !== ''}<div class="value">{center}</div>{/if}
    {#if label}<div class="label">{label}</div>{/if}
  </div>
</div>

<style>
  .donut {
    position: relative;
    flex-shrink: 0;
  }
  .donut svg {
    width: 100%;
    height: 100%;
    display: block;
  }
  .center {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    pointer-events: none;
  }
  .value {
    font-family: var(--font-mono);
    font-size: 22px;
    font-weight: 600;
    letter-spacing: -0.5px;
    color: var(--text-primary);
    line-height: 1.1;
  }
  .label {
    font-size: 9px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 1.2px;
    margin-top: 2px;
  }
</style>
