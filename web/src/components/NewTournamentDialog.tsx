import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { useToast } from '../lib/toast';
import type { Tournament } from '../lib/types';
import { Button, cx, Dialog, fieldClass } from './ui';

interface Props {
  open: boolean;
  defaultName: string;
  onClose: () => void;
  onCreated: (tournament: Tournament) => Promise<void>;
}

export function NewTournamentDialog({ open, defaultName, onClose, onCreated }: Props) {
  return (
    <Dialog open={open} title="Nuovo torneo" onClose={onClose}>
      <NewTournamentForm defaultName={defaultName} onClose={onClose} onCreated={onCreated} />
    </Dialog>
  );
}

function NewTournamentForm({ defaultName, onClose, onCreated }: Omit<Props, 'open'>) {
  const { run } = useToast();
  const [saving, setSaving] = useState(false);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const name = String(new FormData(e.currentTarget).get('name')).trim();
    setSaving(true);
    run(async () => {
      const tournament = await api.createTournament(name);
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
      <div className="mt-6 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>Annulla</Button>
        <Button disabled={saving}>Crea torneo</Button>
      </div>
    </form>
  );
}
