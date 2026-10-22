// Dati di F1: suggerimenti per il campo scuderia e tipi di gara. I circuiti
// arrivano dal server (GET /api/tracks), che li usa anche per le gare create
// dalle registrazioni.
import type { PointsMode, PointsPreset, RaceType, ScoringRules } from './types';

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
  points_mode: 'absolute', pole_points: 0, fastest_lap_points: 0, points_preset: 'ultimate_champion', race_points: null, sprint_points: null,
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
    hint: 'Le auto della IA non contano: le posizioni si contano tra i soli piloti, con la tabella dei punti scelta qui sotto.',
  },
];

// Tabelle dei punti per posizione in modalità players, nell'ordine della select.
// I punti li calcola il server (scoring.Presets): una tabella nuova va aggiunta
// anche lì, e al tipo PointsPreset.
export const POINTS_PRESETS: { value: PointsPreset; label: string; hint: string }[] = [
  {
    value: 'ultimate_champion',
    label: 'Ultimate Champion',
    hint: '1 punto al primo tra i piloti, 0 agli altri; +1 a chi vince davanti alla IA. Le sprint danno solo il +1.',
  },
  {
    value: 'rank_down',
    label: 'Rank Down',
    hint: 'Al primo tanti punti quanti i piloti in gara, uno in meno a ogni posizione: con 4 piloti 4-3-2-1.',
  },
  { value: 'all_or_nothing', label: 'All or Nothing', hint: 'Solo il vincitore prende punti: 25 nei Gran Premi, 8 nelle sprint.' },
  {
    value: 'push_and_pay',
    label: 'Push & Pay',
    hint: 'Il vincitore prende 2 punti, tutti gli altri ne perdono 1, ritirati compresi: si può andare sotto zero.',
  },
  { value: 'f1', label: 'F1', hint: 'Gran Premi 25-18-15-12-10-8-6-4-2-1, sprint 8-7-6-5-4-3-2-1.' },
  { value: 'custom', label: 'Custom', hint: 'Scegli i punti di ogni posizione (0 per non darne).' },
];

const pointsPresetLabel = (preset: PointsPreset) => POINTS_PRESETS.find((p) => p.value === preset)?.label ?? preset;

// Le regole in breve: "Punti per posizione tra i piloti (Rank Down) · pole +3 · giro veloce +1";
// con la tabella custom, al posto del nome ci sono i punti.
export function rulesSummary(rules: ScoringRules): string {
  const parts = ["Punti per posizione d'arrivo, IA compresa"];
  if (rules.points_mode === 'players') {
    if (rules.points_preset === 'custom') {
      parts[0] = 'Punti per posizione tra i piloti';
      parts.push(`Gran Premio ${(rules.race_points ?? F1_POINTS.race).join('-')}`);
      parts.push(`sprint ${(rules.sprint_points ?? F1_POINTS.sprint).join('-')}`);
    } else {
      parts[0] = `Punti per posizione tra i piloti (${pointsPresetLabel(rules.points_preset)})`;
    }
  }
  if (rules.pole_points) parts.push(`pole +${rules.pole_points}`);
  if (rules.fastest_lap_points) parts.push(`giro veloce +${rules.fastest_lap_points}`);
  return parts.join(' · ');
}
