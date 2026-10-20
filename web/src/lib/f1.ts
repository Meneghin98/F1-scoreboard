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

// Regolamento F1 2025: le regole di un torneo nuovo.
export const DEFAULT_RULES: ScoringRules = { points_mode: 'absolute', pole_points: 0, fastest_lap_points: 0 };

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
    hint: 'Le auto della IA non contano: il primo dei piloti prende 25 punti, il secondo 18 e così via.',
  },
];

// Le regole in breve: "Punti per posizione tra i piloti · pole +3 · giro veloce +1".
export function rulesSummary(rules: ScoringRules): string {
  const parts = [rules.points_mode === 'players' ? 'Punti per posizione tra i piloti' : "Punti per posizione d'arrivo, IA compresa"];
  if (rules.pole_points) parts.push(`pole +${rules.pole_points}`);
  if (rules.fastest_lap_points) parts.push(`giro veloce +${rules.fastest_lap_points}`);
  return parts.join(' · ');
}
