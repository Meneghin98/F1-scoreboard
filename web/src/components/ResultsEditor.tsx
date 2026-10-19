import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { countsForPoints } from '../lib/f1';
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
  // Chi ha fatto il giro più veloce (uno solo), null se nessuno dei piloti.
  const [fastest, setFastest] = useState<number | null>(() => race.results.find((r) => r.fastest_lap)?.player_id ?? null);
  // Il giro veloce conta nelle gare che danno punti.
  const withFastest = countsForPoints(race.type);

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
      .map((r) => ({
        player_id: r.playerId,
        position: r.dnf ? null : Number(r.position),
        dnf: r.dnf,
        fastest_lap: withFastest && fastest === r.playerId,
      }));

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
      <p className="mb-2 text-muted">
        Inserisci la posizione d'arrivo; lascia vuoto chi non ha partecipato.
        {withFastest && ' Segna chi ha fatto il giro più veloce, se è uno dei piloti.'}
      </p>
      <Table>
        <thead>
          <tr>
            <Th>Pilota</Th><Th>Posizione</Th><Th>DNF</Th>
            {withFastest && <Th title="Giro più veloce della gara">Giro veloce</Th>}
          </tr>
        </thead>
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
              {withFastest && (
                <Td>
                  {/* Un secondo clic sullo stesso pilota toglie il giro veloce. */}
                  <input
                    type="radio" name="fastest"
                    className="size-4 accent-purple-500"
                    checked={fastest === p.id}
                    onChange={() => setFastest(p.id)}
                    onClick={() => fastest === p.id && setFastest(null)}
                  />
                </Td>
              )}
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
