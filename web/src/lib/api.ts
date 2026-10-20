import type {
  BackupInfo, Player, PlayerProfile, Race, RaceType, RaceWithResults, ResultInput, ScoringRules, SessionRaceInput,
  SessionRecord, Settings, SettingsSaved, StandingsResponse, StatsResponse, SystemInfo, Tournament,
  ReplayFrame, ReplayInfo, Track, TrackMap,
} from './types';

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Errore ${res.status}`);
  return data as T;
}

// Eliminando una gara o un torneo le sue registrazioni passano in "Nessun torneo".
interface Deleted {
  movedRecordings: number;
}

export interface RaceInput {
  track: string;
  type: RaceType;
  date: string;
}

export const api = {
  tracks: () => request<Track[]>('/tracks'),

  tournaments: () => request<Tournament[]>('/tournaments'),
  createTournament: (name: string, start: boolean, rules: ScoringRules) =>
    request<Tournament>('/tournaments', 'POST', { name, start, ...rules }),
  updateTournament: (id: number, name: string, rules: ScoringRules) =>
    request<Tournament>(`/tournaments/${id}`, 'PUT', { name, ...rules }),
  deleteTournament: (id: number) => request<Deleted>(`/tournaments/${id}`, 'DELETE'),
  startTournament: (id: number) => request<Tournament>(`/tournaments/${id}/start`, 'POST'),
  endTournament: (id: number) => request<Tournament>(`/tournaments/${id}/end`, 'POST'),

  players: () => request<Player[]>('/players'),
  createPlayer: (name: string, team: string) => request<Player>('/players', 'POST', { name, team }),
  updatePlayer: (id: number, name: string, team: string, color: string) =>
    request<Player>(`/players/${id}`, 'PUT', { name, team, color }),
  deletePlayer: (id: number) => request<void>(`/players/${id}`, 'DELETE'),

  races: (tournamentId: number) => request<RaceWithResults[]>(`/tournaments/${tournamentId}/races`),
  createRace: (tournamentId: number, race: RaceInput) => request<Race>(`/tournaments/${tournamentId}/races`, 'POST', race),
  updateRace: (id: number, race: RaceInput) => request<Race>(`/races/${id}`, 'PUT', race),
  deleteRace: (id: number) => request<Deleted>(`/races/${id}`, 'DELETE'),
  saveResults: (raceId: number, results: ResultInput[]) =>
    request<void>(`/races/${raceId}/results`, 'PUT', { results }),
  resyncRace: (raceId: number) => request<void>(`/races/${raceId}/resync`, 'POST'),

  standings: (tournamentId: number) => request<StandingsResponse>(`/tournaments/${tournamentId}/standings`),
  stats: () => request<StatsResponse>('/stats'),
  playerStats: (id: number) => request<PlayerProfile>(`/players/${id}/stats`),

  liveTrack: () => request<TrackMap>('/live/track'),
  setLineup: (players: { carIndex: number; playerId: number | null }[], confirm: boolean) =>
    request<void>('/live/lineup', 'PUT', { players, confirm }),

  sessions: () => request<SessionRecord[]>('/telemetry/sessions'),
  deleteSession: (id: number) => request<void>(`/telemetry/sessions/${id}`, 'DELETE'),
  setSessionPlayers: (id: number, players: { carIndex: number; playerId: number | null }[]) =>
    request<void>(`/telemetry/sessions/${id}/players`, 'PUT', { players }),
  setSessionRace: (id: number, race: SessionRaceInput) =>
    request<void>(`/telemetry/sessions/${id}/race`, 'PUT', race),

  replay: (sessionId: number) => request<ReplayInfo>(`/telemetry/sessions/${sessionId}/replay`),
  replayFrame: (sessionId: number, ms: number) =>
    request<ReplayFrame>(`/telemetry/sessions/${sessionId}/replay/frame?t=${Math.round(ms)}`),
  replayTrack: (sessionId: number) => request<TrackMap>(`/telemetry/sessions/${sessionId}/replay/track`),

  system: () => request<SystemInfo>('/system'),
  backups: () => request<BackupInfo[]>('/backups'),
  settings: () => request<Settings>('/settings'),
  saveSettings: (settings: Settings) => request<SettingsSaved>('/settings', 'PUT', settings),
};

// Indirizzo per scaricare il file di una registrazione.
export const sessionDownloadUrl = (id: number) => `/api/telemetry/sessions/${id}/download`;

// Indirizzi per scaricare un backup del database fatto al momento, o uno di quelli automatici.
export const backupUrl = '/api/backup';
export const autoBackupUrl = (name: string) => `/api/backups/${encodeURIComponent(name)}`;
