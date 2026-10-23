import { LiveBadge } from './components/LiveBadge';
import { cx, Empty, Loading } from './components/ui';
import { api } from './lib/api';
import { TEAMS } from './lib/f1';
import {
  href, NAV_VIEWS, previousView, STANDALONE_VIEWS, useView, useViewParam, VIEW_LABELS, VIEW_PARENT, type View,
} from './lib/route';
import type { Tournament } from './lib/types';
import { useLoad } from './lib/useLoad';
import { DashboardPage } from './pages/DashboardPage';
import { LivePage } from './pages/LivePage';
import { OverlayPage } from './pages/OverlayPage';
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

// Torneo in cui vengono salvate le gare finite: nell'header, uguale in tutte le pagine.
function ActiveTournament({ tournaments }: { tournaments: Tournament[] | null }) {
  if (!tournaments) return null;
  const active = tournaments.find((t) => t.active);
  return (
    <a
      href={href('tournaments')}
      title="Le gare finite vengono salvate nel torneo in corso; se non ce n'è uno, in &quot;Nessun torneo&quot;"
      className="text-sm text-muted hover:text-fg"
    >
      {active ? <>Torneo in corso: <span className="font-semibold text-fg">{active.name}</span></> : 'Nessun torneo in corso'}
    </a>
  );
}

function GearIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="block size-5"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

export function App() {
  const view = useView();
  // Le pagine a schermo intero non hanno header né menu e non caricano i
  // tornei: l'overlay di OBS non deve mostrare nemmeno un messaggio d'errore.
  if (STANDALONE_VIEWS.includes(view)) return view === 'overlay' ? <OverlayPage /> : <TvPage />;
  return <MainApp view={view} />;
}

function MainApp({ view }: { view: View }) {
  const viewParam = useViewParam();
  // I tornei si ricaricano da soli quando cambiano (vedi useLoad).
  const { data: tournaments, reload: reloadTournaments } = useLoad(api.tournaments, []);
  const { data: tracks } = useLoad(api.tracks, []);
  const parent = VIEW_PARENT[view];
  const section = parent ?? view; // pagina evidenziata nella barra

  return (
    <>
      <header className="border-t-3 border-accent bg-surface">
        <div className="flex flex-wrap items-center justify-between gap-4 px-6 pt-3 pb-2">
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <h1 className="text-2xl font-bold tracking-wide">
              <a href={href('dashboard')}>
                <span className="font-black text-accent italic">F1</span> Scoreboard
              </a>
            </h1>
            <LiveBadge />
          </div>
          <div className="flex items-center gap-2">
            <ActiveTournament tournaments={tournaments} />
            <a
              href={href('system')}
              aria-label={VIEW_LABELS.system}
              title={VIEW_LABELS.system}
              aria-current={view === 'system' ? 'page' : undefined}
              className={cx(
                'rounded-md p-1.5 transition-colors hover:bg-line hover:text-fg',
                view === 'system' ? 'text-accent' : 'text-muted',
              )}
            >
              <GearIcon />
            </a>
          </div>
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

      {/* Le pagine montano solo dopo aver caricato i tornei: i filtri partono dal torneo in corso. */}
      {tournaments ? (
        <main className="mx-auto max-w-300 p-6">
          {/* Sistema non è nella barra: il titolo dice dove ci si trova. */}
          {view === 'system' && <h2 className="mb-4 text-xl font-bold">{VIEW_LABELS.system}</h2>}
          {parent && (
            <nav aria-label="Percorso" className="mb-4 flex items-center gap-2 text-sm">
              <a
                href={href(parent)}
                className="text-muted hover:text-fg"
                onClick={(e) => {
                  // Arrivando da lì si torna indietro, così la pagina riapre con i filtri com'erano.
                  if (previousView() !== parent) return;
                  e.preventDefault();
                  history.back();
                }}
              >
                {VIEW_LABELS[parent]}
              </a>
              <span className="text-muted">/</span>
              <span aria-current="page" className="font-semibold">{VIEW_LABELS[view]}</span>
            </nav>
          )}
          <section>
            {view === 'dashboard' && <DashboardPage tournaments={tournaments} />}
            {view === 'standings' && <StandingsPage tournaments={tournaments} />}
            {view === 'races' && <RacesPage tournaments={tournaments} />}
            {view === 'stats' && <StatsPage />}
            {view === 'players' && <PlayersPage />}
            {view === 'player' && (
              viewParam ? <PlayerPage key={viewParam} playerId={viewParam} /> : <Empty>Pilota non trovato.</Empty>
            )}
            {view === 'tournaments' && (
              <TournamentsPage tournaments={tournaments} onChanged={reloadTournaments} />
            )}
            {view === 'live' && <LivePage />}
            {view === 'recordings' && <RecordingsPage tournaments={tournaments} />}
            {view === 'replay' && (
              viewParam ? <ReplayPage key={viewParam} sessionId={viewParam} /> : <Empty>Registrazione non trovata.</Empty>
            )}
            {view === 'system' && <SystemPage />}
          </section>
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
