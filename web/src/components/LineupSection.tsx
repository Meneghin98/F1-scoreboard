import { useState, type ReactNode } from 'react';
import { api } from '../lib/api';
import { formatClock } from '../lib/format';
import { teamColor } from '../lib/live';
import { href } from '../lib/route';
import { useToast } from '../lib/toast';
import type { Lineup, LineupCar, LineupStage, Player, SavedLineup } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { Button, Card, cx, dnfClass, fieldSmClass, rankClass } from './ui';

function title({ stage, kind }: Lineup): string {
  if (stage === 'drivers') return 'Chi guida?';
  if (stage === 'racing') return 'Piloti in pista';
  if (stage === 'results') return 'Risultati';
  return kind === 'race' ? 'Gara salvata' : 'Sessione salvata';
}

// La scheda della sessione si evidenzia solo se c'è da intervenire: un'auto
// senza pilota o un salvataggio non riuscito. Altrimenti si va avanti da soli.
export function lineupNeedsAttention({ stage, cars, error }: Lineup): boolean {
  return (stage === 'drivers' || stage === 'results') && (error !== '' || cars.some((c) => c.playerId === null));
}

// A piloti confermati la scelta si riduce a un riepilogo nella riga delle
// previsioni; si riapre a richiesta e torna ridotta a ogni cambio di fase.
// Restituisce se mostrare il riepilogo e come aprire (true) o chiudere la scelta.
export function useLineupSummary(stage: LineupStage | undefined): [boolean, (editing: boolean) => void] {
  const [editing, setEditing] = useState(false);
  const [prevStage, setPrevStage] = useState(stage);
  if (stage !== prevStage) {
    setPrevStage(stage);
    setEditing(false);
  }
  return [stage === 'racing' && !editing, setEditing];
}

// Riepilogo su una riga di chi guida cosa, a piloti confermati.
export function LineupSummary({ lineup, onEdit }: { lineup: Lineup; onEdit: () => void }) {
  return (
    <span className="flex min-w-0 flex-1 basis-64 items-center gap-x-4">
      <span className="shrink-0 text-muted">Piloti</span>
      <ul className="flex min-w-0 items-center gap-x-4 overflow-hidden">
        {lineup.cars.map((car) => (
          <li key={car.carIndex} className="flex min-w-0 items-center gap-2">
            <span className="h-3.5 w-1 shrink-0 rounded-full" style={{ background: teamColor(car.team) }} />
            <span className="truncate">
              <span className="font-semibold">{car.playerName || <span className="text-muted">Nessun pilota</span>}</span>
              <span className="ml-2 text-muted">{car.name || `Auto ${car.carIndex + 1}`}</span>
            </span>
          </li>
        ))}
      </ul>
      <button type="button" className="shrink-0 text-muted underline hover:text-fg" onClick={onEdit}>
        Modifica
      </button>
    </span>
  );
}

type Choice = { carIndex: number; playerId: number | null };

// Scegliere un pilota che guida già l'altra auto scambia i due piloti.
function choicesFor(cars: LineupCar[], car: LineupCar, playerId: number | null): Choice[] {
  const other = cars.find((c) => c.carIndex !== car.carIndex && playerId !== null && c.playerId === playerId);
  const choices = [{ carIndex: car.carIndex, playerId }];
  if (other) choices.push({ carIndex: other.carIndex, playerId: car.playerId });
  return choices;
}

