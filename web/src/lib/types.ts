// Tipi delle risposte delle API: rispecchiano le struct in internal/api e internal/scoring.

export type RaceType = 'race' | 'sprint' | 'qualifying' | 'practice';

// absolute: punti in base alla posizione d'arrivo, IA compresa; players: in base
// alla posizione tra i soli piloti, come se la IA non ci fosse.
export type PointsMode = 'absolute' | 'players';

// Tabella dei punti per posizione in modalità players (vedi scoring.Presets e POINTS_PRESETS):
// custom usa i punti di race_points e sprint_points.
export type PointsPreset = 'ultimate_champion' | 'f1' | 'rank_down' | 'all_or_nothing' | 'push_and_pay' | 'custom';

// Regole di punteggio di un torneo (vedi scoring.Rules).
export interface ScoringRules {
  points_mode: PointsMode;
  pole_points: number; // al primo della qualifica
  fastest_lap_points: number; // al giro più veloce di un Gran Premio, se non si ritira
  points_preset: PointsPreset;
  // Punti per posizione con la tabella custom, dal 1° in giù: null vale la tabella F1.
  // Inviando null la tabella resta com'è.
  race_points: number[] | null;
  sprint_points: number[] | null;
}

export interface Tournament extends ScoringRules {
  id: number;
  name: string;
  created_at: string;
  free: boolean; // "Nessun torneo": raccoglie le gare quando nessun torneo è in corso
  started_at: string | null; // UTC
  ended_at: string | null; // UTC
  active: boolean; // in corso: le gare finite vengono salvate qui
  races: number; // gare del torneo
  recordings: number; // registrazioni collegate alle sue gare
  rotation: boolean; // turnazione dei controller attiva (vedi internal/rotation)
  leader?: { name: string; color: string; points: number }; // primo in classifica (solo nell'elenco dei tornei)
}

// Circuito proposto per le gare (GET /api/tracks), nell'ordine del calendario seguito dalle varianti.
export interface Track {
  id: number;
  name: string;
  excluded: boolean; // da non suggerire nel programma, scelto nella pagina Piste
}

export interface Player {
  id: number;
  name: string;
  team: string | null;
  color: string; // "#rrggbb": lo stesso nei grafici, nella classifica live e sulla mappa
}

export interface Race {
  id: number;
  tournament_id: number;
  track: string;
  track_id: number | null; // circuito del gioco, null se il nome non è tra quelli noti
  type: RaceType;
  date: string | null;
  manual_results: boolean; // risultati modificati a mano: la registrazione non li riscrive
}

export interface RaceResult {
  player_id: number;
  position: number | null;
  dnf: boolean;
  fastest_lap: boolean; // giro più veloce della gara, IA compresa, come salvato
  best_lap_ms: number | null; // miglior giro, solo dalle registrazioni
  // Come conta il risultato con le regole del torneo.
  points: number; // compresi i bonus
  scored_position: number | null; // posizione che dà i punti: tra i soli piloti in modalità "players"
  bonus: number; // punti per la pole o il giro veloce, già compresi in points
  win_bonus: number; // punti per la vittoria davanti alla IA (Ultimate Champion), già compresi in points
}

export interface RaceWithResults extends Race {
  results: RaceResult[];
  // Registrazioni collegate, dalla più recente: quella con source dà i risultati.
  recordings: RaceRecording[];
}

export interface RaceRecording {
  id: number;
  source: boolean;
  restarts: number | null; // volte che la sessione è stata ricominciata, null se non ancora contate
  flashbacks: number | null; // flashback usati, null se non ancora contati
}

export interface RaceScore {
  points: number; // compresi i bonus
  position: number | null; // posizione che dà i punti: tra i soli piloti in modalità "players"
  dnf: boolean;
  bonus: number; // punti della pole (nelle qualifiche) o del giro veloce (nei Gran Premi)
  win_bonus: number; // punti per la vittoria davanti alla IA (Ultimate Champion), già compresi in points
}

export interface Standing {
  rank: number;
  player_id: number;
  name: string;
  team: string | null;
  color: string;
  points: number;
  wins: number;
  podiums: number;
  races: number;
  dnf: number;
  poles: number; // qualifiche vinte
  fastestLaps: number; // giri più veloci nei Gran Premi
  perRace: Record<number, RaceScore>;
}

export interface StandingsResponse {
  tournament: Tournament;
  races: StandingRace[];
  standings: Standing[];
}

// Gara in classifica, con riavvii e flashback della sua registrazione
// (null se non ne ha o non sono ancora contati).
export interface StandingRace extends Race {
  restarts: number | null;
  flashbacks: number | null;
}

