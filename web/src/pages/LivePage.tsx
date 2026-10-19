import { Fragment, useState, type ReactNode } from 'react';
import { formatClock, formatGap, formatLapTime } from '../lib/format';
import {
  controllerLabel, isLive, isLoading, isPaused, SAFETY_CAR, teamColor, telemetryProblem, TYRE_CLASS, WEATHER,
} from '../lib/live';
import type { LiveDriver, LiveSession, LiveSnapshot, PlayerCar, RaceEvent, TelemetryStatus, TrackMap } from '../lib/types';
import { useLive, usePedalHistory, type PedalSample } from '../lib/useLive';
import { href } from '../lib/route';
import { DuelCard } from '../components/DuelCard';
import { LiveDot } from '../components/LiveBadge';
import { LineupCard } from '../components/LineupCard';
import { PlayerCarCard } from '../components/PlayerCarCard';
import { RaceControlCard } from '../components/RaceControlCard';
import { TrackMapCard } from '../components/TrackMapCard';
import { Button, buttonClass, Card, ColorDot, cx, Empty, rankClass, Table, TeamTag, Td, Th, Tr } from '../components/ui';

// Classifica della sessione in corso, aggiornata in tempo reale dalla telemetria.
export function LivePage() {
  const live = useLive();
  const pedals = usePedalHistory();

  if (!live) return <Empty>Connessione al server in corso…</Empty>;
  if (!isLive(live)) return <WaitingCard telemetry={live.telemetry} />;

  if (isLoading(live.session)) {
    return (
      <Card className="flex items-center gap-3">
        <LiveDot active />
        <div>
          <div className="text-xl font-bold">Sessione in caricamento…</div>
          <div className="text-sm text-muted">Circuito e piloti arrivano quando il gioco ti porta ai box o in pista.</div>
        </div>
      </Card>
    );
  }

  return (
    <>
      <SessionCard
        live={live}
        session={live.session}
        actions={
          <a href={href('tv')} className={buttonClass('ghost', 'sm')} title="Classifica e mappa a schermo intero, per la TV">
            Modalità TV
          </a>
        }
      />
      {/* A sessione finita non arrivano più dati: il server la chiude poco dopo. */}
      {live.status === 'paused' && !live.finished && (
        <p className="mb-4 text-sm text-muted">
          Il gioco non sta inviando dati (pausa, menù o impostazioni): si vede l'ultimo stato ricevuto.
        </p>
      )}
      {live.lineup && <LineupCard lineup={live.lineup} />}
      {live.drivers.length ? (
        <LiveBoard session={live.session} drivers={live.drivers} players={live.players} events={live.events} finished={live.finished} pedals={pedals} />
      ) : (
        <Empty>In attesa dei dati dei piloti…</Empty>
      )}
    </>
  );
}

// Senza una sessione in corso: cosa impostare nel gioco perché la telemetria
// arrivi qui, ed eventuali problemi di ricezione.
export function WaitingCard({ telemetry: t }: { telemetry?: TelemetryStatus }) {
  const problem = telemetryProblem(t);
  const [primary, ...others] = t?.addresses ?? [];
  const settings: [string, ReactNode][] = [
    ['Telemetria UDP', 'Attiva'],
    ['Modalità broadcast UDP', 'Disattivata'],
    [
      'Indirizzo IP UDP',
      <>
        <span className="font-semibold">{primary ?? 'IP di questo PC'}</span>
        {others.length > 0 && <span className="text-muted"> (oppure {others.join(', ')})</span>}
        <span className="text-muted"> · 127.0.0.1 se il gioco gira su questo PC</span>
      </>,
    ],
    ['Porta UDP', <span className="font-semibold">{t?.port || 20777}</span>],
    ['Frequenza di invio UDP', '20 Hz'],
    ['Formato UDP', '2025, oppure 2026 con il DLC 2026 Season Pack'],
  ];

  return (
    <Card>
      <div className="mb-4 flex items-center gap-3">
        <LiveDot />
        <div>
          <h2 className="text-xl font-bold">Nessuna sessione in corso</h2>
          <p className="text-sm text-muted">La classifica compare appena il gioco invia la telemetria, in pista o nel garage.</p>
        </div>
      </div>
      {problem && (
        <p className="mb-4 rounded-md border border-accent/60 bg-accent/10 px-3 py-2 text-sm">{problem.long}</p>
      )}
      <h3 className="mb-2 text-xs font-bold tracking-wider text-muted uppercase">
        Nel gioco: Impostazioni → Impostazioni telemetria
      </h3>
      <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
        {settings.map(([label, value]) => (
          <Fragment key={label}>
            <dt className="text-muted">{label}</dt>
            <dd>{value}</dd>
          </Fragment>
        ))}
      </dl>
    </Card>
  );
}

