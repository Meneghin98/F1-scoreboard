// Dati di F1: suggerimenti per i campi circuito e scuderia (calendario e team F1 2025) e tipi di gara.
import type { RaceType } from './types';

export const TRACKS = [
  'Australia – Melbourne',
  'Cina – Shanghai',
  'Giappone – Suzuka',
  'Bahrain – Sakhir',
  'Arabia Saudita – Jeddah',
  'Miami',
  'Emilia-Romagna – Imola',
  'Monaco',
  'Spagna – Barcellona',
  'Canada – Montréal',
  'Austria – Red Bull Ring',
  'Gran Bretagna – Silverstone',
  'Belgio – Spa-Francorchamps',
  'Ungheria – Hungaroring',
  'Olanda – Zandvoort',
  'Italia – Monza',
  'Azerbaijan – Baku',
  'Singapore – Marina Bay',
  'USA – Austin',
  'Messico – Città del Messico',
  'Brasile – Interlagos',
  'Las Vegas',
  'Qatar – Lusail',
  'Abu Dhabi – Yas Marina',
];

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
