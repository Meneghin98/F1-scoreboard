// Dati di F1: suggerimenti per il campo scuderia e tipi di gara. I circuiti
// arrivano dal server (GET /api/tracks), che li usa anche per le gare create
// dalle registrazioni.
import type { RaceType } from './types';

// Scuderie proposte per i piloti: la griglia 2025, più quelle del 2026.
export const TEAMS = [
  'McLaren',
  'Ferrari',
  'Red Bull Racing',
  'Mercedes',
  'Aston Martin',
  'Alpine',
  'Haas',
  'Racing Bulls',
  'Williams',
  'Kick Sauber',
  'Audi',
  'Cadillac',
];

// Tipi di gara del torneo. Prove libere e qualifiche mostrano i piazzamenti ma non danno punti.
export const RACE_TYPES: { value: RaceType; label: string }[] = [
  { value: 'race', label: 'Gran Premio' },
  { value: 'sprint', label: 'Sprint' },
  { value: 'qualifying', label: 'Qualifica' },
  { value: 'practice', label: 'Prove libere' },
];

export const raceTypeLabel = (type: RaceType) => RACE_TYPES.find((t) => t.value === type)?.label ?? type;

export const countsForPoints = (type: RaceType) => type === 'race' || type === 'sprint';