interface LiveBoardProps {
  session: LiveSession;
  drivers: LiveDriver[];
  players: PlayerCar[];
  events: RaceEvent[];
  pedals: PedalSample[]; // di tutti i giocatori: ogni scheda prende quelli del suo controller
  loadMap?: () => Promise<TrackMap>;
  mapPoints?: number; // punti della pianta, se diversi da quelli della sessione
  finished: boolean; // classifica finale arrivata
}

// Classifica ridotta a sinistra di mappa e auto dei giocatori; estesa mostra
// tutti i dati e nasconde la mappa. È usata anche dal replay, che passa la sua
// pianta (loadMap) e il suo storico dei pedali.
export function LiveBoard({ session, drivers, players, events, finished, pedals, loadMap, mapPoints }: LiveBoardProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    // Solo la mappa ha una dimensione fissa: la classifica si allunga quanto serve per tutti i piloti.
    <div className={cx('mb-4 grid items-start gap-4', !expanded && 'md:grid-cols-[20rem_1fr]')}>
      <Card className="mb-0 p-0">
        <div className="flex items-center justify-between border-b border-line px-4 py-2">
          <h2 className="font-bold">Classifica</h2>
          <Button variant="ghost" size="sm" onClick={() => setExpanded(!expanded)}>
            {expanded ? 'Riduci' : 'Espandi'}
          </Button>
        </div>
        {expanded ? (
          <DriversTable session={session} drivers={drivers} />
        ) : (
          <CompactStandings session={session} drivers={drivers} />
        )}
      </Card>
      {/* Le schede delle auto (due in split-screen) si affiancano alla mappa se c'è spazio, altrimenti vanno sotto. */}
      <div className="flex flex-wrap items-start gap-4">
        {/* Nascosta e non smontata, così la mappa già scaricata resta pronta per quando si riduce. */}
        <TrackMapCard
          className={cx('mb-0 h-80 w-full md:size-88', expanded && 'hidden')}
          uid={session.uid}
          mapPoints={mapPoints ?? session.mapPoints}
          drivers={drivers}
          marshalZones={session.marshalZones}
          loadMap={loadMap}
        />
        {players.length === 2 && <DuelCard session={session} drivers={drivers} className="mb-0" />}
        {players.map((car) => (
          <PlayerCarCard
            key={car.controller}
            car={car}
            playerName={drivers.find((d) => d.carIndex === car.carIndex)?.playerName}
            pedals={pedals.filter((p) => p.controller === car.controller)}
            // Il gioco calcola il rientro dai box solo per il giocatore principale.
            pitRejoin={session.isRace && !finished && car.controller === 1 ? session.pitRejoin : 0}
            className="mb-0"
          />
        ))}
        <RaceControlCard events={events} drivers={drivers} className="mb-0 w-full" />
      </div>
    </div>
  );
}

