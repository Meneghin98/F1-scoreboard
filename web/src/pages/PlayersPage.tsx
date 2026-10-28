import { useState, type FormEvent, type ReactNode } from 'react';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { href } from '../lib/route';
import type { Player, RotationPair, RotationState } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { Button, Card, ColorDot, cx, Empty, fieldClass, Loading, TeamTag } from '../components/ui';

export function PlayersPage() {
  const { run } = useToast();
  const confirm = useConfirm();
  const { data: players, reload } = useLoad(api.players, []);
  const { data: rotation, reload: reloadRotation } = useLoad(api.rotation, []);
  const [editingId, setEditingId] = useState<number | null>(null);

  function handleAdd(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fields = new FormData(form);
    run(async () => {
      await api.createPlayer(String(fields.get('name')), String(fields.get('team')));
      form.reset();
      form.querySelector<HTMLInputElement>('[name=name]')?.focus();
      await reload();
    });
  }

  function handleSave(player: Player, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fields = new FormData(e.currentTarget);
    run(async () => {
      await api.updatePlayer(player.id, String(fields.get('name')), String(fields.get('team')), String(fields.get('color')));
      setEditingId(null);
      await reload();
    });
  }

  async function handleDelete(player: Player) {
    const ok = await confirm({
      title: 'Eliminare il pilota?',
      message: `${player.name} verrà rimosso dall'elenco dei piloti.`,
      confirmLabel: 'Elimina pilota',
    });
    if (!ok) return;
    run(async () => {
      await api.deletePlayer(player.id);
      await reload();
    });
  }

  function togglePresent(player: Player) {
    if (!rotation) return;
    const present = rotation.present.includes(player.id)
      ? rotation.present.filter((id) => id !== player.id)
      : [...rotation.present, player.id];
    run(async () => {
      await api.setPresent(present);
      await reloadRotation();
    });
  }

  function handleDraw() {
    run(async () => {
      await api.drawRotation();
      await reloadRotation();
    });
  }

  if (!players) return <Loading />;
  return (
    <>
      {rotation && <RotationCard rotation={rotation} players={players} onDraw={handleDraw} />}
      <Card>
        {/* Il colore lo sceglie il server (il primo libero): si cambia modificando il pilota. */}
        <form className="flex flex-wrap gap-2" onSubmit={handleAdd}>
          <PlayerFields />
          <Button>Aggiungi pilota</Button>
        </form>
      </Card>
      {players.length ? players.map((p) => (
        <Card key={p.id}>
          {editingId === p.id ? (
            <form className="flex flex-wrap gap-2" onSubmit={(e) => handleSave(p, e)}>
              <input
                name="color"
                type="color"
                defaultValue={p.color || '#9a9aae'}
                title="Colore del pilota nei grafici e nella pagina Live"
                className="h-10.5 w-12 cursor-pointer rounded-md border border-line bg-surface-2 p-1"
              />
              <PlayerFields player={p} />
              <Button type="button" variant="ghost" onClick={() => setEditingId(null)}>Annulla</Button>
              <Button>Salva</Button>
            </form>
          ) : (
            <div className="flex items-center justify-between gap-2">
              <a href={href('player', p.id)} className="flex items-center gap-2 hover:underline" title="Apri il profilo">
                <ColorDot color={p.color} className="size-3" />
                {p.name} <TeamTag team={p.team} />
              </a>
              <div className="flex items-center gap-2">
                {rotation && (
                  <label className="mr-2 flex cursor-pointer items-center gap-2 text-sm" title="Chi è presente entra nella turnazione dei controller">
                    <input
                      type="checkbox"
                      checked={rotation.present.includes(p.id)}
                      onChange={() => togglePresent(p)}
                      className="size-4 accent-accent"
                    />
                    Presente
                  </label>
                )}
                <Button variant="ghost" size="sm" onClick={() => setEditingId(p.id)}>Modifica</Button>
                <Button variant="ghost" size="sm" onClick={() => handleDelete(p)}>Elimina</Button>
              </div>
            </div>
          )}
        </Card>
      )) : <Empty>Nessun pilota registrato.</Empty>}
    </>
  );
}

// Nome e scuderia di un pilota, per aggiungerlo o modificarlo.
function PlayerFields({ player }: { player?: Player }) {
  return (
    <>
      <input
        name="name"
        placeholder="Nome pilota"
        required
        defaultValue={player?.name}
        className={cx(fieldClass, 'flex-[1_1_200px]')}
      />
      <input
        name="team"
        list="teams"
        placeholder="Scuderia (opzionale)"
        defaultValue={player?.team ?? ''}
        className={cx(fieldClass, 'flex-[1_1_200px]')}
      />
    </>
  );
}

// Turnazione dei controller nel torneo in corso: chi corre il prossimo Gran
// Premio e i successivi tra i presenti. Prima del primo Gran Premio si può
// rifare il sorteggio, che decide chi parte.
function RotationCard({ rotation, players, onDraw }: {
  rotation: RotationState;
  players: Player[];
  onDraw: () => void;
}) {
  const name = (id: number) => players.find((p) => p.id === id)?.name ?? '?';
  const pair = (p: RotationPair) => `${name(p.p1)} e ${name(p.p2)}`;
  const [next, ...later] = rotation.next;

  let body: ReactNode;
  if (!rotation.tournament) {
    body = <p className="text-sm text-muted">Nessun torneo in corso: la turnazione dei controller parte con il torneo.</p>;
  } else if (!next) {
    body = <p className="text-sm text-muted">Segna almeno due piloti presenti per sapere chi prende i controller.</p>;
  } else {
    const resting = rotation.present.filter((id) => id !== next.p1 && id !== next.p2);
    body = (
      <div className="text-sm">
        <p>
          <span className="text-muted">{rotation.canDraw ? 'Primo Gran Premio' : 'Prossimo Gran Premio'}:</span>{' '}
          <span className="font-semibold">{name(next.p1)}</span> <span className="text-muted">(controller 1)</span> e{' '}
          <span className="font-semibold">{name(next.p2)}</span> <span className="text-muted">(controller 2)</span>
          {resting.length > 0 && <span className="text-muted"> · {resting.length === 1 ? 'riposa' : 'riposano'} {resting.map(name).join(', ')}</span>}
        </p>
        {later.length > 0 && <p className="text-muted">Poi: {later.map(pair).join(' · ')}</p>}
      </div>
    );
  }

  return (
    <Card className="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 className="font-bold">Turnazione{rotation.tournament && <span className="font-normal text-muted"> · {rotation.tournament.name}</span>}</h2>
        {body}
      </div>
      {rotation.canDraw && next && (
        <Button variant="ghost" size="sm" onClick={onDraw} title="Sorteggia di nuovo chi parte: si può rifare finché il torneo non ha Gran Premi">
          Genera coppia
        </Button>
      )}
    </Card>
  );
}
