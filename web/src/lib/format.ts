import type { Race } from './types';

export function formatDate(iso: string | null): string {
  return iso ? new Date(`${iso}T00:00:00`).toLocaleDateString('it-IT') : '';
}

// Tempo sul giro: 83456 → "1:23.456". 0 (nessun tempo) → "–".
export function formatLapTime(ms: number): string {
  if (!ms) return '–';
  const min = Math.floor(ms / 60000);
  const sec = (ms % 60000) / 1000;
  return min ? `${min}:${sec.toFixed(3).padStart(6, '0')}` : sec.toFixed(3);
}

// Distacco: 1234 → "+1.234", 62345 → "+1:02.345".
export function formatGap(ms: number): string {
  return ms ? `+${formatLapTime(ms)}` : '–';
}

// Tempo rimanente: 754 secondi → "12:34".
export function formatClock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

// Etichette brevi per le colonne: R1, R2… per i GP, S1, S2… per le sprint.
export function raceLabels(races: Race[]): Map<number, string> {
  let r = 0;
  let s = 0;
  return new Map(races.map((race) => [race.id, race.type === 'sprint' ? `S${++s}` : `R${++r}`]));
}
