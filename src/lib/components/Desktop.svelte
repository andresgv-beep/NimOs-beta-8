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
</style>
