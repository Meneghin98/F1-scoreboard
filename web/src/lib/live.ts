// Etichette per i dati della telemetria live.
import type { LiveSession, LiveSnapshot } from './types';

export const WEATHER = ['Sereno', 'Poco nuvoloso', 'Nuvoloso', 'Pioggia leggera', 'Pioggia forte', 'Temporale'];

// Indici come il campo safetyCar: 0 nessuna, 1 safety car, 2 virtual, 3 giro di formazione.
export const SAFETY_CAR = ['', 'Safety car', 'Virtual safety car', 'Giro di formazione'];

// Indici come i campi fuelMix ed ersMode dell'auto del giocatore.
export const FUEL_MIX = ['Povera', 'Standard', 'Ricca', 'Massima'];
export const ERS_MODE = ['Spento', 'Medio', 'Hotlap', 'Sorpasso'];

// Gravità di usura e danni: "warn" oltre la soglia di attenzione, "bad" oltre quella critica.
export type Severity = 'ok' | 'warn' | 'bad';
export function severity(value: number, warn: number, bad: number): Severity {
  if (value >= bad) return 'bad';
  if (value >= warn) return 'warn';
  return 'ok';
}

// Colori delle mescole come nella grafica TV.
export const TYRE_CLASS: Record<string, string> = {
  S: 'border-red-500 text-red-400',
  M: 'border-yellow-400 text-yellow-300',
  H: 'border-gray-200 text-gray-100',
  I: 'border-green-500 text-green-400',
  W: 'border-blue-500 text-blue-400',
};

// Colori delle scuderie per la mappa. Il gioco aggiunge l'anno alle squadre
// delle stagioni passate ("Ferrari '24"): si usa il colore della squadra.
const TEAM_COLORS: Record<string, string> = {
  Mercedes: '#27f4d2',
  Ferrari: '#e8002d',
  'Red Bull Racing': '#3671c6',
  Williams: '#64c4ff',
  'Aston Martin': '#229971',
  Alpine: '#ff87bc',
  RB: '#6692ff',
  Haas: '#b6babd',
  McLaren: '#ff8000',
  Sauber: '#52e252',
  Audi: '#bb0a30',
  Cadillac: '#d4d4d8',
};

export function teamColor(team: string): string {
  return TEAM_COLORS[team.replace(/ '\d\d$/, '')] ?? '#9a9aae';
}

// Etichetta del giocatore che guida con il controller indicato (1 o 2): "P1", "P2".
export function controllerLabel(controller: number): string {
  return `P${controller}`;
}

// C'è una sessione aperta, anche in pausa, e il gioco ne ha già comunicato i dettagli.
export function isLive(live: LiveSnapshot | null): live is LiveSnapshot & { session: NonNullable<LiveSnapshot['session']> } {
  return !!live && live.status !== 'waiting' && live.session !== null;
}

// In pausa: il gioco è nei menù o in pausa e non invia dati, oppure lo dice la sessione.
// A fine gara il gioco risulta in pausa sulla schermata dei risultati: non serve dirlo.
export function isPaused(live: LiveSnapshot): boolean {
  return (live.status === 'paused' || !!live.session?.paused) && !live.finished;
}

// Il gioco ha già cambiato sessione ma non ne ha ancora inviato i dettagli: mentre
// carica manda solo eventi. Circuito e piloti arrivano tutti insieme quando si è ai box o in pista.
export function isLoading(session: LiveSession): boolean {
  return session.trackId < 0;
}

// Riassunto in una riga: "Monza · Gara · Giro 12/20".
export function sessionSummary(live: LiveSnapshot): string {
  const s = live.session;
  if (!s) return '';
  if (isLoading(s)) return 'Sessione in caricamento…';
  const details = [s.track, s.type];
  if (s.isRace && s.totalLaps > 0) {
    details.push(live.finished ? 'Finita' : `Giro ${Math.max(s.currentLap, 1)}/${s.totalLaps}`);
  }
  if (isPaused(live)) details.push('In pausa');
  return details.filter(Boolean).join(' · ') || 'Sessione in corso';
}
