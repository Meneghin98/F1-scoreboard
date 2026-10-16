import type { FormEvent } from 'react';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import type { Player } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { Button, Card, cx, Empty, fieldClass, TeamTag } from './ui';

export function PlayersTab() {
  const { run } = useToast();
  const confirm = useConfirm();
  const { data: players, reload } = useLoad(api.players, []);

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
          <input name="name" placeholder="Nome pilota" required className={cx(fieldClass, 'flex-[1_1_200px]')} />
          <input name="team" list="teams" placeholder="Scuderia (opzionale)" className={cx(fieldClass, 'flex-[1_1_200px]')} />
          <Button>Aggiungi pilota</Button>
        </form>
      </Card>
      {players && (players.length ? players.map((p) => (
        <Card key={p.id} className="flex items-center justify-between">
          <span>{p.name} <TeamTag team={p.team} /></span>
          <Button variant="ghost" size="sm" onClick={() => handleDelete(p)}>Elimina</Button>
        </Card>
      )) : <Empty>Nessun pilota registrato.</Empty>)}
    </>
  );
}