// indicator sostituisce il pallino verde della diretta (il replay mostra il suo);
// actions sono pulsanti da mostrare in fondo a destra.
export function SessionCard({ live, session: s, indicator, actions }: {
  live: LiveSnapshot;
  session: LiveSession;
  indicator?: ReactNode;
  actions?: ReactNode;
}) {
  let progress = '';
  if (live.finished) progress = 'Finita';
  else if (s.isRace && s.totalLaps > 0) progress = `Giro ${Math.max(s.currentLap, 1)}/${s.totalLaps}`;
  else if (s.timeLeft > 0) progress = `${formatClock(s.timeLeft)} rimanenti`;

  const facts = [
    WEATHER[s.weather],
    `Pista ${s.trackTemp}°C`,
    `Aria ${s.airTemp}°C`,
  ].filter(Boolean);
  const yellow = s.marshalZones.some((z) => z.flag === 3);
  // Le previsioni oltre la fine della sessione non servono.
  const forecast = live.finished ? [] : s.forecast.filter((f) => !s.timeLeft || f.offset * 60 <= s.timeLeft);

  return (
    <Card className="flex flex-wrap items-center justify-between gap-4">
      <div className="flex items-center gap-3">
        {indicator ?? <LiveDot active={live.status === 'live'} />}
        <div>
          <div className="text-xl font-bold">{s.track || 'Circuito sconosciuto'}</div>
          <div className="text-sm text-muted">{s.type}</div>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        {s.safetyCar > 0 && !live.finished && (
          <span className="rounded-md bg-sprint-bg px-2.5 py-1 text-sm font-bold text-sprint">
            {SAFETY_CAR[s.safetyCar]}
          </span>
        )}
        {yellow && !live.finished && (
          <span className="rounded-md bg-sprint-bg px-2.5 py-1 text-sm font-bold text-sprint">Bandiera gialla</span>
        )}
        {isPaused(live) && <span className="text-sm font-semibold text-muted">In pausa</span>}
        <span className="text-sm text-muted">{facts.join(' · ')}</span>
        {progress && <span className="text-lg font-extrabold tabular-nums">{progress}</span>}
        {actions}
      </div>
      {(forecast.length > 0 || (s.isRace && s.pitWindowIdeal > 0 && !live.finished)) && (
        <div className="flex w-full flex-wrap items-center gap-x-4 gap-y-1 border-t border-line pt-3 text-sm">
          {forecast.length > 0 && (
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span className="text-muted">Previsioni</span>
              {forecast.map((f) => (
                <span key={f.offset} title={`Pista ${f.trackTemp}°C · Aria ${f.airTemp}°C`}>
                  <span className="text-muted tabular-nums">+{f.offset}′</span> {WEATHER[f.weather]}
                  {f.rain > 0 && <span className="text-sky-400 tabular-nums"> {f.rain}%</span>}
                </span>
              ))}
            </span>
          )}
          {s.isRace && s.pitWindowIdeal > 0 && !live.finished && (
            <span>
              <span className="text-muted">Finestra pit stop</span>{' '}
              <span className="font-semibold tabular-nums">
                giri {s.pitWindowIdeal}–{s.pitWindowLatest}
              </span>
            </span>
          )}
        </div>
      )}
    </Card>
  );
}

function fastestLap(drivers: LiveDriver[]): number {
  const bestTimes = drivers.map((d) => d.bestLapMs).filter(Boolean);
  return bestTimes.length ? Math.min(...bestTimes) : 0;
}

// Distacco dal primo e dall'auto davanti. In gara è quello in pista; in prova e
// qualifica conta il miglior giro.
function driverGap(session: LiveSession, d: LiveDriver, ahead: LiveDriver | undefined, fastest: number): [string, string] {
  if (d.position === 1) return ['', ''];
  if (session.isRace) return [formatGap(d.gapToLeaderMs), formatGap(d.gapAheadMs)];
  if (!d.bestLapMs) return ['–', '–'];
  return [
    formatGap(d.bestLapMs - fastest),
    ahead?.bestLapMs ? formatGap(d.bestLapMs - ahead.bestLapMs) : '–',
  ];
}

