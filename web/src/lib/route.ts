import { useSyncExternalStore } from 'react';

// Navigazione tra le pagine con l'hash dell'URL (#/live, #/races…): il tasto
// "indietro" del browser funziona e una pagina si può aprire direttamente da un link.
// Alcune pagine hanno un parametro dopo il nome: #/replay/12.

export type View = 'dashboard' | 'standings' | 'races' | 'players' | 'live' | 'recordings' | 'replay';

export const VIEW_LABELS: Record<View, string> = {
  dashboard: 'Dashboard',
  standings: 'Classifica',
  races: 'Gare',
  players: 'Piloti',
  live: 'Live',
  recordings: 'Registrazioni',
  replay: 'Replay',
};

// Pagina da cui si arriva, mostrata nel percorso tra la dashboard e la pagina attuale.
export const VIEW_PARENT: Partial<Record<View, View>> = {
  replay: 'recordings',
};

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
