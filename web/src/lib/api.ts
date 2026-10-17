import type {
  Player, Race, RaceType, RaceWithResults, ResultInput, SessionRaceInput, SessionRecord, StandingsResponse, Tournament,
  TrackMap,
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

export const api = {
  tournaments: () => request<Tournament[]>('/tournaments'),
  createTournament: (name: string) => request<Tournament>('/tournaments', 'POST', { name }),
  deleteTournament: (id: number) => request<void>(`/tournaments/${id}`, 'DELETE'),

  players: () => request<Player[]>('/players'),
  createPlayer: (name: string, team: string) => request<Player>('/players', 'POST', { name, team }),
  deletePlayer: (id: number) => request<void>(`/players/${id}`, 'DELETE'),

  races: (tournamentId: number) => request<RaceWithResults[]>(`/tournaments/${tournamentId}/races`),
  createRace: (tournamentId: number, race: { track: string; type: RaceType; date: string }) =>
    request<Race>(`/tournaments/${tournamentId}/races`, 'POST', race),
  deleteRace: (id: number) => request<void>(`/races/${id}`, 'DELETE'),
  saveResults: (raceId: number, results: ResultInput[]) =>
    request<void>(`/races/${raceId}/results`, 'PUT', { results }),

  standings: (tournamentId: number) => request<StandingsResponse>(`/tournaments/${tournamentId}/standings`),

  liveTrack: () => request<TrackMap>('/live/track'),

  sessions: () => request<SessionRecord[]>('/telemetry/sessions'),
  deleteSession: (id: number) => request<void>(`/telemetry/sessions/${id}`, 'DELETE'),
  setSessionPlayers: (id: number, players: { carIndex: number; playerId: number | null }[]) =>
    request<void>(`/telemetry/sessions/${id}/players`, 'PUT', { players }),
  setSessionRace: (id: number, race: SessionRaceInput) =>
    request<void>(`/telemetry/sessions/${id}/race`, 'PUT', race),
};

// Indirizzo per scaricare il file di una registrazione.
export const sessionDownloadUrl = (id: number) => `/api/telemetry/sessions/${id}/download`;
