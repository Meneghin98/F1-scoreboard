// Tipi delle risposte delle API: rispecchiano le struct in internal/api e internal/scoring.

export type RaceType = 'race' | 'sprint' | 'qualifying' | 'practice';

export interface Tournament {
  id: number;
  name: string;
  created_at: string;
  free: boolean; // "Nessun torneo": raccoglie le gare quando nessun torneo è in corso
  started_at: string | null; // UTC
  ended_at: string | null; // UTC
  active: boolean; // in corso: le gare finite vengono salvate qui
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
  controller: number; // 1 o 2 per le auto dei giocatori (split-screen), 0 per le altre
  playerName: string; // pilota del segnapunti alla guida, vuoto se non scelto
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
// Auto di un giocatore. Le ruote sono in ordine: ant. sx, ant. dx, post. sx, post. dx.
export interface PlayerCar {
  carIndex: number;
  controller: number; // 1 o 2 (split-screen)
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
  // live: dati in arrivo; paused: sessione aperta ma il gioco non ne invia (pausa, menù); waiting: nessuna sessione.
  status: 'waiting' | 'live' | 'paused';
  session: LiveSession | null;
  drivers: LiveDriver[];
  players: PlayerCar[]; // auto dei giocatori, in ordine di controller
  finished: boolean;
  lineup: Lineup | null;
}

// Chi guida le auto dei giocatori in gara (vedi internal/live/lineup.go).
// drivers: scelta prima della partenza; racing: gara in corso; results: conferma
// dei risultati; saved: gara salvata nel torneo.
export type LineupStage = 'drivers' | 'racing' | 'results' | 'saved';

export interface LineupCar {
  controller: number; // 1 o 2
  carIndex: number;
  name: string; // nome mostrato dal gioco
  team: string;
  raceNumber: number;
  playerId: number | null;
  playerName: string;
  position: number | null; // dalla classifica finale
  dnf: boolean;
}

// Gruppo del tipo di sessione: 'race' comprende le sprint, 'qualifying' lo Sprint Shootout.
export type SessionKind = 'race' | 'qualifying' | 'practice' | '';

export interface Lineup {
  stage: LineupStage;
  kind: SessionKind;
  secondsLeft: number | null; // al salvataggio automatico, null se non previsto
  cars: LineupCar[];
  tournament: { id: number; name: string; free: boolean } | null; // dove finisce (o è finita) la gara
  raceId: number | null;
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
  kind: SessionKind; // '' per le sessioni fuori dai tornei (prova a tempo)
  providesResults: boolean; // sessione del weekend con classifica finale: i risultati del torneo vengono da qui
  deleteAt: string | null; // UTC: eliminata perché non terminata (senza classifica finale e senza torneo)
}

// Collegamento a una gara: esistente, nessuna (null) o nuova nel torneo indicato.
export type SessionRaceInput = { raceId: number | null } | { tournamentId: number; type: RaceType };

// --- Replay (vedi internal/live/replay.go) ----------------------------------

export interface ReplayLap {
  lap: number;
  atMs: number; // quando il primo in classifica inizia il giro
}

export interface ReplayInfo {
  durationMs: number;
  mapPoints: number; // punti della pianta completa, 0 se non c'è
  laps: ReplayLap[];
}

// Acceleratore e freno (0..1) del controller indicato all'istante t, in ms dall'inizio del replay.
export interface ReplayPedal {
  t: number;
  controller: number;
  throttle: number;
  brake: number;
}

export interface ReplayFrame {
  t: number;
  snapshot: LiveSnapshot;
  pedals: ReplayPedal[]; // di tutti i giocatori, dal più vecchio
}
