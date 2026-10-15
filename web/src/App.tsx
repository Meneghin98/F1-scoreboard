import { useEffect, useState } from 'react';
import { PlayersTab } from './components/PlayersTab';
import { RacesTab } from './components/RacesTab';
import { StandingsTab } from './components/StandingsTab';
import { Button, cx, Empty, fieldSmClass } from './components/ui';
import { api } from './lib/api';
import { TEAMS, TRACKS } from './lib/f1';
import { useToast } from './lib/toast';
import type { Tournament } from './lib/types';

type Tab = 'standings' | 'races' | 'players';

const TABS: { id: Tab; label: string }[] = [
  { id: 'standings', label: 'Classifica' },
  { id: 'races', label: 'Gare' },
  { id: 'players', label: 'Piloti' },
];

export function App() {
  const { run } = useToast();
  const [tournaments, setTournaments] = useState<Tournament[] | null>(null);
  const [tournamentId, setTournamentId] = useState<number | null>(
    () => Number(localStorage.getItem('tournamentId')) || null,
  );
  const [tab, setTab] = useState<Tab>('standings');

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

  function handleNew() {
    const name = prompt('Nome del torneo:', `Torneo F1 2025 #${(tournaments?.length ?? 0) + 1}`);
    if (!name?.trim()) return;
    run(async () => {
      const t = await api.createTournament(name);
      await loadTournaments(t.id);
    });
  }

  function handleDelete() {
    const t = tournaments?.find((x) => x.id === tournamentId);
    if (!t || !confirm(`Eliminare il torneo "${t.name}" con tutte le sue gare?`)) return;
    run(async () => {
      await api.deleteTournament(t.id);
      await loadTournaments(null);
    });
  }

  const missing = tab !== 'players' && !tournamentId;

  return (
    <>
      <header className="flex flex-wrap items-center justify-between gap-4 border-b-3 border-accent bg-surface px-6 py-4">
        <h1 className="text-2xl font-bold tracking-wide">
          <span className="font-black text-accent italic">F1</span> 2025 Scoreboard
        </h1>
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
          <Button size="sm" onClick={handleNew}>+ Torneo</Button>
          <Button variant="ghost" size="sm" title="Elimina torneo" disabled={!tournamentId} onClick={handleDelete}>
            Elimina
          </Button>
        </div>
      </header>

      <nav className="flex gap-1 border-b border-line bg-surface px-6">
        {TABS.map((t) => (
          <button
            key={t.id}
            className={cx(
              'cursor-pointer border-b-3 px-4 py-3 font-semibold',
              tab === t.id ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg',
            )}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {/* I tab montano solo dopo aver caricato i tornei, per non chiedere un torneo salvato che non esiste più. */}
      {tournaments && (
        <main className="mx-auto max-w-300 p-6">
          {missing ? (
            <Empty>Crea un torneo per iniziare.</Empty>
          ) : (
            <section>
              {/* key: cambiando torneo il tab riparte da zero (es. chiude l'editor dei risultati). */}
              {tab === 'standings' && <StandingsTab key={tournamentId} tournamentId={tournamentId!} />}
              {tab === 'races' && <RacesTab key={tournamentId} tournamentId={tournamentId!} />}
              {tab === 'players' && <PlayersTab />}
            </section>
          )}
        </main>
      )}

      <datalist id="tracks">
        {TRACKS.map((track) => <option key={track} value={track} />)}
      </datalist>
      <datalist id="teams">
        {TEAMS.map((team) => <option key={team} value={team} />)}
      </datalist>
    </>
  );
}
