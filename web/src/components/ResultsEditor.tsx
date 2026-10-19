import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import type { Player, RaceWithResults, ResultInput } from '../lib/types';
import { useToast } from '../lib/toast';
import { Button, cx, fieldClass, Table, TeamTag, Td, Th, Tr } from './ui';

interface Row {
  position: string; // valore del campo, vuoto se il pilota non ha partecipato
  dnf: boolean;
}

interface Props {
  race: RaceWithResults;
  players: Player[];
  fromRecording: boolean; // i risultati vengono da una registrazione
  onCancel: () => void;
  onSaved: () => Promise<void>;
}

export function ResultsEditor({ race, players, fromRecording, onCancel, onSaved }: Props) {
  const { run, toast } = useToast();
  const [rows, setRows] = useState<Record<number, Row>>(() => Object.fromEntries(
    players.map((p) => {
      const r = race.results.find((x) => x.player_id === p.id);
      return [p.id, { position: r && !r.dnf ? String(r.position) : '', dnf: r?.dnf ?? false }];
    }),
  ));

  if (!players.length) {
    return <p className="mt-4 text-muted">Aggiungi prima dei piloti nella scheda "Piloti".</p>;
  }

  // Spuntare DNF svuota la posizione e scrivere una posizione toglie il DNF.
  function setPosition(playerId: number, position: string) {
    setRows((prev) => ({ ...prev, [playerId]: { position, dnf: position ? false : prev[playerId].dnf } }));
  }
  function setDnf(playerId: number, dnf: boolean) {
    setRows((prev) => ({ ...prev, [playerId]: { position: dnf ? '' : prev[playerId].position, dnf } }));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const results: ResultInput[] = players
      .map((p) => ({ playerId: p.id, ...rows[p.id] }))
      .filter((r) => r.position !== '' || r.dnf)
      .map((r) => ({ player_id: r.playerId, position: r.dnf ? null : Number(r.position), dnf: r.dnf }));

    run(async () => {
      await api.saveResults(race.id, results);
      toast('Risultati salvati');
      await onSaved();
    });
  }

  return (
    <form className="mt-4" onSubmit={handleSubmit}>
      {fromRecording && (
        <p className="mb-2 rounded-md border border-sprint/40 bg-sprint-bg/40 px-3 py-2 text-sm">
          I risultati vengono dalla registrazione della gara. Se li modifichi restano questi: cambiare i piloti
          della registrazione non li aggiornerà più, finché non li ripristini.
        </p>
      )}
      <p className="mb-2 text-muted">Inserisci la posizione d'arrivo; lascia vuoto chi non ha partecipato.</p>
      <Table>
        <thead><tr><Th>Pilota</Th><Th>Posizione</Th><Th>DNF</Th></tr></thead>
        <tbody>
          {players.map((p) => (
            <Tr key={p.id}>
              <Td>{p.name} <TeamTag team={p.team} /></Td>
              <Td>
                <input
                  type="number" min="1" step="1" name="position"
                  className={cx(fieldClass, 'w-20')}
                  value={rows[p.id].position}
                  onChange={(e) => setPosition(p.id, e.target.value)}
                />
              </Td>
              <Td>
                <input
                  type="checkbox" name="dnf"
                  className="size-4 accent-accent"
                  checked={rows[p.id].dnf}
                  onChange={(e) => setDnf(p.id, e.target.checked)}
                />
              </Td>
            </Tr>
          ))}
        </tbody>
      </Table>
      <div className="mt-3 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onCancel}>Annulla</Button>
        <Button>Salva risultati</Button>
      </div>
    </form>
  );
}
