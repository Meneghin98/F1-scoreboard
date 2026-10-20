import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { DEFAULT_RULES, POINTS_MODES } from '../lib/f1';
import { useToast } from '../lib/toast';
import type { PointsMode, ScoringRules, Tournament } from '../lib/types';
import { Button, cx, Dialog, fieldClass } from './ui';

interface Props {
  open: boolean;
  tournament?: Tournament; // torneo da modificare; senza, se ne crea uno nuovo
  defaultName?: string; // nome proposto per un torneo nuovo
  canStart?: boolean; // falso se c'è già un torneo in corso
  onClose: () => void;
  onSaved: (tournament: Tournament) => Promise<void>;
}

// Crea un torneo, o ne cambia nome e regole di punteggio.
export function TournamentDialog({ open, ...props }: Props) {
  return (
    <Dialog open={open} title={props.tournament ? 'Impostazioni del torneo' : 'Nuovo torneo'} onClose={props.onClose}>
      <TournamentForm {...props} />
    </Dialog>
  );
}

function TournamentForm({ tournament, defaultName = '', canStart = false, onClose, onSaved }: Omit<Props, 'open'>) {
  const { run } = useToast();
  const [saving, setSaving] = useState(false);
  const current: ScoringRules = tournament ?? DEFAULT_RULES;

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const name = String(form.get('name')).trim();
    const rules: ScoringRules = {
      points_mode: form.get('points_mode') as PointsMode,
      pole_points: Number(form.get('pole_points')),
      fastest_lap_points: Number(form.get('fastest_lap_points')),
    };
    setSaving(true);
    run(async () => {
      const saved = tournament
        ? await api.updateTournament(tournament.id, name, rules)
        : await api.createTournament(name, form.has('start'), rules);
      onClose();
      await onSaved(saved);
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
        defaultValue={tournament?.name ?? defaultName}
        // Testo già selezionato: si può confermare il nome proposto o scriverne subito un altro.
        onFocus={(e) => e.target.select()}
        className={cx(fieldClass, 'w-full')}
      />

      <fieldset className="mt-4">
        <legend className="mb-2 text-sm text-muted">Punti in base alla</legend>
        <div className="flex flex-col gap-2">
          {POINTS_MODES.map((m) => (
            <label key={m.value} className="flex items-start gap-2 text-sm">
              <input
                type="radio"
                name="points_mode"
                value={m.value}
                defaultChecked={current.points_mode === m.value}
                className="mt-0.5 size-4 shrink-0 accent-accent"
              />
              <span>
                <span className="font-semibold">{m.label}</span>
                <span className="block text-muted">{m.hint}</span>
              </span>
            </label>
          ))}
        </div>
      </fieldset>

      <div className="mt-4 grid grid-cols-2 gap-3">
        <BonusField name="pole_points" label="Punti per la pole" hint="Al primo della qualifica" value={current.pole_points} />
        <BonusField
          name="fastest_lap_points"
          label="Punti per il giro veloce"
          hint="Nei Gran Premi, solo a chi arriva"
          value={current.fastest_lap_points}
        />
      </div>

      {tournament ? (
        tournament.races > 0 && (
          <p className="mt-4 text-sm text-muted">La classifica si ricalcola con le nuove regole, anche per le gare già corse.</p>
        )
      ) : (
        <label className={cx('mt-4 flex items-start gap-2 text-sm', !canStart && 'text-muted')}>
          <input type="checkbox" name="start" defaultChecked={canStart} disabled={!canStart} className="mt-0.5 size-4 accent-accent" />
          <span>
            Inizia subito: le prossime gare verranno salvate in questo torneo
            {!canStart && ' (c\'è già un torneo in corso)'}
          </span>
        </label>
      )}
      <div className="mt-6 flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>Annulla</Button>
        <Button disabled={saving}>{tournament ? 'Salva' : 'Crea torneo'}</Button>
      </div>
    </form>
  );
}

// Punti bonus (pole o giro veloce): 0 per non darne.
function BonusField({ name, label, hint, value }: { name: string; label: string; hint: string; value: number }) {
  return (
    <label className="text-sm">
      <span className="mb-1 block text-muted">{label}</span>
      <input type="number" name={name} min={0} max={25} step={1} required defaultValue={value} className={cx(fieldClass, 'w-24')} />
      <span className="mt-1 block text-xs text-muted">{hint}</span>
    </label>
  );
}