export interface ResultInput {
  player_id: number;
  position: number | null;
  dnf: boolean;
  fastest_lap: boolean;
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
  sectors: [number, number]; // metri dal traguardo a cui iniziano il settore 2 e il 3, 0 se non ancora noti
  forecast: LiveForecast[]; // previsioni per il resto della sessione
  pitWindowIdeal: number; // giro ideale per la sosta, 0 se non previsto
  pitWindowLatest: number;
  pitRejoin: number; // posizione prevista al rientro dai box del giocatore principale
  marshalZones: MarshalZone[];
}

export interface LiveForecast {
  offset: number; // minuti da adesso
  weather: number;
  rain: number; // probabilità di pioggia, %
  trackTemp: number;
  airTemp: number;
}

// Settore dei commissari: va da start all'inizio del successivo (frazioni del giro).
export interface MarshalZone {
  start: number;
  flag: number; // -1 sconosciuta, 0 nessuna, 1 verde, 2 blu, 3 gialla
}

export interface Stint {
  tyre: string; // S, M, H, I, W
  endLap: number; // ultimo giro, 0 se ancora in uso
}

export interface LiveDriver {
  carIndex: number;
  position: number;
  name: string;
  team: string;
  raceNumber: number;
  controller: number; // 1 o 2 per le auto dei giocatori (split-screen), 0 per le altre
  playerName: string; // pilota del segnapunti alla guida, vuoto se non scelto
  playerColor: string; // suo colore, vuoto se non scelto
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
  gridPosition: number;
  lapInvalid: boolean; // giro in corso annullato
  warnings: number;
  cornerCuts: number; // avvertimenti per taglio di curva
  unserved: number; // drive through e stop & go da scontare
  speedTrap: number; // km/h, 0 se non rilevata
  lastSectors: number[]; // settori dell'ultimo giro completato, ms
  bestSectors: number[]; // migliori settori personali, ms
  stints: Stint[];
  drs: boolean; // ala mobile aperta
  overtake: boolean; // Overtake Mode (regolamento 2026) o ERS in modalità sorpasso
  blueFlag: boolean; // bandiera blu: sta per essere doppiato
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
  // Il gioco nasconde carburante, ERS, ripartizione freni, danni, usura, set di
  // gomme e assetto di quest'auto (il giocatore 2 in split-screen): sono a zero.
  restricted: boolean;
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
  tyreSets: TyreSet[]; // set ancora disponibili, quello montato per primo
  setup: CarSetup | null;
}

export interface TyreSet {
  tyre: string;
  wear: number; // %
  lifeSpan: number; // giri rimanenti
  deltaMs: number; // differenza sul giro rispetto al set montato
  fitted: boolean;
}

export interface CarSetup {
  frontWing: number;
  rearWing: number;
  onThrottle: number; // differenziale in accelerazione, %
  offThrottle: number;
  frontCamber: number;
  rearCamber: number;
  frontToe: number;
  rearToe: number;
  frontSuspension: number;
  rearSuspension: number;
  frontAntiRollBar: number;
  rearAntiRollBar: number;
  frontHeight: number;
  rearHeight: number;
  brakePressure: number; // %
  brakeBias: number; // %
  engineBraking: number; // %
  tyrePressures: number[]; // PSI: ant. sx, ant. dx, post. sx, post. dx
  fuelLoad: number; // kg
}

// Tipi dei messaggi della direzione gara (vedi internal/live/events.go).
export type RaceEventKind =
  | 'penalty' | 'collision' | 'overtake' | 'fastestLap' | 'retirement' | 'safetyCar' | 'redFlag' | 'flag' | 'drs'
  | 'flashback';

export interface RaceEvent {
  id: number;
  time: number; // secondi dall'inizio della sessione
  lap: number; // giro del primo in classifica
  kind: RaceEventKind;
  text: string;
  cars: number[]; // auto coinvolte
}

export interface LiveSnapshot {
  // live: dati in arrivo; paused: sessione aperta ma il gioco non ne invia (pausa, menù); waiting: nessuna sessione.
  status: 'waiting' | 'live' | 'paused';
  session: LiveSession | null;
  drivers: LiveDriver[];
  players: PlayerCar[]; // auto dei giocatori, in ordine di controller
  finished: boolean;
  lineup: Lineup | null;
  lastSaved?: SavedLineup | null; // ultima sessione salvata nel torneo, anche se già finita (solo nello stato live)
  rotation?: RotationState; // turnazione dei controller nel torneo in corso (solo nello stato live)
  events: RaceEvent[]; // ultimi messaggi della direzione gara, dal più vecchio
  telemetry?: TelemetryStatus; // solo nello stato live, non nei replay
  dataVersion?: number; // cambia a ogni modifica dei dati salvati: le pagine si ricaricano (solo nello stato live)
}

