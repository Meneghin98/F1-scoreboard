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
  mapPoints: number;
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
  x: number;
  z: number;
}
export interface LiveWheel {
  surfaceTemp: number;
  innerTemp: number;
  pressure: number;
  wear: number;
  brakeTemp: number;
  brakeDamage: number;
}
// Auto del giocatore. Le ruote sono in ordine: ant. sx, ant. dx, post. sx, post. dx.
export interface PlayerCar {
  carIndex: number;
  speed: number;
  gear: number;
  rpm: number;
  maxRpm: number;
  throttle: number;
  brake: number;
  drs: boolean;
  engineTemp: number;
  drsAllowed: boolean;
  pitLimiter: boolean;
  brakeBias: number;
  fuelMix: number;
  fuelKg: number;
  fuelLaps: number;
  ersPercent: number;
  ersMode: number;
  ersDeployedMj: number;
  ersHarvestMj: number;
  wheels: LiveWheel[];
  damage: {
    frontWingLeft: number;
    frontWingRight: number;
    rearWing: number;
    floor: number;
    diffuser: number;
    sidepod: number;
    gearbox: number;
    engine: number;
    drsFault: boolean;
    ersFault: boolean;
  };
}

export interface LiveSnapshot {
  status: 'waiting' | 'live';
  session: LiveSession | null;
  drivers: LiveDriver[];
  player: PlayerCar | null;
  finished: boolean;
}

// Pianta del circuito ricostruita dalle posizioni delle auto (metri, X e Z del gioco).
export interface TrackMap {
  length: number;
  points: { x: number; z: number }[];
}

// --- Registrazioni (vedi internal/live/sessions.go) -----------------------

export interface SessionPlayer {
  carIndex: number;
  name: string; // nome mostrato dal gioco
  team: string;
  raceNumber: number;
  position: number | null;
  dnf: boolean;
  playerId: number | null; // pilota del segnapunti associato
  playerName: string | null;
}

export interface SessionRace {
  id: number;
  track: string;
  type: RaceType;
  date: string | null;
  tournamentId: number;
  tournamentName: string;
}

export interface SessionRecord {
  id: number;
  uid: string;
  format: number;
  trackId: number | null;
  track: string;
  sessionType: number | null;
  type: string;
  isRace: boolean;
  totalLaps: number | null;
  startedAt: string; // UTC, "AAAA-MM-GG hh:mm:ss"
  endedAt: string | null;
  recording: boolean;
  downloadable: boolean;
  sizeBytes: number;
  packets: number;
  hasClassification: boolean;
  cars: number | null;
  analyzed: boolean;
  players: SessionPlayer[];
  race: SessionRace | null;
  providesResults: boolean; // gara con classifica finale: i risultati del torneo vengono da qui
}

// Collegamento a una gara: esistente, nessuna (null) o nuova nel torneo indicato.
export type SessionRaceInput = { raceId: number | null } | { tournamentId: number; type: RaceType };