// Versione ridotta della classifica: posizione, pilota e distacco dal primo.
function CompactStandings({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  const fastest = fastestLap(drivers);
  const compact = 'px-2 py-1.5';

  return (
    <table className="w-full">
      <tbody>
        {drivers.map((d, i) => {
          const [toLeader, toAhead] = driverGap(session, d, drivers[i - 1], fastest);
          // Accanto al primo: il giro in gara, il miglior tempo in prova e qualifica.
          const leaderInfo = session.isRace ? `Giro ${d.lap}` : formatLapTime(d.bestLapMs);
          return (
            <Tr key={d.carIndex}>
              <Td className={cx(rankClass(d.position), compact, 'w-8 pl-4 text-right')}>{d.position}</Td>
              <Td className={cx(compact, 'max-w-0 w-full', d.controller > 0 && 'font-bold')}>
                <span className="flex items-center gap-2">
                  <span className="h-4 w-1 shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
                  <span className="truncate" title={d.team}>{d.name}</span>
                  {d.controller > 0 && <PlayerTag driver={d} />}
                  {d.inPit && <span className="text-xs font-bold text-sprint">PIT</span>}
                </span>
              </Td>
              <Td num className={cx(compact, 'pr-4 text-sm')} title={toAhead && `Intervallo ${toAhead}`}>
                {d.position === 1 ? <span className="text-muted">{leaderInfo}</span> : toLeader}
              </Td>
            </Tr>
          );
        })}
      </tbody>
    </table>
  );
}

// Migliori settori della sessione: il minimo dei migliori personali.
function fastestSectors(drivers: LiveDriver[]): number[] {
  return [0, 1, 2].map((s) => {
    const times = drivers.map((d) => d.bestSectors[s]).filter(Boolean);
    return times.length ? Math.min(...times) : 0;
  });
}

function DriversTable({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  const fastest = fastestLap(drivers);
  const sectors = fastestSectors(drivers);

  return (
    <Table>
      <thead>
        <tr>
          <Th>Pos</Th>
          <Th>Pilota</Th>
          {session.isRace && <Th num>Giro</Th>}
          <Th num>Distacco</Th>
          <Th num>Intervallo</Th>
          <Th num>Ultimo giro</Th>
          <Th num title="Settori dell'ultimo giro: viola il migliore della sessione, verde il migliore personale">
            Settori
          </Th>
          <Th num>Miglior giro</Th>
          <Th>Gomme</Th>
          <Th num>Soste</Th>
          <Th num title="Velocità massima allo speed trap">Vel. max</Th>
          <Th>Stato</Th>
        </tr>
      </thead>
      <tbody>
        {drivers.map((d, i) => {
          const [toLeader, toAhead] = driverGap(session, d, drivers[i - 1], fastest);
          return (
            <Tr key={d.carIndex}>
              <Td className={rankClass(d.position)}>
                {d.position}
                {session.isRace && <PositionsGained driver={d} />}
              </Td>
              <Td className={cx(d.controller > 0 && 'font-bold')}>
                <span className="mr-2 inline-block w-6 text-right text-sm text-muted tabular-nums">{d.raceNumber || ''}</span>
                {d.name} <TeamTag team={d.team || null} />
                {d.controller > 0 && <PlayerTag driver={d} className="ml-2" />}
              </Td>
              {session.isRace && <Td num>{d.lap}</Td>}
              <Td num>{toLeader}</Td>
              <Td num className="text-muted">{toAhead}</Td>
              <Td num>{formatLapTime(d.lastLapMs)}</Td>
              <Td num className="text-sm whitespace-nowrap">
                <Sectors driver={d} fastest={sectors} />
              </Td>
              <Td num className={cx(d.bestLapMs && d.bestLapMs === fastest && 'font-bold text-purple-400')}>
                {formatLapTime(d.bestLapMs)}
              </Td>
              <Td><Tyre compound={d.tyre} age={d.tyreAge} stints={d.stints} /></Td>
              <Td num>{d.pitStops}</Td>
              <Td num className="text-sm text-muted">{d.speedTrap ? Math.round(d.speedTrap) : '–'}</Td>
              <Td><DriverStatus driver={d} /></Td>
            </Tr>
          );
        })}
      </tbody>
    </Table>
  );
}

// Etichetta del giocatore: il controller (P1, P2) e, se scelto in "Chi guida?", il pilota.
function PlayerTag({ driver, className }: { driver: LiveDriver; className?: string }) {
  return (
    <span className={cx('inline-flex shrink-0 items-center gap-1 text-xs font-bold text-accent-text uppercase', className)}>
      {controllerLabel(driver.controller)}
      {driver.playerName && (
        <>
          {' · '}
          <ColorDot color={driver.playerColor} className="size-2" />
          <span style={{ color: driver.playerColor || undefined }}>{driver.playerName}</span>
        </>
      )}
    </span>
  );
}

// Posizioni guadagnate (▲) o perse (▼) rispetto alla griglia di partenza.
function PositionsGained({ driver: d }: { driver: LiveDriver }) {
  const gained = d.gridPosition ? d.gridPosition - d.position : 0;
  if (!gained) return null;
  return (
    <span
      className={cx('ml-1.5 text-xs font-semibold', gained > 0 ? 'text-green-400' : 'text-accent-text')}
      title={`Partito ${d.gridPosition}°`}
    >
      {gained > 0 ? '▲' : '▼'}{Math.abs(gained)}
    </span>
  );
}

// Settori dell'ultimo giro: viola il migliore della sessione, verde il migliore personale.
function Sectors({ driver: d, fastest }: { driver: LiveDriver; fastest: number[] }) {
  if (!d.lastSectors.some(Boolean)) return <span className="text-muted">–</span>;
  return (
    <span className="inline-flex gap-2">
      {d.lastSectors.map((ms, s) => (
        <span
          key={s}
          className={cx(
            ms && ms === fastest[s] ? 'font-bold text-purple-400' : ms && ms === d.bestSectors[s] ? 'text-green-400' : '',
          )}
        >
          {ms ? (ms / 1000).toFixed(3) : '–'}
        </span>
      ))}
    </span>
  );
}

function Tyre({ compound, age, stints = [] }: { compound: string; age: number; stints?: LiveDriver['stints'] }) {
  if (!compound) return <span className="text-muted">–</span>;
  // Le mescole usate prima di quella montata.
  const previous = stints.slice(0, -1).map((s) => s.tyre).filter(Boolean);
  return (
    <span className="flex items-center gap-2" title={`${age} giri`}>
      {previous.length > 0 && (
        <span className="text-xs text-muted" title="Gomme usate in precedenza">{previous.join(' › ')} ›</span>
      )}
      <span
        className={cx(
          'inline-flex size-6 items-center justify-center rounded-full border-2 text-xs font-extrabold',
          TYRE_CLASS[compound],
        )}
      >
        {compound}
      </span>
      <span className="text-sm text-muted tabular-nums">{age}</span>
    </span>
  );
}

function DriverStatus({ driver: d }: { driver: LiveDriver }) {
  const parts = [];
  if (d.inPit) parts.push(<span key="pit" className="font-bold text-sprint">PIT</span>);
  if (d.penaltiesSec) parts.push(<span key="pen" className="font-bold text-accent-text">+{d.penaltiesSec}s</span>);
  if (d.unserved) {
    parts.push(
      <span key="unserved" className="font-bold text-accent-text" title="Drive through o stop & go da scontare">
        {d.unserved} da scontare
      </span>,
    );
  }
  if (d.lapInvalid) parts.push(<span key="invalid" className="text-accent-text" title="Giro in corso annullato">Giro annullato</span>);
  if (d.warnings) {
    parts.push(
      <span key="warn" className="text-sprint" title={`Avvertimenti, di cui ${d.cornerCuts} per taglio di curva`}>
        ⚠ {d.warnings}
      </span>,
    );
  }
  // "In pista" è lo stato normale: si mostrano solo gli altri (arrivato, ritirato…).
  if (d.status !== 'In pista') parts.push(<span key="status" className="text-muted">{d.status}</span>);
  return <span className="flex gap-2 text-sm">{parts}</span>;
}
