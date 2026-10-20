import { useSyncExternalStore } from 'react';

// Navigazione tra le pagine con l'hash dell'URL (#/live, #/races…): il tasto
// "indietro" del browser funziona e una pagina si può aprire direttamente da un link.
// Alcune pagine hanno un parametro dopo il nome: #/replay/12.

export type View =
  | 'dashboard' | 'standings' | 'races' | 'stats' | 'players' | 'player' | 'tournaments' | 'live' | 'recordings' | 'replay'
  | 'system' | 'tv' | 'overlay';

export const VIEW_LABELS: Record<View, string> = {
  dashboard: 'Dashboard',
  standings: 'Classifica',
  races: 'Gare',
  stats: 'Statistiche',
  players: 'Piloti',
  player: 'Profilo',
  tournaments: 'Tornei',
  live: 'Live',
  recordings: 'Registrazioni',
  replay: 'Replay',
  system: 'Sistema',
  tv: 'Modalità TV',
  overlay: 'Overlay',
};

// Pagine nella barra di navigazione, in ordine. Sistema non c'è: si apre con l'ingranaggio nell'header.
export const NAV_VIEWS: View[] = [
  'dashboard', 'standings', 'races', 'stats', 'players', 'tournaments', 'live', 'recordings',
];

// Pagina della barra a cui appartiene una pagina che non c'è (il replay sta sotto le registrazioni).
export const VIEW_PARENT: Partial<Record<View, View>> = {
  replay: 'recordings',
  player: 'players',
};

// Pagine a schermo intero, senza header e barra di navigazione.
export const STANDALONE_VIEWS: View[] = ['tv', 'overlay'];

export function href(view: View, param?: number): string {
  if (view === 'dashboard') return '#/';
  return param === undefined ? `#/${view}` : `#/${view}/${param}`;
}

function hashParts(): string[] {
  return location.hash.replace(/^#\/?/, '').split('/');
}

function currentView(): View {
  const id = hashParts()[0];
  return id in VIEW_LABELS ? (id as View) : 'dashboard';
}

function currentParam(): number | null {
  const n = Number(hashParts()[1]);
  return Number.isInteger(n) && n > 0 ? n : null;
}

function subscribe(onChange: () => void) {
  window.addEventListener('hashchange', onChange);
  return () => window.removeEventListener('hashchange', onChange);
}

export function useView(): View {
  return useSyncExternalStore(subscribe, currentView);
}

// Parametro numerico della pagina (#/replay/12 → 12), null se manca.
export function useViewParam(): number | null {
  return useSyncExternalStore(subscribe, currentParam);
}

// Va alla pagina indicata.
export function navigate(view: View, param?: number) {
  location.hash = href(view, param);
}
