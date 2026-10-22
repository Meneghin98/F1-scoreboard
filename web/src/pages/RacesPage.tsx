import { useState, type FormEvent } from 'react';
import { api, type RaceInput } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { givesPoints, RACE_TYPES, raceTypeLabel } from '../lib/f1';
import { formatDate, formatLapTime, raceLabels } from '../lib/format';
import { href } from '../lib/route';
import type { Player, RaceType, RaceWithResults, ScoringRules, Tournament } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { ResultsEditor } from '../components/ResultsEditor';
import { Button, buttonClass, Card, cx, dnfClass, Empty, fieldClass, Loading, rankClass, Table, Td, Th, Tr } from '../components/ui';

// Cosa si sta modificando di una gara: i risultati o circuito, tipo e data.
type Editing = { raceId: number; what: 'results' | 'details' } | null;

// Le gare del torneo con i risultati, e i punti calcolati con le sue regole.
// In "Nessun torneo" (free), eliminando una gara la registrazione resta senza torneo.
export function RacesPage({ tournament }: { tournament: Tournament }) {
  const { id: tournamentId, free } = tournament;
  const { run, toast } = useToast();
  const confirm = useConfirm();
  const [editing, setEditing] = useState<Editing>(null);
  const { data, reload } = useLoad(
    () => Promise.all([api.races(tournamentId), api.players()]),
    [tournamentId],
  );

  function handleAdd(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    run(async () => {
      const race = await api.createRace(tournamentId, raceInput(form));
      form.reset();
      setEditing({ raceId: race.id, what: 'results' });
      await reload();
    });
  }

  async function handleDelete(race: RaceWithResults) {
    const recordings = race.recordings.length;
    const ok = await confirm({
      title: 'Eliminare la gara?',
      message: (
        <>
          {race.track} verrà eliminata insieme ai suoi risultati.
          {recordings > 0 && (free
            ? ` ${recordings === 1 ? 'La sua registrazione resterà' : `Le sue ${recordings} registrazioni resteranno`} senza torneo.`
            : ` ${recordings === 1 ? 'La sua registrazione passerà' : `Le sue ${recordings} registrazioni passeranno`} in "Nessun torneo".`)}
        </>
      ),
      confirmLabel: 'Elimina gara',
    });
    if (!ok) return;
    run(async () => {
      const { movedRecordings } = await api.deleteRace(race.id);
      if (editing?.raceId === race.id) setEditing(null);
      await reload();
      if (movedRecordings) toast('La registrazione della gara è passata in "Nessun torneo"');
    });
  }

  async function handleResync(race: RaceWithResults) {
    const ok = await confirm({
      title: 'Ripristinare i risultati?',
      message: 'Le modifiche fatte a mano vengono annullate: i risultati tornano quelli della registrazione della gara.',
      confirmLabel: 'Ripristina',
    });
    if (!ok) return;
    run(async () => {
      await api.resyncRace(race.id);
      await reload();
      toast('Risultati ripresi dalla registrazione');
    });
  }

  function handleSaveDetails(race: RaceWithResults, input: RaceInput) {
    run(async () => {
      await api.updateRace(race.id, input);
      setEditing(null);
      await reload();
    });
  }

  return (
    <>
      <Card>
        <form className="flex flex-wrap gap-2" onSubmit={handleAdd}>
          <RaceFields />
          <Button>Aggiungi gara</Button>
        </form>
      </Card>
      {!data ? <Loading /> : (data[0].length ? (
        <RaceList
          races={data[0]}
          players={data[1]}
          rules={tournament}
          editing={editing}
          onEdit={setEditing}
          onDelete={handleDelete}
          onResync={handleResync}
          onSaveDetails={handleSaveDetails}
          onSaved={async () => {
            setEditing(null);
            await reload();
          }}
        />
      ) : <Empty>Nessuna gara in questo torneo.</Empty>)}
    </>
  );
}

