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

// --- Telemetria live (vedi internal/live/tracker.go) ----------------------

export interface LiveSession {
  uid: string;
  format: number;
  trackId: number;
  track: string;
  sessionType: number;
  type: string;
  isRace: boolean;
  totalLaps: number;
  currentLap: number;
  timeLeft: number;
  paused: boolean;
  safetyCar: number;
  weather: number;
  trackTemp: number;
  airTemp: number;
}

export interface LiveDriver {
  carIndex: number;
  position: number;
  name: string;
  team: string;
  raceNumber: number;
  isPlayer: boolean;
  ai: boolean;
  lap: number;
  lastLapMs: number;
  bestLapMs: number;
  gapToLeaderMs: number;
  gapAheadMs: number;
  inPit: boolean;
  pitStops: number;
  penaltiesSec: number;
  status: string;
  tyre: string;
  tyreAge: number;
}

export interface LiveSnapshot {
  status: 'waiting' | 'live';
  session: LiveSession | null;
  drivers: LiveDriver[];
  finished: boolean;
}
