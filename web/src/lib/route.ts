import { useSyncExternalStore } from 'react';

// Navigazione tra le pagine con l'hash dell'URL (#/live, #/races…): il tasto
// "indietro" del browser funziona e una pagina si può aprire direttamente da un link.
// Alcune pagine hanno un parametro dopo il nome (#/replay/12) e i filtri dopo il "?"
// (#/races?torneo=5): un filtro che manca ha il valore di default della pagina.

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

// Pagine nella barra di navigazione, nello stesso ordine delle card della dashboard.
// Sistema non c'è: si apre con l'ingranaggio nell'header.
export const NAV_VIEWS: View[] = [
  'dashboard', 'live', 'standings', 'races', 'players', 'recordings', 'tournaments', 'stats',
];

// Pagina della barra a cui appartiene una pagina che non c'è (il replay sta sotto le registrazioni).
export const VIEW_PARENT: Partial<Record<View, View>> = {
  replay: 'recordings',
  player: 'players',
};

// Pagine a schermo intero, senza header e barra di navigazione.
export const STANDALONE_VIEWS: View[] = ['tv', 'overlay'];

// Filtri di una pagina: null o undefined li toglie dall'indirizzo.
export type Filters = Record<string, string | number | null | undefined>;

export function href(view: View, param?: number, filters?: Filters): string {
  const path = view === 'dashboard' ? '#/' : param === undefined ? `#/${view}` : `#/${view}/${param}`;
  return path + queryString(new URLSearchParams(), filters ?? {});
}

function queryString(query: URLSearchParams, changes: Filters): string {
  for (const [name, value] of Object.entries(changes)) {
    if (value === null || value === undefined) query.delete(name);
    else query.set(name, String(value));
  }
  const q = query.toString();
  return q ? `?${q}` : '';
}

// "races/5" di "#/races/5?torneo=2".
function hashPath(hash: string): string {
  return hash.replace(/^#\/?/, '').split('?')[0];
}

function hashQuery(): URLSearchParams {
  const i = location.hash.indexOf('?');
  return new URLSearchParams(i < 0 ? '' : location.hash.slice(i + 1));
}

function viewOf(hash: string): View {
  const id = hashPath(hash).split('/')[0];
  return id in VIEW_LABELS ? (id as View) : 'dashboard';
}

function currentView(): View {
  return viewOf(location.hash);
}

function currentParam(): number | null {
  const n = Number(hashPath(location.hash).split('/')[1]);
  return Number.isInteger(n) && n > 0 ? n : null;
}

// replaceState non fa scattare hashchange: chi ascolta viene avvisato a mano.
const listeners = new Set<() => void>();

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  window.addEventListener('hashchange', onChange);
  return () => {
    listeners.delete(onChange);
    window.removeEventListener('hashchange', onChange);
  };
}

// Indirizzo della pagina aperta prima di quella attuale, con i suoi filtri.
let previousHash: string | null = null;
window.addEventListener('hashchange', (e) => {
  previousHash = new URL(e.oldURL).hash;
});

// Pagina aperta prima di quella attuale, null se non si sa (es. dopo aver ricaricato).
export function previousView(): View | null {
  return previousHash === null ? null : viewOf(previousHash);
}

export function useView(): View {
  return useSyncExternalStore(subscribe, currentView);
}

// Parametro numerico della pagina (#/replay/12 → 12), null se manca.
export function useViewParam(): number | null {
  return useSyncExternalStore(subscribe, currentParam);
}

// Valore di un filtro nell'indirizzo, null se manca.
export function useFilter(name: string): string | null {
  return useSyncExternalStore(subscribe, () => hashQuery().get(name));
}

// Cambia i filtri della pagina aperta senza aggiungere un passo alla cronologia:
// "indietro" porta alla pagina precedente, non al filtro precedente.
export function setFilters(changes: Filters) {
  const hash = `#/${hashPath(location.hash)}${queryString(hashQuery(), changes)}`;
  if (hash === (location.hash || '#/')) return;
  history.replaceState(history.state, '', hash);
  for (const onChange of listeners) onChange();
}

// Va alla pagina indicata.
export function navigate(view: View, param?: number, filters?: Filters) {
  location.hash = href(view, param, filters);
}