// Campi di una gara, per aggiungerla o modificarla.
function RaceFields({ race }: { race?: RaceWithResults }) {
  return (
    <>
      <input
        name="track"
        list="tracks"
        placeholder="Circuito"
        required
        defaultValue={race?.track}
        className={cx(fieldClass, 'flex-[1_1_200px]')}
      />
      <select name="type" defaultValue={race?.type} className={fieldClass}>
        {RACE_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
      </select>
      <input name="date" type="date" defaultValue={race?.date ?? ''} className={fieldClass} />
    </>
  );
}

function raceInput(form: HTMLFormElement): RaceInput {
  const fields = new FormData(form);
  return {
    track: String(fields.get('track')),
    type: fields.get('type') as RaceType,
    date: String(fields.get('date')),
  };
}

interface RaceListProps {
  races: RaceWithResults[];
  players: Player[];
  rules: ScoringRules; // regole di punteggio del torneo
  editing: Editing;
  onEdit: (editing: Editing) => void;
  onDelete: (race: RaceWithResults) => void;
  onResync: (race: RaceWithResults) => void;
  onSaveDetails: (race: RaceWithResults, input: RaceInput) => void;
  onSaved: () => Promise<void>;
}

function RaceList({ races, players, rules, editing, onEdit, onDelete, onResync, onSaveDetails, onSaved }: RaceListProps) {
  const labels = raceLabels(races);
  const playerName = (id: number) => players.find((p) => p.id === id)?.name ?? '?';

  // La più recente in cima; le etichette (G1, G2…) restano in ordine cronologico.
  return [...races].reverse().map((race) => {
    const what = editing?.raceId === race.id ? editing.what : null;
    const source = race.recordings.find((r) => r.source);
    const replay = source ?? race.recordings[0];
    return (
      <Card key={race.id}>
        {what === 'details' ? (
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              onSaveDetails(race, raceInput(e.currentTarget));
            }}
          >
            <RaceFields race={race} />
            <Button type="button" variant="ghost" onClick={() => onEdit(null)}>Annulla</Button>
            <Button>Salva</Button>
          </form>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-4">
            <h3 className="flex flex-wrap items-center gap-x-2 text-lg font-bold">
              <span className="text-muted">{labels.get(race.id)}</span> {race.track}
              {race.type !== 'race' && <TypeBadge type={race.type} rules={rules} />}
              {race.date && <span className="text-muted">· {formatDate(race.date)}</span>}
              {source && (
                <span
                  className={cx(
                    'rounded px-2 py-0.5 text-xs font-bold uppercase',
                    race.manual_results ? 'bg-surface-2 text-sprint' : 'bg-surface-2 text-green-400',
                  )}
                  title={race.manual_results
                    ? 'I risultati vengono dalla registrazione ma sono stati modificati a mano: la registrazione non li aggiorna più'
                    : 'I risultati vengono dalla registrazione della gara'}
                >
                  {race.manual_results ? 'Modificati a mano' : 'Dalla telemetria'}
                </span>
              )}
            </h3>
            <div className="flex flex-wrap gap-2">
              {replay && (
                <a href={href('replay', replay.id)} className={buttonClass('ghost', 'sm')} title="Rivedi la gara">
                  ▶ Replay
                </a>
              )}
              {source && race.manual_results && (
                <Button variant="ghost" size="sm" onClick={() => onResync(race)}>Ripristina risultati</Button>
              )}
              {!what && (
                <>
                  <Button variant="ghost" size="sm" onClick={() => onEdit({ raceId: race.id, what: 'details' })}>
                    Modifica gara
                  </Button>
                  <Button size="sm" onClick={() => onEdit({ raceId: race.id, what: 'results' })}>
                    {race.results.length ? 'Modifica risultati' : 'Inserisci risultati'}
                  </Button>
                </>
              )}
              <Button variant="ghost" size="sm" onClick={() => onDelete(race)}>Elimina</Button>
            </div>
          </div>
        )}
        {what === 'results' ? (
          <ResultsEditor
            race={race}
            players={players}
            fromRecording={!!source}
            onCancel={() => onEdit(null)}
            onSaved={onSaved}
          />
        ) : race.results.length ? (
          <ResultsTable race={race} rules={rules} playerName={playerName} />
        ) : <p className="mt-2 text-muted">Nessun risultato inserito.</p>}
      </Card>
    );
  });
}

// Risultati di una gara: posizione, pilota, giro veloce e punti. Il miglior
// giro c'è solo per le gare prese da una registrazione. Nei tornei in modalità
// "players" i punti seguono la posizione tra i piloti, mostrata accanto.
function ResultsTable({ race, rules, playerName }: { race: RaceWithResults; rules: ScoringRules; playerName: (id: number) => string }) {
  const points = givesPoints(race.type, rules);
  const playersMode = rules.points_mode === 'players';
  const laps = race.results.some((r) => r.best_lap_ms);
  return (
    <Table>
      <thead>
        <tr>
          <Th>Pos</Th>
          {points && playersMode && <Th num title="Posizione tra i soli piloti, che dà i punti">Tra i piloti</Th>}
          <Th>Pilota</Th>
          {laps && <Th num>Miglior giro</Th>}
          {points && <Th num>Punti</Th>}
        </tr>
      </thead>
      <tbody>
        {race.results.map((r) => (
          <Tr key={r.player_id}>
            <Td className={r.dnf ? dnfClass : rankClass(r.position)}>{r.dnf ? 'DNF' : r.position}</Td>
            {points && playersMode && <Td num className={rankClass(r.scored_position)}>{r.scored_position ?? '–'}</Td>}
            <Td>
              {playerName(r.player_id)}
              {r.scored_fastest_lap && (
                <span
                  className="ml-2 rounded bg-purple-950 px-1.5 py-0.5 text-xs font-bold text-purple-400"
                  title={playersMode ? 'Giro più veloce tra i piloti' : 'Giro più veloce della gara'}
                >
                  GV
                </span>
              )}
            </Td>
            {laps && <Td num className={cx(r.scored_fastest_lap && 'font-bold text-purple-400')}>{formatLapTime(r.best_lap_ms ?? 0)}</Td>}
            {points && (
              <Td num>
                {r.points}
                {r.bonus > 0 && (
                  <span className="ml-1 text-xs text-muted" title="Punti bonus, già compresi">
                    (+{r.bonus} {race.type === 'qualifying' ? 'pole' : 'GV'})
                  </span>
                )}
                {r.win_bonus > 0 && (
                  <span className="ml-1 text-xs text-muted" title="Vittoria davanti alla IA, già compresa">
                    (+{r.win_bonus} 1° assoluto)
                  </span>
                )}
              </Td>
            )}
          </Tr>
        ))}
      </tbody>
    </Table>
  );
}

// Etichetta del tipo per tutto ciò che non è un Gran Premio; in grigio le
// sessioni che non danno punti (prove libere, e qualifiche senza punti per la pole).
function TypeBadge({ type, rules }: { type: RaceType; rules: ScoringRules }) {
  const points = givesPoints(type, rules);
  return (
    <span
      className={cx(
        'inline-block rounded px-2 py-0.5 align-middle text-xs font-bold uppercase',
        points ? 'bg-sprint-bg text-sprint' : 'bg-surface-2 text-muted',
      )}
      title={points ? undefined : 'Non assegna punti'}
    >
      {raceTypeLabel(type)}
    </span>
  );
}