// Chi tiene in mano i controller: il gioco non lo sa, quindi lo si sceglie qui.
// Senza interventi il server conferma da solo (alla partenza o dopo 5 minuti)
// e a fine sessione salva i risultati nel torneo in corso o in "Nessun torneo".
// Vale per tutte le sessioni del weekend; prove libere e qualifiche non danno punti.
// È la parte bassa della scheda della sessione; onDone richiude la scelta già confermata.
export function LineupSection({ lineup, onDone }: { lineup: Lineup; onDone: () => void }) {
  const { run, toast } = useToast();
  const { data: players } = useLoad(() => api.players(), []);
  const [busy, setBusy] = useState(false);
  const { sessionId, stage, kind, cars, tournament } = lineup;
  const open = stage === 'drivers' || stage === 'results';

  function choose(car: LineupCar, playerId: number | null) {
    void run(async () => {
      await api.setLineup(sessionId, choicesFor(cars, car, playerId), false);
    });
  }

  // Se nel frattempo la sessione è finita il server l'ha già salvata da solo:
  // la risposta dice comunque dove.
  function confirm() {
    setBusy(true);
    void run(async () => {
      const res = await api.setLineup(sessionId, [], true);
      if (stage === 'results' && res.tournament) toast(`Risultati salvati in "${res.tournament.name}"`);
    }).finally(() => setBusy(false));
  }

  return (
    <div className="w-full border-t border-line pt-3">
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-bold">{title(lineup)}</h2>
        <span className="flex items-center gap-3">
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
          {stage === 'racing' && <Button variant="ghost" size="sm" onClick={onDone}>Fatto</Button>}
        </span>
      </div>

      <CarList
        cars={cars}
        results={stage === 'results' || stage === 'saved'}
        player={(car) => stage === 'saved'
          ? <span className="w-44 font-semibold">{car.playerName || <span className="text-muted">Nessun pilota</span>}</span>
          : <PlayerSelect car={car} players={players} onChange={(id) => choose(car, id)} />}
      />
      {players?.length === 0 && stage !== 'saved' && (
        <p className="mt-2 text-sm text-muted">
          Nessun pilota: aggiungili nella pagina <a className="underline" href={href('players')}>Piloti</a>.
        </p>
      )}

      {open && (
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
          <Button size="sm" variant={lineupNeedsAttention(lineup) ? 'primary' : 'ghost'} disabled={busy} onClick={confirm}>
            {stage === 'drivers' ? 'Conferma piloti' : 'Salva ora'}
          </Button>
        </div>
      )}
      {stage === 'saved' && (
        <p className="mt-3 text-sm text-muted">
          Per correggere i piloti usa la pagina <a className="underline" href={href('recordings')}>Registrazioni</a>.
        </p>
      )}
    </div>
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
    ? <>Si salvano da soli tra <b className="text-fg tabular-nums">{formatClock(secondsLeft)}</b> o quando parte la sessione successiva</>
    : <>Piloti modificati: si salvano quando parte la sessione successiva, o subito con "Salva ora"</>;
}

// L'ultima sessione salvata, mostrata quando nel gioco si è già passati alla
// successiva: dice dove è finita e permette di correggerne i piloti.
export function SavedLineupCard({ saved }: { saved: SavedLineup }) {
  const { run, toast } = useToast();
  const { data: players } = useLoad(() => api.players(), []);
  const { sessionId, cars, tournament, kind } = saved;

  function choose(car: LineupCar, playerId: number | null) {
    void run(async () => {
      await api.setSessionPlayers(sessionId, choicesFor(cars, car, playerId));
      toast('Piloti corretti');
    });
  }

  return (
    <Card>
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-bold">{saved.track} · {saved.session}</h2>
        <span className="text-sm text-muted">
          {tournament && <>Salvata in <span className="font-semibold text-fg">{tournament.name}</span></>}
          {kind !== 'race' && ' · non assegna punti'}
        </span>
      </div>
      <CarList
        cars={cars}
        results
        player={(car) => <PlayerSelect car={car} players={players} onChange={(id) => choose(car, id)} />}
      />
      <p className="mt-3 text-sm text-muted">
        Si può correggere anche più tardi dalla pagina <a className="underline" href={href('recordings')}>Registrazioni</a>.
      </p>
    </Card>
  );
}

// Le auto dei giocatori, una per riga: controller, auto, posizione finale e pilota.
function CarList({ cars, results, player }: { cars: LineupCar[]; results: boolean; player: (car: LineupCar) => ReactNode }) {
  if (cars.length === 0) return null;
  return (
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
          {results && (
            car.dnf
              ? <span className={cx(dnfClass, 'w-12')}>DNF</span>
              : <span className={cx(rankClass(car.position), 'w-12')}>P{car.position}</span>
          )}
          {player(car)}
        </li>
      ))}
    </ul>
  );
}

function PlayerSelect({ car, players, onChange }: {
  car: LineupCar;
  players: Player[] | null | undefined;
  onChange: (playerId: number | null) => void;
}) {
  return (
    <select
      aria-label={`Pilota del controller ${car.controller}`}
      className={cx(fieldSmClass, 'w-44')}
      value={car.playerId ?? ''}
      onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
    >
      <option value="">Nessun pilota</option>
      {players?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
    </select>
  );
}
