import { useEffect, useState } from 'react';
import { LiveBadge } from './components/LiveBadge';
import { cx, Empty, fieldSmClass, Loading } from './components/ui';
import { api } from './lib/api';
import { TEAMS } from './lib/f1';
import { href, NAV_VIEWS, STANDALONE_VIEWS, useView, useViewParam, VIEW_LABELS, VIEW_PARENT } from './lib/route';
import type { Tournament } from './lib/types';
import { useLoad } from './lib/useLoad';
import { DashboardPage } from './pages/DashboardPage';
import { LivePage } from './pages/LivePage';
import { PlayerPage } from './pages/PlayerPage';
import { PlayersPage } from './pages/PlayersPage';
import { RacesPage } from './pages/RacesPage';
import { RecordingsPage } from './pages/RecordingsPage';
import { ReplayPage } from './pages/ReplayPage';
import { StandingsPage } from './pages/StandingsPage';
import { StatsPage } from './pages/StatsPage';
import { SystemPage } from './pages/SystemPage';
import { TournamentsPage } from './pages/TournamentsPage';
import { TvPage } from './pages/TvPage';

function tournamentLabel(t: Tournament): string {
  if (t.active) return `${t.name} (in corso)`;
  if (t.ended_at) return `${t.name} (terminato)`;
  return t.name;
}

export function App() {
  const view = useView();
  const viewParam = useViewParam();
  // I tornei si ricaricano da soli quando cambiano (vedi useLoad).
  const { data: tournaments, reload: reloadTournaments } = useLoad(api.tournaments, []);
  const { data: tracks } = useLoad(api.tracks, []);
  const [selectedId, setSelectedId] = useState<number | null>(
    () => Number(localStorage.getItem('tournamentId')) || null,
  );

  // Il torneo scelto in alto o, se non esiste più, il primo dell'elenco (quello in corso).
  const tournamentId = tournaments?.some((t) => t.id === selectedId) ? selectedId : tournaments?.[0]?.id ?? null;
  const current = tournaments?.find((t) => t.id === tournamentId);
  useEffect(() => {
    if (tournamentId !== null) localStorage.setItem('tournamentId', String(tournamentId));
  }, [tournamentId]);

  if (STANDALONE_VIEWS.includes(view)) return <TvPage />;

  // Classifica e gare appartengono a un torneo: senza tornei non c'è niente da mostrare.
  const missing = (view === 'standings' || view === 'races') && !tournamentId;
  const parent = VIEW_PARENT[view];
  const section = parent ?? view; // pagina evidenziata nella barra

  return (
    <>
      <header className="border-t-3 border-accent bg-surface">
        <div className="flex flex-wrap items-center justify-between gap-4 px-6 pt-3 pb-2">
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <h1 className="text-2xl font-bold tracking-wide">
              <a href={href('dashboard')}>
                <span className="font-black text-accent italic">F1</span> 2025 Scoreboard
              </a>
            </h1>
            <LiveBadge />
          </div>
          <select
            aria-label="Torneo"
            title="Torneo mostrato in dashboard, classifica e gare"
            className={fieldSmClass}
            value={tournamentId ?? ''}
            onChange={(e) => setSelectedId(Number(e.target.value))}
          >
            {tournaments?.length
              ? tournaments.map((t) => <option key={t.id} value={t.id}>{tournamentLabel(t)}</option>)
              : <option value="">Nessun torneo</option>}
          </select>
        </div>
        <nav aria-label="Pagine" className="flex flex-wrap gap-1 border-b border-line px-4">
          {NAV_VIEWS.map((v) => (
            <a
              key={v}
              href={href(v)}
              aria-current={section === v ? 'page' : undefined}
              className={cx(
                '-mb-px border-b-2 px-3 pt-1.5 pb-2.5 text-sm font-semibold transition-colors',
                section === v ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg',
              )}
            >
              {VIEW_LABELS[v]}
            </a>
          ))}
        </nav>
      </header>

      {/* Le pagine montano solo dopo aver caricato i tornei, per non chiedere un torneo salvato che non esiste più. */}
      {tournaments ? (
        <main className="mx-auto max-w-300 p-6">
          {parent && (
            <nav aria-label="Percorso" className="mb-4 flex items-center gap-2 text-sm">
              <a href={href(parent)} className="text-muted hover:text-fg">{VIEW_LABELS[parent]}</a>
              <span className="text-muted">/</span>
              <span aria-current="page" className="font-semibold">{VIEW_LABELS[view]}</span>
            </nav>
          )}
          {missing ? (
            <Empty>Crea un torneo nella pagina <a href={href('tournaments')} className="underline">Tornei</a> per iniziare.</Empty>
          ) : (
            <section>
              {/* key: cambiando torneo la pagina riparte da zero (es. chiude l'editor dei risultati). */}
              {view === 'dashboard' && <DashboardPage key={tournamentId} tournamentId={tournamentId} />}
              {view === 'standings' && <StandingsPage key={tournamentId} tournamentId={tournamentId!} />}
              {view === 'races' && current && <RacesPage key={tournamentId} tournament={current} />}
              {view === 'stats' && <StatsPage />}
              {view === 'players' && <PlayersPage />}
              {view === 'player' && (
                viewParam ? <PlayerPage key={viewParam} playerId={viewParam} /> : <Empty>Pilota non trovato.</Empty>
              )}
              {view === 'tournaments' && (
                <TournamentsPage
                  tournaments={tournaments}
                  selectedId={tournamentId}
                  onSelect={setSelectedId}
                  onChanged={reloadTournaments}
                />
              )}
              {view === 'live' && <LivePage />}
              {view === 'recordings' && <RecordingsPage tournaments={tournaments} />}
              {view === 'replay' && (
                viewParam ? <ReplayPage key={viewParam} sessionId={viewParam} /> : <Empty>Registrazione non trovata.</Empty>
              )}
              {view === 'system' && <SystemPage />}
            </section>
          )}
        </main>
      ) : (
        <Loading />
      )}

      <datalist id="tracks">
        {tracks?.map((track) => <option key={track.id} value={track.name} />)}
      </datalist>
      <datalist id="teams">
        {TEAMS.map((team) => <option key={team} value={team} />)}
      </datalist>
    </>
  );
}
