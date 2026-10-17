import { useEffect, useState } from 'react';
import { LiveBadge } from './components/LiveBadge';
import { NewTournamentDialog } from './components/NewTournamentDialog';
import { Button, Empty, fieldSmClass } from './components/ui';
import { api } from './lib/api';
import { useConfirm } from './lib/confirm';
import { TEAMS, TRACKS } from './lib/f1';
import { href, useView, VIEW_LABELS } from './lib/route';
import { useToast } from './lib/toast';
import type { Tournament } from './lib/types';
import { DashboardPage } from './pages/DashboardPage';
import { LivePage } from './pages/LivePage';
import { PlayersPage } from './pages/PlayersPage';
import { RacesPage } from './pages/RacesPage';
import { RecordingsPage } from './pages/RecordingsPage';
import { StandingsPage } from './pages/StandingsPage';

export function App() {
  const { run } = useToast();
  const confirm = useConfirm();
  const view = useView();
  const [tournaments, setTournaments] = useState<Tournament[] | null>(null);
  const [tournamentId, setTournamentId] = useState<number | null>(
    () => Number(localStorage.getItem('tournamentId')) || null,
  );
  const [creating, setCreating] = useState(false);

  // Ricarica i tornei e seleziona selectId, oppure il primo se non esiste più.
  async function loadTournaments(selectId: number | null) {
    const list = await api.tournaments();
    setTournaments(list);
    setTournamentId(list.some((t) => t.id === selectId) ? selectId : list[0]?.id ?? null);
  }

  // Solo all'avvio: dopo, i tornei si ricaricano esplicitamente quando si creano o si eliminano.
  useEffect(() => {
    run(() => loadTournaments(tournamentId));
  }, []);

  useEffect(() => {
    localStorage.setItem('tournamentId', String(tournamentId ?? ''));
  }, [tournamentId]);

  async function handleDelete() {
    const t = tournaments?.find((x) => x.id === tournamentId);
    if (!t) return;
    const ok = await confirm({
      title: 'Eliminare il torneo?',
      message: `"${t.name}" verrà eliminato insieme a tutte le sue gare e ai risultati.`,
      confirmLabel: 'Elimina torneo',
    });
    if (!ok) return;
    run(async () => {
      await api.deleteTournament(t.id);
      await loadTournaments(null);
    });
  }

  // Classifica e gare appartengono a un torneo: senza tornei non c'è niente da mostrare.
  const missing = (view === 'standings' || view === 'races') && !tournamentId;

  return (
    <>
      <header className="flex flex-wrap items-center justify-between gap-4 border-b-3 border-accent bg-surface px-6 py-4">
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          <h1 className="text-2xl font-bold tracking-wide">
            <a href={href('dashboard')}>
              <span className="font-black text-accent italic">F1</span> 2025 Scoreboard
            </a>
          </h1>
          <LiveBadge />
        </div>
        <div className="flex items-center gap-2">
          <select
            aria-label="Torneo"
            className={fieldSmClass}
            value={tournamentId ?? ''}
            onChange={(e) => setTournamentId(Number(e.target.value))}
          >
            {tournaments?.length
              ? tournaments.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)
              : <option value="">Nessun torneo</option>}
          </select>
          <Button size="sm" onClick={() => setCreating(true)}>+ Torneo</Button>
          <Button variant="ghost" size="sm" title="Elimina torneo" disabled={!tournamentId} onClick={handleDelete}>
            Elimina
          </Button>
        </div>
      </header>

      {/* Le pagine montano solo dopo aver caricato i tornei, per non chiedere un torneo salvato che non esiste più. */}
      {tournaments && (
        <main className="mx-auto max-w-300 p-6">
          {view !== 'dashboard' && (
            <nav aria-label="Percorso" className="mb-4 flex items-center gap-2 text-sm">
              <a href={href('dashboard')} className="text-muted hover:text-fg">{VIEW_LABELS.dashboard}</a>
              <span className="text-muted">/</span>
              <span aria-current="page" className="font-semibold">{VIEW_LABELS[view]}</span>
            </nav>
          )}
          {missing ? (
            <Empty>Crea un torneo per iniziare.</Empty>
          ) : (
            <section>
              {/* key: cambiando torneo la pagina riparte da zero (es. chiude l'editor dei risultati). */}
              {view === 'dashboard' && <DashboardPage key={tournamentId} tournamentId={tournamentId} />}
              {view === 'standings' && <StandingsPage key={tournamentId} tournamentId={tournamentId!} />}
              {view === 'races' && <RacesPage key={tournamentId} tournamentId={tournamentId!} />}
              {view === 'players' && <PlayersPage />}
              {view === 'live' && <LivePage />}
              {view === 'recordings' && <RecordingsPage tournaments={tournaments} />}
            </section>
          )}
        </main>
      )}

      <NewTournamentDialog
        open={creating}
        defaultName={`Torneo F1 2025 #${(tournaments?.length ?? 0) + 1}`}
        onClose={() => setCreating(false)}
        onCreated={(t) => loadTournaments(t.id)}
      />

      <datalist id="tracks">
        {TRACKS.map((track) => <option key={track} value={track} />)}
      </datalist>
      <datalist id="teams">
        {TEAMS.map((team) => <option key={team} value={team} />)}
      </datalist>
    </>
  );
}
