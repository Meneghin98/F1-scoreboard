import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { RACE_TYPES } from '../lib/f1';
import { formatDate, raceLabels } from '../lib/format';
import { useToast } from '../lib/toast';
import type { RaceType, SessionRecord, Tournament } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { Button, cx, Dialog, fieldClass } from './ui';

interface Props {
  session: SessionRecord | null; // aperto quando c'è una sessione
  tournaments: Tournament[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}

// Collega una registrazione a una gara di un torneo, esistente o creata al momento.
export function AssignRaceDialog({ session, tournaments, onClose, onSaved }: Props) {
  return (
    <Dialog open={session !== null} title="Assegna a un torneo" onClose={onClose}>
      {session && <AssignRaceForm session={session} tournaments={tournaments} onClose={onClose} onSaved={onSaved} />}
    </Dialog>
  );
}

function AssignRaceForm({ session, tournaments, onClose, onSaved }: Omit<Props, 'session'> & { session: SessionRecord }) {
  const { run } = useToast();
  const [saving, setSaving] = useState(false);
  // Di partenza: il torneo della registrazione, altrimenti quello selezionato in alto.
  const [tournamentId, setTournamentId] = useState<number | null>(() => {
    const selected = Number(localStorage.getItem('tournamentId'));
    const fallback = tournaments.some((t) => t.id === selected) ? selected : tournaments[0]?.id;
    return session.race?.tournamentId ?? fallback ?? null;
  });
  // 'new' crea una nuova gara sul circuito della registrazione.
  const [raceId, setRaceId] = useState<number | 'new'>(session.race?.id ?? 'new');
  // Il tipo proposto viene dalla sessione; le sprint il frontend non le riconosce.
  const [type, setType] = useState<RaceType>(session.kind || 'race');
  const { data: races } = useLoad(
    () => (tournamentId ? api.races(tournamentId) : Promise.resolve([])),
    [tournamentId],
  );
  const labels = raceLabels(races ?? []);

  function save(action: () => Promise<void>) {
    setSaving(true);
    run(async () => {
      await action();
      onClose();
      await onSaved();
    }).finally(() => setSaving(false));
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!tournamentId) return;
    save(() => api.setSessionRace(
      session.id,
      raceId === 'new' ? { tournamentId, type } : { raceId },
    ));
  }

  if (!tournaments.length) {
    return (
      <>
        <p className="text-muted">Crea prima un torneo con il pulsante "+ Torneo" in alto.</p>
        <div className="mt-6 flex justify-end">
          <Button variant="ghost" onClick={onClose}>Chiudi</Button>
        </div>
      </>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <label className="flex flex-col gap-1">
        <span className="text-sm text-muted">Torneo</span>
        <select
          className={fieldClass}
          value={tournamentId ?? ''}
          onChange={(e) => {
            setTournamentId(Number(e.target.value));
            setRaceId('new');
          }}
        >
          {tournaments.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </select>
      </label>

      <label className="flex flex-col gap-1">
        <span className="text-sm text-muted">Gara</span>
        <select
          className={fieldClass}
          value={raceId}
          disabled={!races}
          onChange={(e) => setRaceId(e.target.value === 'new' ? 'new' : Number(e.target.value))}
        >
          <option value="new">Nuova gara: {session.track}</option>
          {races?.map((r) => (
            <option key={r.id} value={r.id}>
              {labels.get(r.id)} {r.track}{r.date ? ` · ${formatDate(r.date)}` : ''}
            </option>
          ))}
        </select>
      </label>

      {raceId === 'new' && (
        <label className="flex flex-col gap-1">
          <span className="text-sm text-muted">Tipo</span>
          <select className={fieldClass} value={type} onChange={(e) => setType(e.target.value as RaceType)}>
            {RACE_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
        </label>
      )}

      <p className={cx('text-sm', session.providesResults ? 'text-fg' : 'text-muted')}>
        {session.providesResults
          ? 'I risultati verranno presi dalla classifica finale di questa registrazione e sostituiranno quelli già inseriti.'
          : 'Questa registrazione non ha la classifica finale: i risultati non cambiano.'}
      </p>

      <div className="mt-2 flex flex-wrap justify-end gap-2">
        {/* Le sessioni finite stanno sempre in un torneo: si possono spostare, non togliere. */}
        {session.race && !session.providesResults && (
          <Button
            type="button"
            variant="ghost"
            className="mr-auto"
            disabled={saving}
            onClick={() => save(() => api.setSessionRace(session.id, { raceId: null }))}
          >
            Togli dal torneo
          </Button>
        )}
        <Button type="button" variant="ghost" onClick={onClose}>Annulla</Button>
        <Button disabled={saving || !tournamentId}>Salva</Button>
      </div>
    </form>
  );
}
