/**
 * NimOS · Mapeo de appId → icono PNG
 * ──────────────────────────────────────
 * Centraliza las rutas de los iconos de cada app nativa.
 * Los iconos viven en `/static/icons/` (servidos como `/icons/*.png`).
 *
 * Uso:
 *   import { getAppIcon } from '$lib/appIcons.js';
 *   <AppIcon src={getAppIcon('storage')} alt="Storage" />
 */

const APP_ICONS = {
  // Apps del sistema
  nimhealth:  '/icons/nimhealth.png',
  network:    '/icons/network.png',
  storage:    '/icons/storage.png',
  files:      '/icons/files.png',
  filemanager:'/icons/files.png',
  settings:   '/icons/settings.png',
  nimtorrent: '/icons/nimtorrent.png',
  nimbackup:  '/icons/nimbackup.png',
  notes:      '/icons/notes.png',
  terminal:   '/icons/terminal.png',
  appstore:   '/icons/appstore.png',

  // Otros
  containers: '/icons/containers.png',
  media:      '/icons/media.png',
  vms:        '/icons/vms.png',
  users:      '/icons/users.png',
  lock:       '/icons/lock.png',
};

/**
 * Devuelve la ruta del icono para un appId, o null si no existe.
 */
export function getAppIcon(appId) {
  if (!appId) return null;
  return APP_ICONS[appId.toLowerCase()] || null;
}

/**
 * Devuelve el objeto completo (si quieres iterar).
 */
export const appIcons = APP_ICONS;
