// Tipi delle risposte delle API: rispecchiano le struct in internal/api e internal/scoring.

export type RaceType = 'race' | 'sprint';

export interface Tournament {
  id: number;
  name: string;
  created_at: string;
}

export interface Player {
  id: number;
  name: string;
  team: string | null;
}

export interface Race {
  id: number;
  tournament_id: number;
  track: string;
  type: RaceType;
  date: string | null;
}

export interface RaceResult {
  player_id: number;
  position: number | null;
  dnf: boolean;
  points: number;
}

export interface RaceWithResults extends Race {
  results: RaceResult[];
}

export interface RaceScore {
  points: number;
  position: number | null;
  dnf: boolean;
}

export interface Standing {
  rank: number;
  player_id: number;
  name: string;
  team: string | null;
  points: number;
  wins: number;
  podiums: number;
  races: number;
  dnf: number;
  perRace: Record<number, RaceScore>;
}

export interface StandingsResponse {
  tournament: Tournament;
  races: Race[];
  standings: Standing[];
}

export interface ResultInput {
  player_id: number;
  position: number | null;
  dnf: boolean;
}
