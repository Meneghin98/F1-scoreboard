import { api } from '../lib/api';
import { formatClock } from '../lib/format';
import { teamColor } from '../lib/live';
import { href } from '../lib/route';
import { useToast } from '../lib/toast';
import type { Lineup, LineupCar, Player } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { Button, Card, cx, dnfClass, fieldSmClass, rankClass } from './ui';

function title({ stage, kind }: Lineup): string {
  if (stage === 'drivers') return 'Chi guida?';
  if (stage === 'racing') return 'Piloti in pista';
  if (stage === 'results') return 'Conferma i risultati';
  return kind === 'race' ? 'Gara salvata' : 'Sessione salvata';
}

// Chi tiene in mano i controller: il gioco non lo sa, quindi lo si sceglie qui.
// Senza interventi il server conferma da solo (alla partenza o dopo 5 minuti)
// e a fine sessione salva i risultati nel torneo in corso o in "Nessun torneo".
// Vale per tutte le sessioni del weekend; prove libere e qualifiche non danno punti.
export function LineupCard({ lineup }: { lineup: Lineup }) {
  const { run } = useToast();
  const { data: players } = useLoad(() => api.players(), []);
  const { stage, kind, cars, tournament } = lineup;
  const waiting = stage === 'drivers' || stage === 'results'; // si aspetta una conferma

  // Scegliere un pilota che guida già l'altra auto scambia i due piloti.
  function choose(car: LineupCar, playerId: number | null) {
    const other = cars.find((c) => c.carIndex !== car.carIndex && playerId !== null && c.playerId === playerId);
    const choices = [{ carIndex: car.carIndex, playerId }];
    if (other) choices.push({ carIndex: other.carIndex, playerId: car.playerId });
    void run(() => api.setLineup(choices, false));
  }

  function confirm() {
    void run(() => api.setLineup([], true));
  }

  return (
    <Card className={cx(waiting && 'border-accent')}>
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-lg font-bold">{title(lineup)}</h2>
        <span className="text-sm text-muted">
          {tournament && (stage !== 'saved' && tournament.free ? (
            'Nessun torneo in corso'
          ) : (
            <>
              {stage === 'saved' ? 'Salvata in ' : 'Verrà salvata in '}
              <span className="font-semibold text-fg">{tournament.name}</span>
            </>
          ))}
          {kind !== 'race' && ' · non assegna punti'}
        </span>
      </div>

      {cars.length > 0 && (
        <ul className="flex flex-col gap-2">
          {cars.map((car) => (
            <li key={car.carIndex} className="flex flex-wrap items-center gap-3">
              <span className="w-24 shrink-0 text-xs font-bold tracking-wider text-muted uppercase">
                Controller {car.controller}
              </span>
              <span className="flex min-w-40 flex-1 items-center gap-2">
                <span className="h-5 w-1 shrink-0 rounded-full" style={{ background: teamColor(car.team) }} />
                <span className="truncate">
                  {car.name || `Auto ${car.carIndex + 1}`}
                  {car.raceNumber > 0 && <span className="ml-1 text-sm text-muted">#{car.raceNumber}</span>}
                  {car.team && <span className="ml-2 text-sm text-muted">{car.team}</span>}
                </span>
              </span>
              {(stage === 'results' || stage === 'saved') && (
                car.dnf
                  ? <span className={cx(dnfClass, 'w-12')}>DNF</span>
                  : <span className={cx(rankClass(car.position), 'w-12')}>P{car.position}</span>
              )}
              {stage === 'saved' ? (
                <span className="w-44 font-semibold">{car.playerName || <span className="text-muted">Nessun pilota</span>}</span>
              ) : (
                <select
                  aria-label={`Pilota del controller ${car.controller}`}
                  className={cx(fieldSmClass, 'w-44')}
                  value={car.playerId ?? ''}
                  onChange={(e) => choose(car, e.target.value ? Number(e.target.value) : null)}
                >
                  <option value="">Nessun pilota</option>
                  {players?.map((p: Player) => <option key={p.id} value={p.id}>{p.name}</option>)}
                </select>
              )}
            </li>
          ))}
        </ul>
      )}
      {players?.length === 0 && stage !== 'saved' && (
        <p className="mt-2 text-sm text-muted">
          Nessun pilota: aggiungili nella pagina <a className="underline" href={href('players')}>Piloti</a>.
        </p>
      )}

      {waiting && (
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
          <span className="text-sm text-muted">
            {lineup.error ? (
              <span className="text-accent-text">
                Salvataggio non riuscito ({lineup.error}): nuovo tentativo tra{' '}
                <b className="tabular-nums">{formatClock(lineup.secondsLeft ?? 0)}</b>
              </span>
            ) : (
              <Countdown lineup={lineup} />
            )}
          </span>
          <Button size="sm" onClick={confirm}>{stage === 'drivers' ? 'Conferma piloti' : 'Conferma e salva'}</Button>
        </div>
      )}
      {stage === 'saved' && (
        <p className="mt-3 text-sm text-muted">
          Per correggere i piloti usa la pagina <a className="underline" href={href('recordings')}>Registrazioni</a>.
        </p>
      )}
    </Card>
  );
}

function Countdown({ lineup: { stage, kind, secondsLeft } }: { lineup: Lineup }) {
  if (stage === 'drivers') {
    // Fuori dalla gara non c'è una partenza: la scelta si chiude al secondo giro.
    const start = kind === 'race' ? 'alla partenza' : "all'inizio del secondo giro";
    return secondsLeft !== null
      ? <>Conferma automatica tra <b className="text-fg tabular-nums">{formatClock(secondsLeft)}</b> o {start}</>
      : <>Conferma automatica {start}</>;
  }
  return secondsLeft !== null
    ? <>Salvataggio automatico tra <b className="text-fg tabular-nums">{formatClock(secondsLeft)}</b></>
    : <>Salvataggio automatico sospeso: conferma per salvare (o a fine sessione)</>;
}
