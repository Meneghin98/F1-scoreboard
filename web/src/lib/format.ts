import type { Race, RaceType } from './types';

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

// Etichette brevi per le colonne: R1, R2… per i GP, S1… per le sprint, Q1… per
// le qualifiche, FP1… per le prove libere. Ogni tipo ha la sua numerazione.
const LABEL_PREFIX: Record<RaceType, string> = { race: 'R', sprint: 'S', qualifying: 'Q', practice: 'FP' };

export function raceLabels(races: Race[]): Map<number, string> {
  const counts: Partial<Record<RaceType, number>> = {};
  return new Map(races.map((race) => {
    counts[race.type] = (counts[race.type] ?? 0) + 1;
    return [race.id, `${LABEL_PREFIX[race.type]}${counts[race.type]}`];
  }));
}

// Orario salvato dal server in UTC ("2026-10-16 13:31:38") → data e ora locali.
export function formatDateTime(utc: string): string {
  return new Date(`${utc.replace(' ', 'T')}Z`).toLocaleString('it-IT', {
    weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

// Durata tra due orari UTC del server: "12:34" o "1:02:03".
export function formatDuration(fromUtc: string, toUtc: string): string {
  const parse = (s: string) => Date.parse(`${s.replace(' ', 'T')}Z`);
  return formatElapsed(Math.max(0, Math.round((parse(toUtc) - parse(fromUtc)) / 1000)));
}

// Tempo trascorso in secondi: 754 → "12:34", 3723 → "1:02:03".
export function formatElapsed(seconds: number): string {
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const clock = formatClock(total % 3600);
  return h ? `${h}:${clock.padStart(5, '0')}` : clock;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
