import { writable, derived } from 'svelte/store';
import { getToken } from './auth.js';

/**
 * NimOS Beta 8 theme store
 * ─────────────────────────
 * Solo tenemos un tema base (retro v3) pero mantenemos la posibilidad
 * de personalizar el accent color y la configuración del taskbar.
 *
 * Comparado con Beta 7: quitamos los 3 temas (midnight/dark/light), los modos
 * dock, las posiciones top/left del taskbar, y widgets del desktop.
 * Si en Beta 9 queremos reintroducirlos, se hace aquí.
 */

const ACCENT_COLORS = {
  green:    '#00ff9f', // default · verde fósforo
  amber:    '#ffb800',
  cyan:     '#4db8ff',
  magenta:  '#e873ff',
  orange:   '#ff8c3f',
  red:      '#ff5a5a',
};

const DEFAULTS = {
  accentColor: 'green',
  customAccentColor: '#00ff9f',

  // Taskbar
  taskbarSize:     'medium',    // 'small' (44px) | 'medium' (52px) | 'large' (60px)
  autoHideTaskbar: false,

  // Reloj
  clock24: true,

  // Escalado UI
  textScale: 100,
  uiScale:   'auto', // 'auto' | number (75..150)

  // Wallpaper (path o URL)
  wallpaper: '',

  // Scanlines sobre desktop (el CRT effect opcional reforzado)
  crtOverlay: false,

  // Apps ancladas al taskbar
  pinnedApps: ['files', 'appstore', 'nimsettings', 'nimhealth'],
};

export const prefs = writable({ ...DEFAULTS });

// Derivados
export const accentColor = derived(prefs, $p =>
  ACCENT_COLORS[$p.accentColor] || $p.customAccentColor || ACCENT_COLORS.green
);
export const pinnedApps = derived(prefs, $p => $p.pinnedApps);

let saveTimeout = null;

/**
 * Calcula el factor de escala automático según resolución/DPR.
 * Mismo algoritmo que Beta 7 — funcionaba bien.
 */
function computeUiScale(setting) {
  if (setting !== 'auto' && typeof setting === 'number') return setting / 100;

  const w = window.innerWidth;
  const dpr = window.devicePixelRatio || 1;
  const physicalWidth = w * dpr;
  const baseline = 1920;

  let scale;
  if (dpr > 1 && physicalWidth > baseline * 1.5) {
    // Windows HiDPI / Mac Retina — OS maneja el scaling
    scale = w / baseline;
  } else {
    // Linux suele reportar DPR incorrecto — usamos pixeles físicos
    scale = physicalWidth / baseline;
  }

  return Math.max(0.75, Math.min(1.5, Math.round(scale * 20) / 20));
}

/**
 * Aplica las prefs al DOM vía variables CSS.
 */
function applyToDOM(p) {
  const root = document.documentElement;

  // Accent color
  const accent = ACCENT_COLORS[p.accentColor] || p.customAccentColor || ACCENT_COLORS.green;
  root.style.setProperty('--accent', accent);

  // Derivar accent-dim/glow del accent principal
  const rgb = hexToRgb(accent);
  if (rgb) {
    root.style.setProperty('--accent-dim', `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.12)`);
    root.style.setProperty('--accent-glow', `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.35)`);
  }

  // Taskbar height
  const tbH = p.taskbarSize === 'small' ? 44
            : p.taskbarSize === 'large' ? 60
            : 52;
  root.style.setProperty('--taskbar-height', tbH + 'px');

  // Text / UI scale
  root.style.setProperty('--text-scale', (p.textScale / 100).toString());
  root.style.setProperty('--glow-intensity', '0.5');

  const scale = computeUiScale(p.uiScale);
  root.style.setProperty('--ui-scale', scale.toString());
  root.style.setProperty('--ui-zoom', scale.toString());
  root.style.zoom = scale;

  // CRT overlay opcional (más agresivo que las scanlines base)
  root.classList.toggle('crt-overlay', !!p.crtOverlay);
}

function hexToRgb(hex) {
  const clean = hex.replace('#', '');
  if (clean.length !== 6) return null;
  return {
    r: parseInt(clean.slice(0, 2), 16),
    g: parseInt(clean.slice(2, 4), 16),
    b: parseInt(clean.slice(4, 6), 16),
  };
}

/**
 * Carga prefs desde servidor con fallback a localStorage y defaults.
 * Misma estrategia de 3 pasos que Beta 7.
 */
export async function loadPrefs() {
  // 1 · Prefs inyectadas server-side (instant)
  if (typeof document !== 'undefined') {
    const el = document.getElementById('__nimos_prefs_v1');
    if (el) {
      try {
        const serverPrefs = JSON.parse(atob(el.getAttribute('content')));
        const p = { ...DEFAULTS, ...serverPrefs };
        prefs.set(p);
        applyToDOM(p);
        localStorage.setItem('nimos-prefs', JSON.stringify(p));
        el.remove();
        return;
      } catch {}
    }
  }

  // 2 · Defaults + localStorage
  applyToDOM({ ...DEFAULTS });
  try {
    const cached = localStorage.getItem('nimos-prefs');
    if (cached) {
      const p = { ...DEFAULTS, ...JSON.parse(cached) };
      prefs.set(p);
      applyToDOM(p);
    }
  } catch {}

  // 3 · Fetch al backend
  const token = getToken();
  if (!token) return;

  try {
    const res = await fetch('/api/user/preferences', {
      headers: { 'Authorization': `Bearer ${token}` },
    });
    const data = await res.json();
    if (data.preferences) {
      const p = { ...DEFAULTS, ...data.preferences };
      prefs.set(p);
      applyToDOM(p);
      localStorage.setItem('nimos-prefs', JSON.stringify(p));
    }
  } catch (err) {
    console.error('[Prefs] Load failed:', err.message);
  }
}

export function setPref(key, value) {
  prefs.update(p => {
    const updated = { ...p, [key]: value };
    applyToDOM(updated);
    localStorage.setItem('nimos-prefs', JSON.stringify(updated));
    if (saveTimeout) clearTimeout(saveTimeout);
    saveTimeout = setTimeout(() => saveToServer(key, value), 1500);
    return updated;
  });
}

export function setPrefs(updates) {
  prefs.update(p => {
    const updated = { ...p, ...updates };
    applyToDOM(updated);
    localStorage.setItem('nimos-prefs', JSON.stringify(updated));
    if (saveTimeout) clearTimeout(saveTimeout);
    saveTimeout = setTimeout(() => saveToServer(null, null, updates), 1500);
    return updated;
  });
}

async function saveToServer(key, value, bulk = null) {
  const token = getToken();
  if (!token) return;
  try {
    const body = bulk || { [key]: value };
    await fetch('/api/user/preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify(body),
    });
  } catch {}
}

export { ACCENT_COLORS, DEFAULTS };
