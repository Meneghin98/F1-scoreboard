import type { Race } from './types';

export function formatDate(iso: string | null): string {
  return iso ? new Date(`${iso}T00:00:00`).toLocaleDateString('it-IT') : '';
}

// Etichette brevi per le colonne: R1, R2… per i GP, S1, S2… per le sprint.
export function raceLabels(races: Race[]): Map<number, string> {
  let r = 0;
  let s = 0;
  return new Map(races.map((race) => [race.id, race.type === 'sprint' ? `S${++s}` : `R${++r}`]));
}
