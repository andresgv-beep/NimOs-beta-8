<script>
  /**
   * Desktop · Contenedor raíz de la UI de NimOS
   * ─────────────────────────────────────────────
   * - Wallpaper de fondo
   * - Ventanas flotantes (WindowFrame por cada window abierta)
   * - Taskbar inferior
   * - Listener para notificaciones
   */
  import { onMount, onDestroy } from 'svelte';
  import { windowList } from '$lib/stores/windows.js';
  import { prefs } from '$lib/stores/theme.js';
  import {
    loadNotifications, notifications
  } from '$lib/stores/notifications.js';
  import Taskbar from './Taskbar.svelte';
  import WindowFrame from './WindowFrame.svelte';

  let pollInterval;

  onMount(() => {
    loadNotifications();
    checkSmartOnLogin();
    pollInterval = setInterval(pollNotifications, 30000);
    return () => clearInterval(pollInterval);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function pollNotifications() {
    const prevIds = new Set($notifications.map(n => n.id));
    await loadNotifications();
  }

  async function checkSmartOnLogin() {
    try {
      const token = localStorage.getItem('nimbusos_token') || '';
      const r = await fetch('/api/disks/smart/summary', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const d = await r.json();
      if (d.worstStatus === 'critical' || d.worstStatus === 'warning') {
        const badDisks = (d.disks || []).filter(dk => dk.status !== 'ok');
        const names = badDisks.map(dk => dk.name).join(', ');
        const isCritical = d.worstStatus === 'critical';
        notifications.update(n => [{
          id: 'smart-login-' + Date.now(),
          type: isCritical ? 'error' : 'warning',
          category: 'system',
          title: isCritical ? 'Disco en riesgo de fallo' : 'Disco requiere atención',
          message: `SMART detecta problemas en: ${names}. Revisa Storage → Salud.`,
          timestamp: new Date().toISOString(),
          read: false,
        }, ...n]);
      }
    } catch {}
  }
</script>

<div
  class="desktop"
  style={$prefs.wallpaper
    ? `background-image:url('${$prefs.wallpaper}')`
    : ''
  }
>
  <!-- Ventanas -->
  {#each $windowList as win (win.id)}
    {#if !win.minimized}
      <WindowFrame {win} />
    {/if}
  {/each}

  <!-- Taskbar -->
  <Taskbar />
</div>

<style>
  .desktop {
    position: fixed;
    inset: 0;
    background: var(--wallpaper);
    background-size: cover;
    background-position: center;
    overflow: hidden;
  }

  /* Capas ambiente Eagle Ridge · montañas al fondo.
     Solo visibles si el usuario no tiene wallpaper custom.
     Se dibujan con clip-path, sin imágenes externas. */
  .desktop::before {
    content: '';
    position: absolute;
    bottom: 0; left: 0; right: 0;
    height: 60vh;
    background: linear-gradient(180deg, transparent 40%, rgba(80, 70, 80, 0.35) 40.1%, rgba(50, 45, 55, 0.6) 100%);
    clip-path: polygon(0% 100%, 0% 65%, 8% 52%, 15% 58%, 22% 45%, 30% 50%, 38% 38%, 45% 44%, 52% 30%, 60% 36%, 68% 25%, 75% 32%, 82% 22%, 88% 28%, 95% 20%, 100% 26%, 100% 100%);
    pointer-events: none;
    z-index: 0;
  }
  .desktop::after {
    content: '';
    position: absolute;
    bottom: 0; left: 0; right: 0;
    height: 45vh;
    background: linear-gradient(180deg, transparent 0%, rgba(30, 25, 30, 0.7) 30%, rgba(20, 15, 20, 0.9) 100%);
    clip-path: polygon(0% 100%, 0% 50%, 5% 45%, 10% 55%, 18% 40%, 25% 48%, 32% 30%, 40% 38%, 48% 20%, 55% 28%, 62% 18%, 70% 26%, 78% 14%, 85% 22%, 92% 18%, 100% 24%, 100% 100%);
    pointer-events: none;
    z-index: 0;
  }
</style>
