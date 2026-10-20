import { useState, type FormEvent } from 'react';
import { api } from '../lib/api';
import { DEFAULT_RULES, F1_POINTS, POINTS_MODES, POINTS_PRESETS } from '../lib/f1';
import { useToast } from '../lib/toast';
import type { PointsMode, PointsPreset, ScoringRules, Tournament } from '../lib/types';
import { Button, cx, Dialog, fieldClass, fieldSmClass } from './ui';

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
  const [mode, setMode] = useState(current.points_mode);
  const [preset, setPreset] = useState(current.points_preset);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const name = String(form.get('name')).trim();
    const rules: ScoringRules = {
      points_mode: form.get('points_mode') as PointsMode,
      pole_points: Number(form.get('pole_points')),
      fastest_lap_points: Number(form.get('fastest_lap_points')),
      // Senza la select (modalità assoluta) la tabella resta quella salvata.
      points_preset: (form.get('points_preset') as PointsPreset | null) ?? current.points_preset,
      race_points: pointsTable(form, 'race_points'),
      sprint_points: pointsTable(form, 'sprint_points'),
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
                onChange={() => setMode(m.value)}
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

      {/* Campi disabilitati quando non servono: non finiscono nel form e le scelte salvate restano. */}
      <fieldset disabled={mode !== 'players'} hidden={mode !== 'players'} className="mt-4">
        <label className="mb-1 block text-sm text-muted" htmlFor="points-preset">Tabella dei punti tra i piloti</label>
        <select
          id="points-preset"
          name="points_preset"
          value={preset}
          onChange={(e) => setPreset(e.target.value as PointsPreset)}
          className={cx(fieldClass, 'w-full')}
        >
          {POINTS_PRESETS.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
        </select>
        <p className="mt-1 text-xs text-muted">{POINTS_PRESETS.find((p) => p.value === preset)?.hint}</p>

        <fieldset disabled={preset !== 'custom'} hidden={preset !== 'custom'} className="mt-3 flex flex-col gap-3">
          <PointsTableField name="race_points" label="Gran Premio" values={current.race_points} f1={F1_POINTS.race} />
          <PointsTableField name="sprint_points" label="Sprint" values={current.sprint_points} f1={F1_POINTS.sprint} />
        </fieldset>
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

// Valori di una tabella dei punti dal form; null se i campi sono disabilitati,
// così il server lascia la tabella com'è.
function pointsTable(form: FormData, name: string): number[] | null {
  const values = form.getAll(name);
  return values.length ? values.map(Number) : null;
}

// Un campo per ogni posizione, con i punti del torneo o quelli della F1. Il
// server toglie gli zeri in fondo: qui si mostrano almeno le posizioni della F1.
function PointsTableField({ name, label, values, f1 }: { name: string; label: string; values: number[] | null; f1: number[] }) {
  const table = values ?? f1;
  const positions = Math.max(table.length, f1.length);
  return (
    <div className="text-sm">
      <span className="mb-1 block font-semibold">{label}</span>
      <div className="grid grid-cols-5 gap-2">
        {Array.from({ length: positions }, (_, i) => (
          <label key={i} className="flex items-center gap-1">
            <span className="w-6 text-right text-xs text-muted">{i + 1}°</span>
            <input
              type="number"
              name={name}
              min={0}
              max={100}
              step={1}
              required
              defaultValue={table[i] ?? 0}
              aria-label={`${label}, punti al ${i + 1}°`}
              className={cx(fieldSmClass, 'w-full min-w-0')}
            />
          </label>
        ))}
      </div>
    </div>
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
