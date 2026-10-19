import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import type { Player } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { Button, Card, cx, Empty, fieldClass, TeamTag } from '../components/ui';

export function PlayersPage() {
  const { run } = useToast();
  const confirm = useConfirm();
  const { data: players, reload } = useLoad(api.players, []);
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
      await api.updatePlayer(player.id, String(fields.get('name')), String(fields.get('team')));
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

  return (
    <>
      <Card>
        <form className="flex flex-wrap gap-2" onSubmit={handleAdd}>
          <PlayerFields />
          <Button>Aggiungi pilota</Button>
        </form>
      </Card>
      {players && (players.length ? players.map((p) => (
        <Card key={p.id}>
          {editingId === p.id ? (
            <form className="flex flex-wrap gap-2" onSubmit={(e) => handleSave(p, e)}>
              <PlayerFields player={p} />
              <Button type="button" variant="ghost" onClick={() => setEditingId(null)}>Annulla</Button>
              <Button>Salva</Button>
            </form>
          ) : (
            <div className="flex items-center justify-between gap-2">
              <span>{p.name} <TeamTag team={p.team} /></span>
              <div className="flex gap-2">
                <Button variant="ghost" size="sm" onClick={() => setEditingId(p.id)}>Modifica</Button>
                <Button variant="ghost" size="sm" onClick={() => handleDelete(p)}>Elimina</Button>
              </div>
            </div>
          )}
        </Card>
      )) : <Empty>Nessun pilota registrato.</Empty>)}
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
