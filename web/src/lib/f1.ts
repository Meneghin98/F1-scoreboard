// Dati di F1: suggerimenti per il campo scuderia e tipi di gara. I circuiti
// arrivano dal server (GET /api/tracks), che li usa anche per le gare create
// dalle registrazioni.
import type { PointsMode, RaceType, ScoringRules } from './types';

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

// Gare con la tabella dei punti: Gran Premi e sprint.
export const countsForPoints = (type: RaceType) => type === 'race' || type === 'sprint';

// Gare che danno punti nel torneo: anche le qualifiche, se la pole vale dei punti (vedi scoring.Rules.Counts).
export const givesPoints = (type: RaceType, rules: ScoringRules) =>
  countsForPoints(type) || (type === 'qualifying' && rules.pole_points > 0);

// Punti F1 2025 dal 1° posto in giù (come scoring.Points).
export const F1_POINTS = {
  race: [25, 18, 15, 12, 10, 8, 6, 4, 2, 1],
  sprint: [8, 7, 6, 5, 4, 3, 2, 1],
};

// Regolamento F1 2025: le regole di un torneo nuovo.
export const DEFAULT_RULES: ScoringRules = {
  points_mode: 'absolute', pole_points: 0, fastest_lap_points: 0, race_points: null, sprint_points: null,
};

// Modalità di punteggio, con la spiegazione da mostrare a chi sceglie.
export const POINTS_MODES: { value: PointsMode; label: string; hint: string }[] = [
  {
    value: 'absolute',
    label: "Posizione d'arrivo",
    hint: 'Come nel gioco, IA compresa: chi arriva 11° dietro a dieci auto della IA non prende punti.',
  },
  {
    value: 'players',
    label: 'Posizione tra i piloti',
    hint: 'Le auto della IA non contano: le posizioni si contano tra i soli piloti, con i punti della F1 o con quelli scelti qui sotto.',
  },
];

// Le regole in breve: "Punti per posizione tra i piloti · Gran Premio 10-5-2 · pole +3 · giro veloce +1".
export function rulesSummary(rules: ScoringRules): string {
  const players = rules.points_mode === 'players';
  const parts = [players ? 'Punti per posizione tra i piloti' : "Punti per posizione d'arrivo, IA compresa"];
  if (players && rules.race_points) parts.push(`Gran Premio ${rules.race_points.join('-')}`);
  if (players && rules.sprint_points) parts.push(`sprint ${rules.sprint_points.join('-')}`);
  if (rules.pole_points) parts.push(`pole +${rules.pole_points}`);
  if (rules.fastest_lap_points) parts.push(`giro veloce +${rules.fastest_lap_points}`);
  return parts.join(' · ');
}
