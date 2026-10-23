import { useState } from 'react';
import { api, sessionDownloadUrl } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { raceTypeLabel } from '../lib/f1';
import { formatBytes, formatDate, formatDateTime, formatDuration } from '../lib/format';
import { href, setFilters, useFilter } from '../lib/route';
import { useToast } from '../lib/toast';
import type { Player, SessionPlayer, SessionRecord, Tournament } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { AssignRaceDialog } from '../components/AssignRaceDialog';
import { LiveDot } from '../components/LiveBadge';
import { ALL_TOURNAMENTS, TournamentSelect, UNASSIGNED, useTournamentFilter } from '../components/TournamentFilter';
import {
  Button, buttonClass, Card, cx, dnfClass, Empty, fieldSmClass, Filter, Loading, PageToolbar, rankClass, Table, Td, Th, Tr,
} from '../components/ui';

// Voce "tutti" dei filtri pilota e circuito: nell'indirizzo non c'è.
const ALL = 'all';

// I filtri stanno nell'indirizzo (#/recordings?torneo=tutti&pilota=2&circuito=Monza),
// così tornando dal replay la pagina riapre com'era. Il torneo parte da quello in corso.
// Se tournaments cambia (torneo creato o eliminato) si ricaricano anche le registrazioni.
export function RecordingsPage({ tournaments }: { tournaments: Tournament[] }) {
  const { run, toast } = useToast();
  const confirm = useConfirm();
  const { data, reload } = useLoad(
    () => Promise.all([api.sessions(), api.players()]),
    [tournaments],
  );
  const [tournamentFilter, setTournamentFilter] = useTournamentFilter(tournaments, [ALL_TOURNAMENTS, UNASSIGNED]);
  const tournamentChosen = useFilter('torneo') !== null; // diverso da quello di partenza
  const playerParam = useFilter('pilota');
  const trackParam = useFilter('circuito');
  const [showUnfinished, setShowUnfinished] = useState(false);
  const [assigning, setAssigning] = useState<SessionRecord | null>(null);

  if (!data) return <Loading />;
  const [sessions, players] = data;

  // Circuiti presenti nelle registrazioni, in ordine alfabetico.
  const tracks = [...new Set(sessions.map((s) => s.track).filter(Boolean))].sort();
  // Un pilota o un circuito che non c'è (es. da un vecchio link) vale come "tutti".
  const playerFilter = players.some((p) => String(p.id) === playerParam) ? Number(playerParam) : null;
  const trackFilter = trackParam !== null && tracks.includes(trackParam) ? trackParam : null;

  const unfinishedCount = sessions.filter(isUnfinished).length;
  const matching = sessions.filter((s) =>
    (tournamentFilter === ALL_TOURNAMENTS ||
      (tournamentFilter === UNASSIGNED ? !s.race : s.race?.tournamentId === Number(tournamentFilter))) &&
    (playerFilter === null || s.players.some((p) => p.playerId === playerFilter)) &&
    (trackFilter === null || s.track === trackFilter),
  );
  const visible = showUnfinished ? matching : matching.filter((s) => !isUnfinished(s));
  const filtered = tournamentChosen || playerFilter !== null || trackFilter !== null;

  async function handleDelete(session: SessionRecord) {
    const ok = await confirm({
      title: 'Eliminare la registrazione?',
      message: (
        <>
          {sessionTitle(session)} del {formatDateTime(session.startedAt)} verrà eliminata insieme al file.
          {session.race && ' I risultati della gara nel torneo restano invariati.'}
        </>
      ),
      confirmLabel: 'Elimina registrazione',
    });
    if (!ok) return;
    run(async () => {
      await api.deleteSession(session.id);
      await reload();
    });
  }

  function handlePlayerChange(session: SessionRecord, carIndex: number, playerId: number | null) {
    run(async () => {
      await api.setSessionPlayers(session.id, [{ carIndex, playerId }]);
      await reload();
      if (session.race && session.providesResults) {
        toast(session.race.manualResults
          ? 'I risultati della gara sono stati modificati a mano e non cambiano: ripristinali dalla pagina Gare'
          : 'Risultati della gara e classifica aggiornati');
      }
    });
  }

  return (
    <>
      <PageToolbar
        actions={
          <span className="text-sm text-muted">
            {visible.length} {visible.length === 1 ? 'registrazione' : 'registrazioni'}
            {visible.length !== sessions.length && ` su ${sessions.length}`}
          </span>
        }
      >
        <TournamentSelect tournaments={tournaments} value={tournamentFilter} onChange={setTournamentFilter}>
          <option value={ALL_TOURNAMENTS}>Tutti i tornei</option>
          <option value={UNASSIGNED} title="Prove a tempo e registrazioni delle gare eliminate da &quot;Nessun torneo&quot;">
            Non assegnate
          </option>
        </TournamentSelect>
        <Filter label="Pilota">
          <select
            className={fieldSmClass}
            value={playerFilter ?? ALL}
            onChange={(e) => setFilters({ pilota: e.target.value === ALL ? null : e.target.value })}
          >
            <option value={ALL}>Tutti i piloti</option>
            {players.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </Filter>
        <Filter label="Circuito">
          <select
            className={fieldSmClass}
            value={trackFilter ?? ALL}
            onChange={(e) => setFilters({ circuito: e.target.value === ALL ? null : e.target.value })}
          >
            <option value={ALL}>Tutti i circuiti</option>
            {tracks.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </Filter>
        {unfinishedCount > 0 && (
          <Button
            variant="ghost"
            size="sm"
            aria-pressed={showUnfinished}
            title="Sessioni finite senza classifica finale: vengono eliminate 48 ore dopo la fine, se non sono assegnate a un torneo"
            className={cx(showUnfinished && 'border-muted text-fg')}
            onClick={() => setShowUnfinished(!showUnfinished)}
          >
            {showUnfinished ? 'Nascondi' : 'Mostra'} non terminate ({unfinishedCount})
          </Button>
        )}
        {filtered && (
          <Button variant="ghost" size="sm" onClick={() => setFilters({ torneo: null, pilota: null, circuito: null })}>
            Azzera filtri
          </Button>
        )}
      </PageToolbar>

      {!sessions.length ? (
        <Empty>Nessuna registrazione: le sessioni vengono registrate da sole quando il gioco invia la telemetria.</Empty>
      ) : !visible.length ? (
        <Empty>
          {matching.length
            ? 'Ci sono solo sessioni non terminate: usa "Mostra non terminate" per vederle.'
            : 'Nessuna registrazione corrisponde ai filtri.'}
        </Empty>
      ) : visible.map((s) => (
        <RecordingCard
          key={s.id}
          session={s}
          players={players}
          onAssign={() => setAssigning(s)}
          onDelete={() => handleDelete(s)}
          onPlayerChange={(carIndex, playerId) => handlePlayerChange(s, carIndex, playerId)}
        />
      ))}

      <AssignRaceDialog
        session={assigning}
        tournaments={tournaments}
        onClose={() => setAssigning(null)}
        onSaved={reload}
      />
    </>
  );
}

// Sessione finita senza classifica finale (abbandonata o interrotta): nascosta di default.
function isUnfinished(s: SessionRecord): boolean {
  return !s.hasClassification && !s.recording;
}

// Nome leggibile della registrazione: circuito e tipo di sessione.
function sessionTitle(s: SessionRecord): string {
  return [s.track || 'Circuito sconosciuto', s.type].filter(Boolean).join(' – ');
}

interface RecordingCardProps {
  session: SessionRecord;
  players: Player[];
  onAssign: () => void;
  onDelete: () => void;
  onPlayerChange: (carIndex: number, playerId: number | null) => void;
}

function RecordingCard({ session: s, players, onAssign, onDelete, onPlayerChange }: RecordingCardProps) {
  const humans = s.players.length;
  const details = [
    formatDateTime(s.startedAt),
    s.totalLaps ? `${s.totalLaps} giri` : null,
    s.endedAt ? formatDuration(s.startedAt, s.endedAt) : null,
    s.cars ? `${s.cars} auto` : null,
    s.analyzed ? `${humans} ${humans === 1 ? 'giocatore' : 'giocatori'}` : null,
    s.downloadable ? formatBytes(s.sizeBytes) : null,
  ].filter(Boolean);

  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h3 className="flex items-center gap-2 text-lg font-bold">
            {s.recording && <LiveDot active />}
            {sessionTitle(s)}
            {s.recording && <span className="text-sm font-semibold text-green-400">in registrazione</span>}
            {isUnfinished(s) && <span className="text-sm font-semibold text-muted">non terminata</span>}
          </h3>
          <p className="text-sm text-muted">{details.join(' · ')}</p>
          <p className="mt-1 text-sm">
            {s.race ? (
              <>
                <span className="font-semibold">{s.race.tournamentName}</span>
                <span className="text-muted">
                  {' · '}{s.race.track}{s.race.type !== 'race' && ` (${raceTypeLabel(s.race.type)})`}
                  {s.race.date && ` · ${formatDate(s.race.date)}`}
                  {s.providesResults && (s.race.manualResults
                    ? ' · risultati presi da questa registrazione e modificati a mano'
                    : ' · risultati presi da questa registrazione')}
                </span>
              </>
            ) : (
              <span className="text-muted">Non assegnata</span>
            )}
          </p>
          {s.deleteAt && (
            <p className="mt-1 text-sm text-accent-text">
              Senza classifica finale: verrà eliminata {formatDateTime(s.deleteAt)}, a meno che non la assegni a un torneo.
            </p>
          )}
        </div>
        <div className="flex gap-2">
          {s.downloadable && !s.recording ? (
            <a href={href('replay', s.id)} className={buttonClass('ghost', 'sm')} title="Rivedi la sessione come nella pagina Live">
              ▶ Replay
            </a>
          ) : (
            <Button variant="ghost" size="sm" disabled title={s.recording ? 'Registrazione in corso' : 'File non disponibile'}>
              ▶ Replay
            </Button>
          )}
          <Button size="sm" onClick={onAssign}>{s.race ? 'Cambia torneo' : 'Assegna a torneo'}</Button>
          {s.downloadable && !s.recording ? (
            <a href={sessionDownloadUrl(s.id)} download className={buttonClass('ghost', 'sm')}>Scarica</a>
          ) : (
            <Button variant="ghost" size="sm" disabled title={s.recording ? 'Registrazione in corso' : 'File non disponibile'}>
              Scarica
            </Button>
          )}
          <Button variant="ghost" size="sm" disabled={s.recording} onClick={onDelete}>Elimina</Button>
        </div>
      </div>

      {s.recording ? (
        <p className="mt-3 text-sm text-muted">I giocatori compaiono quando la sessione finisce.</p>
      ) : !s.analyzed ? (
        <p className="mt-3 text-sm text-muted">Lettura dei giocatori dalla registrazione in corso…</p>
      ) : !humans ? (
        <p className="mt-3 text-sm text-muted">Nessun giocatore umano trovato nella registrazione.</p>
      ) : (
        <SessionPlayers session={s} players={players} onChange={onPlayerChange} />
      )}
    </Card>
  );
}

interface SessionPlayersProps {
  session: SessionRecord;
  players: Player[];
  onChange: (carIndex: number, playerId: number | null) => void;
}

// Giocatori della sessione, con il pilota del segnapunti che guidava ciascuna auto.
function SessionPlayers({ session, players, onChange }: SessionPlayersProps) {
  const hasResults = session.hasClassification;
  return (
    <>
      <Table>
        <thead>
          <tr>
            {hasResults && <Th>Pos</Th>}
            <Th>Nel gioco</Th>
            <Th className="text-right">Pilota</Th>
          </tr>
        </thead>
        <tbody>
          {session.players.map((p) => (
            <Tr key={p.carIndex}>
              {hasResults && <Td className={p.dnf ? dnfClass : rankClass(p.position)}>{positionLabel(p)}</Td>}
              <Td>
                {p.name} <span className="text-sm text-muted">{p.team} #{p.raceNumber}</span>
              </Td>
              <Td className="text-right">
                <select
                  aria-label={`Pilota di ${p.name}`}
                  className={cx(fieldSmClass, 'min-w-40', !p.playerId && 'text-muted')}
                  value={p.playerId ?? ''}
                  onChange={(e) => onChange(p.carIndex, e.target.value ? Number(e.target.value) : null)}
                >
                  <option value="">Nessun pilota</option>
                  {players.map((pl) => <option key={pl.id} value={pl.id}>{pl.name}</option>)}
                </select>
              </Td>
            </Tr>
          ))}
        </tbody>
      </Table>
      {!players.length && (
        <p className="mt-2 text-sm text-muted">
          Per associare i giocatori <a href={href('players')} className="underline hover:text-fg">aggiungi i piloti</a>.
        </p>
      )}
    </>
  );
}

function positionLabel(p: SessionPlayer): string {
  if (p.dnf) return 'DNF';
  return p.position ? String(p.position) : '–';
}
