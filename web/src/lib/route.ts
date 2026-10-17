import { useSyncExternalStore } from 'react';

// Navigazione tra le pagine con l'hash dell'URL (#/live, #/races…): il tasto
// "indietro" del browser funziona e una pagina si può aprire direttamente da un link.

export type View = 'dashboard' | 'standings' | 'races' | 'players' | 'live' | 'recordings';

export const VIEW_LABELS: Record<View, string> = {
  dashboard: 'Dashboard',
  standings: 'Classifica',
  races: 'Gare',
  players: 'Piloti',
  live: 'Live',
  recordings: 'Registrazioni',
};

export function href(view: View): string {
  return view === 'dashboard' ? '#/' : `#/${view}`;
}

function currentView(): View {
  const id = location.hash.replace(/^#\/?/, '');
  return id in VIEW_LABELS ? (id as View) : 'dashboard';
}

function subscribe(onChange: () => void) {
  window.addEventListener('hashchange', onChange);
  return () => window.removeEventListener('hashchange', onChange);
}

export function useView(): View {
  return useSyncExternalStore(subscribe, currentView);
}