// Ricezione della telemetria (vedi internal/live/diagnostics.go).
export interface TelemetryStatus {
  port: number; // porta UDP in ascolto, 0 se non è aperta
  error: string; // perché la porta non si apre
  addresses: string[]; // indirizzi IP del server da impostare nel gioco, il principale per primo
  badFormat: number; // formato UDP non supportato arrivato da poco, 0 se nessuno
}

// Chi guida le auto dei giocatori in gara (vedi internal/live/lineup.go).
// drivers: scelta prima della partenza; racing: gara in corso; results: classifica
// finale, in attesa del salvataggio; saved: gara salvata nel torneo.
export type LineupStage = 'drivers' | 'racing' | 'results' | 'saved';

export interface LineupCar {
  controller: number; // 1 o 2
  carIndex: number;
  name: string; // nome mostrato dal gioco
  team: string;
  raceNumber: number;
  playerId: number | null;
  playerName: string;
  playerColor: string;
  position: number | null; // dalla classifica finale
  dnf: boolean;
}

// Gruppo del tipo di sessione: 'race' comprende le sprint, 'qualifying' lo Sprint Shootout.
export type SessionKind = 'race' | 'qualifying' | 'practice' | '';

export interface LineupTournament {
  id: number;
  name: string;
  free: boolean;
}

export interface Lineup {
  sessionId: number; // la conferma vale solo per questa sessione
  stage: LineupStage;
  kind: SessionKind;
  secondsLeft: number | null; // al salvataggio automatico, null se non previsto
  cars: LineupCar[];
  tournament: LineupTournament | null; // dove finisce (o è finita) la gara
  raceId: number | null;
  error: string; // perché il salvataggio non è riuscito (si riprova da soli), vuoto se nessun errore
}

// Riepilogo dell'ultima sessione salvata, mostrato finché non parte la successiva.
export interface SavedLineup extends Lineup {
  track: string;
  session: string; // tipo della sessione, per esempio "Gara"
}

