import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { useToast } from '../lib/toast';
import type { Tournament } from '../lib/types';
import { Button, cx, Dialog, fieldClass } from './ui';

interface Props {
  open: boolean;
  defaultName: string;
  canStart: boolean; // falso se c'è già un torneo in corso
  onClose: () => void;
  onCreated: (tournament: Tournament) => Promise<void>;
}

export function NewTournamentDialog({ open, ...props }: Props) {
  return (
    <Dialog open={open} title="Nuovo torneo" onClose={props.onClose}>
      <NewTournamentForm {...props} />
    </Dialog>
  );
}

function NewTournamentForm({ defaultName, canStart, onClose, onCreated }: Omit<Props, 'open'>) {
  const { run } = useToast();
  const [saving, setSaving] = useState(false);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const name = String(form.get('name')).trim();
    setSaving(true);
    run(async () => {
      const tournament = await api.createTournament(name, form.has('start'));
      onClose();
      await onCreated(tournament);
    }).finally(() => setSaving(false));
  }

  return (
    <form onSubmit={handleSubmit}>
      <label className="mb-1 block text-sm text-muted" htmlFor="tournament-name">Nome del torneo</label>
      <input
        id="tournament-name"
        name="name"
        required
        pattern=".*\S.*" // required da solo accetterebbe un nome fatto di soli spazi
        title="Inserisci un nome"
        defaultValue={defaultName}
        // Testo già selezionato: si può confermare il nome proposto o scriverne subito un altro.
        onFocus={(e) => e.target.select()}
        className={cx(fieldClass, 'w-full')}
      />
      <label className={cx('mt-4 flex items-start gap-2 text-sm', !canStart && 'text-muted')}>
        <input type="checkbox" name="start" defaultChecked={canStart} disabled={!canStart} className="mt-0.5 size-4 accent-accent" />
        <span>
          Inizia subito: le prossime gare verranno salvate in questo torneo
          {!canStart && ' (c\'è già un torneo in corso)'}
        </span>
      </label>
      <div className="mt-6 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>Annulla</Button>
        <Button disabled={saving}>Crea torneo</Button>
      </div>
    </form>
  );
}
