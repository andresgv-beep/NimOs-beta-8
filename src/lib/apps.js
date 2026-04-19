/**
 * NimOS Beta 8 · App Manifest
 * ────────────────────────────────
 *
 * Este archivo declara las apps disponibles en el launcher y el taskbar.
 *
 * SCOPE BETA 8 v1 (core — incluidas):
 *   - Files, NimSettings, Storage, Network, NimTorrent
 *   - AppStore, NimBackup, NimHealth, NimShield
 *   - Terminal, Notes
 *
 * POSPUESTAS PARA BETA 9+ (no incluidas):
 *   - MediaPlayer  → en un NAS normalmente Jellyfin vía Docker
 *   - Virtual Machines → mover a AppStore como instalación opcional
 *   - NimLink → reevaluar si sigue siendo relevante
 *
 * WIDGETS DEL SYSTRAY (no son apps con ventana, viven en el taskbar):
 *   - Transferencias (popover) → se abre como panel lateral, no ventana
 *   - Notificaciones (panel)   → igual
 *
 * ICONOS:
 *   - Ruta en /static/icons/*.png (mantenemos los 3D de Beta 7)
 *   - Si el icono falla, se usa el emoji de fallback en `fallback`
 *
 * CATEGORÍAS (para el launcher agrupado):
 *   - 'system'     → apps core de NimOS
 *   - 'utilities'  → herramientas (Terminal, Notes)
 *   - 'docker'     → apps externas instaladas desde AppStore (se cargan dinámicamente)
 */

export const APP_META = {
  files: {
    name:     'Files',
    icon:     '/icons/files.png',
    fallback: '📁',
    width:    1100,
    height:   720,
    category: 'system',
  },

  nimsettings: {
    name:     'NimSettings',
    icon:     '/icons/settings.png',
    fallback: '⚙️',
    width:    960,
    height:   640,
    category: 'system',
  },

  storage: {
    name:     'Storage',
    icon:     '/icons/storage.png',
    fallback: '🗄️',
    width:    1200,
    height:   720,
    category: 'system',
  },

  network: {
    name:     'Network',
    icon:     '/icons/network.png',
    fallback: '🌐',
    width:    960,
    height:   640,
    category: 'system',
  },

  nimtorrent: {
    name:     'NimTorrent',
    icon:     '/icons/nimtorrent.png',
    fallback: '⬇️',
    width:    960,
    height:   600,
    category: 'system',
  },

  appstore: {
    name:     'App Store',
    icon:     '/icons/appstore.png',
    fallback: '🛍️',
    width:    1040,
    height:   680,
    category: 'system',
  },

  nimbackup: {
    name:     'NimBackup',
    icon:     '/icons/nimbackup.png',
    fallback: '📦',
    width:    1040,
    height:   680,
    category: 'system',
  },

  nimhealth: {
    name:     'NimHealth',
    icon:     '/icons/nimhealth.png',
    fallback: '💓',
    width:    1280,
    height:   760,
    category: 'system',
  },

  nimshield: {
    // NOTA: falta el icono 3D de NimShield en /static/icons/.
    // Usa fallback (emoji) hasta que diseñes el PNG como las demás.
    name:     'NimShield',
    icon:     '/icons/nimshield.png',
    fallback: '🛡️',
    width:    1280,
    height:   760,
    category: 'system',
  },

  terminal: {
    name:     'Terminal',
    icon:     '/icons/terminal.png',
    fallback: '💻',
    width:    820,
    height:   520,
    category: 'utilities',
  },

  notes: {
    name:     'Notes',
    icon:     '/icons/notes.png',
    fallback: '📝',
    width:    900,
    height:   620,
    category: 'utilities',
  },
};

/**
 * Helpers
 */
export function getAppMeta(appId) {
  return APP_META[appId] || null;
}

export function listAppsByCategory(category) {
  return Object.entries(APP_META)
    .filter(([, meta]) => meta.category === category)
    .map(([id, meta]) => ({ id, ...meta }));
}

export function listAllApps() {
  return Object.entries(APP_META).map(([id, meta]) => ({ id, ...meta }));
}