// Pianta del circuito ricostruita dalle posizioni delle auto (metri, X e Z del gioco).
export interface TrackMap {
  length: number;
  points: { x: number; z: number; d: number }[]; // d: metri dall'inizio del giro
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
  manualResults: boolean; // risultati della gara modificati a mano: cambiare i piloti non li aggiorna
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

export interface ReplayEvent extends RaceEvent {
  atMs: number; // istante del replay
}

export interface ReplayInfo {
  durationMs: number;
  mapPoints: number; // punti della pianta completa, 0 se non c'è
  laps: ReplayLap[];
  events: ReplayEvent[];
  analysis: Analysis | null; // riassunto della sessione alla fine della registrazione
}

// Riassunto di una sessione (vedi internal/live/analysis.go).
export interface Analysis {
  laps: number; // giri con le posizioni: il primo (0) è la griglia
  cars: AnalysisCar[]; // in ordine di classifica
}

export interface AnalysisCar {
  carIndex: number;
  position: number;
  name: string;
  team: string;
  controller: number;
  playerName: string;
  playerColor: string;
  human: boolean;
  status: string;
  positions: number[]; // posizione all'inizio di ogni giro, 0 se manca
  stints: Stint[];
  laps: { timeMs: number; sectors: number[]; valid: boolean }[];
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

// --- Statistiche (vedi internal/api/stats.go) ---------------------------------

export interface CareerStats {
  player_id: number;
  name: string;
  team: string | null;
  color: string;
  races: number; // Gran Premi corsi
  sprints: number;
  wins: number; // vittorie nei Gran Premi
  sprintWins: number;
  podiums: number;
  poles: number;
  fastestLaps: number;
  dnf: number;
  avgFinish: number | null; // media dei piazzamenti nei Gran Premi finiti
  bestFinish: number | null;
  points: number; // somma dei punti nelle classifiche dei tornei
  tournaments: number;
  titles: number; // tornei terminati da primo
}

// Giro più veloce dei piloti su un circuito, dalle registrazioni.
export interface TrackRecord {
  trackId: number;
  track: string;
  lapMs: number;
  player_id: number;
  name: string;
  color: string;
  sessionId: number;
  sessionType: string;
  date: string; // UTC
}

// Record di un circuito in gara e in qualifica, null se ancora nessuno. I giri delle prove non contano.
export interface TrackRecords {
  trackId: number;
  track: string;
  race: TrackRecord | null;
  qualifying: TrackRecord | null;
}

export interface H2HPlayer {
  id: number;
  name: string;
  color: string;
}

export interface StatsResponse {
  career: CareerStats[]; // dal migliore
  records: TrackRecords[]; // nell'ordine del calendario
  headToHead: { players: H2HPlayer[]; ahead: number[][] }; // ahead[i][j]: gare in cui i è arrivato davanti a j
}

export interface PlayerProfile {
  player: Player;
  career: CareerStats;
  tournaments: { id: number; name: string; rank: number; of: number; points: number; wins: number; active: boolean; ended: boolean }[];
  bestLaps: { trackId: number; track: string; lapMs: number; sessionId: number; sessionType: string; date: string; record: boolean }[];
  recent: {
    raceId: number;
    tournamentId: number;
    tournamentName: string;
    track: string;
    type: RaceType;
    date: string | null;
    position: number | null;
    dnf: boolean;
    fastestLap: boolean;
  }[];
  rivals: (H2HPlayer & { ahead: number; behind: number })[];
}

// --- Sistema (vedi internal/api/system.go) -----------------------------------

export interface SystemInfo {
  version: {
    number: string; // versione semver (es. "1.0.0"), vuota nelle build di sviluppo
    time: string; // data del commit, RFC 3339, vuota se compilato senza Git (o con go run)
  };
  update: UpdateStatus;
  database: { path: string; sizeBytes: number };
  recordings: {
    dir: string;
    enabled: boolean;
    files: number;
    sizeBytes: number;
    unfinished: { count: number; sizeBytes: number }; // sessioni non terminate, eliminate dopo 48 ore
  };
  backups: { dir: string; keep: number; count: number; sizeBytes: number }; // keep 0: backup automatici spenti
  disk: { freeBytes: number; totalBytes: number } | null; // disco della cartella dei dati, null se non si sa
}

// Aggiornamenti dalle release su GitHub (vedi internal/update/update.go).
export interface UpdateStatus {
  enabled: boolean; // false nelle build di sviluppo o con UPDATE=false
  current: string; // versione in esecuzione, vuota nelle build di sviluppo
  latest: string; // ultima release su GitHub, vuota se non ancora controllato
  staged: string; // versione scaricata, da installare
  checkedAt: string; // ultimo controllo riuscito (UTC), vuoto se nessuno
  error: string; // errore dell'ultimo controllo o download
  available: string; // versione nuova trovata e non ancora scaricata
  phase: '' | 'checking' | 'downloading'; // operazione in corso, vuota se nessuna
  size: number; // byte dell'eseguibile da scaricare, 0 se non si sa
  downloaded: number; // byte già scaricati durante un download
  installHour: number; // ora (locale) del riavvio notturno che installa la versione scaricata
}

// Backup automatico del database.
export interface BackupInfo {
  name: string;
  sizeBytes: number;
  createdAt: string; // UTC
}

// Conservazione delle registrazioni: dopo days giorni se ne eliminano i file (0 = mai).
export interface Retention {
  days: number;
  keepRecords: boolean; // tieni i file delle sessioni che detengono un record
}

export interface Settings {
  recordings: Retention;
}

// Risposta al salvataggio: i file delle registrazioni eliminati subito con le nuove impostazioni.
export interface SettingsSaved extends Settings {
  deletedFiles: number;
  freedBytes: number;
}

// Risposta di "Libera spazio": sessioni non terminate eliminate e spazio liberato.
export interface UnfinishedDeleted {
  deletedSessions: number;
  freedBytes: number;
}

// Turnazione dei controller nel torneo in corso (vedi internal/rotation).
export interface RotationPair {
  p1: number; // pilota al controller 1
  p2: number; // pilota al controller 2
}

export interface RotationState {
  tournament: { id: number; name: string } | null; // senza torneo in corso la turnazione non c'è
  present: number[]; // piloti presenti, scelti nella pagina Piloti
  enabled: boolean; // turnazione attiva nel torneo in corso: si spegne nelle impostazioni del torneo
  canDraw: boolean; // turnazione attiva e torneo senza Gran Premi: il sorteggio si può rifare
  // Prossime coppie dal prossimo Gran Premio, almeno 3 e una per pista del programma;
  // vuoto con la turnazione spenta o meno di due presenti.
  next: RotationPair[];
  plan: PlannedRace[]; // piste del programma ancora da correre, dal prossimo Gran Premio
}

// Gran Premio del programma: pista suggerita e chi corre secondo la turnazione.
export interface PlannedRace {
  track: number; // id del circuito (vedi Track)
  pair: RotationPair | null; // null con la turnazione spenta o meno di due presenti
}
