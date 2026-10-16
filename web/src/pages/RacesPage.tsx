import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { formatDate, raceLabels } from '../lib/format';
import type { Player, RaceType, RaceWithResults } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { ResultsEditor } from '../components/ResultsEditor';
import { Button, Card, cx, dnfClass, Empty, fieldClass, rankClass, Table, Td, Th, Tr } from '../components/ui';

export function RacesPage({ tournamentId }: { tournamentId: number }) {
  const { run } = useToast();
  const confirm = useConfirm();
  const [editingRaceId, setEditingRaceId] = useState<number | null>(null);
  const { data, reload } = useLoad(
    () => Promise.all([api.races(tournamentId), api.players()]),
    [tournamentId],
  );

  function handleAdd(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fields = new FormData(form);
    run(async () => {
      const race = await api.createRace(tournamentId, {
        track: String(fields.get('track')),
        type: fields.get('type') as RaceType,
        date: String(fields.get('date')),
      });
      form.reset();
      setEditingRaceId(race.id);
      await reload();
    });
  }

  async function handleDelete(raceId: number) {
    const race = data?.[0].find((r) => r.id === raceId);
    const ok = await confirm({
      title: 'Eliminare la gara?',
      message: `${race?.track ?? 'La gara'} verrà eliminata insieme ai suoi risultati.`,
      confirmLabel: 'Elimina gara',
    });
    if (!ok) return;
    run(async () => {
      await api.deleteRace(raceId);
      if (editingRaceId === raceId) setEditingRaceId(null);
      await reload();
    });
  }

  return (
    <>
      <Card>
        <form className="flex flex-wrap gap-2" onSubmit={handleAdd}>
          <input name="track" list="tracks" placeholder="Circuito" required className={cx(fieldClass, 'flex-[1_1_200px]')} />
          <select name="type" className={fieldClass}>
            <option value="race">Gran Premio</option>
            <option value="sprint">Sprint</option>
          </select>
          <input name="date" type="date" className={fieldClass} />
          <Button>Aggiungi gara</Button>
        </form>
      </Card>
      {data && <RaceList
        races={data[0]}
        players={data[1]}
        editingRaceId={editingRaceId}
        onEdit={setEditingRaceId}
        onDelete={handleDelete}
        onSaved={async () => {
          setEditingRaceId(null);
          await reload();
        }}
      />}
    </>
  );
}

interface RaceListProps {
  races: RaceWithResults[];
  players: Player[];
  editingRaceId: number | null;
  onEdit: (raceId: number | null) => void;
  onDelete: (raceId: number) => void;
  onSaved: () => Promise<void>;
}

function RaceList({ races, players, editingRaceId, onEdit, onDelete, onSaved }: RaceListProps) {
  if (!races.length) return <Empty>Nessuna gara in questo torneo.</Empty>;

  const labels = raceLabels(races);
  const playerName = (id: number) => players.find((p) => p.id === id)?.name ?? '?';

  return races.map((race) => {
    const editing = editingRaceId === race.id;
    return (
      <Card key={race.id}>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h3 className="text-lg font-bold">
            <span className="text-muted">{labels.get(race.id)}</span> {race.track}
            {race.type === 'sprint' && <> <SprintBadge /></>}
            {race.date && <> <span className="text-muted">· {formatDate(race.date)}</span></>}
          </h3>
          <div className="flex gap-2">
            {!editing && (
              <Button size="sm" onClick={() => onEdit(race.id)}>
                {race.results.length ? 'Modifica risultati' : 'Inserisci risultati'}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => onDelete(race.id)}>Elimina</Button>
          </div>
        </div>
        {editing ? (
          <ResultsEditor race={race} players={players} onCancel={() => onEdit(null)} onSaved={onSaved} />
        ) : race.results.length ? (
          <Table>
            <thead><tr><Th>Pos</Th><Th>Pilota</Th><Th num>Punti</Th></tr></thead>
            <tbody>
              {race.results.map((r) => (
                <Tr key={r.player_id}>
                  <Td className={r.dnf ? dnfClass : rankClass(r.position)}>{r.dnf ? 'DNF' : r.position}</Td>
                  <Td>{playerName(r.player_id)}</Td>
                  <Td num>{r.points}</Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        ) : <p className="mt-2 text-muted">Nessun risultato inserito.</p>}
      </Card>
    );
  });
}

function SprintBadge() {
  return (
    <span className="inline-block rounded bg-sprint-bg px-2 py-0.5 align-middle text-xs font-bold text-sprint uppercase">
      Sprint
    </span>
  );
}
