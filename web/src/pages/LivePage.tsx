import { Fragment, useRef, useState, type ReactNode } from 'react';
import { formatClock, formatGap, formatLapTime } from '../lib/format';
import {
  controllerLabel, isLive, isLoading, isPaused, SAFETY_CAR, teamColor, telemetryProblem, TYRE_CLASS, WEATHER,
} from '../lib/live';
import type { LiveDriver, LiveSession, LiveSnapshot, PlayerCar, RaceEvent, TelemetryStatus, TrackMap } from '../lib/types';
import { useLive, usePedalHistory, type PedalSample } from '../lib/useLive';
import { usePositionAnimation } from '../lib/usePositionAnimation';
import { href } from '../lib/route';
import { DriverBadges } from '../components/DriverBadges';
import { DuelCard } from '../components/DuelCard';
import { LiveDot } from '../components/LiveBadge';
import { LineupSection, LineupSummary, lineupWaiting, useLineupSummary } from '../components/LineupSection';
import { PlayerCarCard } from '../components/PlayerCarCard';
import { PositionsGained } from '../components/PositionsGained';
import { RaceControlCard } from '../components/RaceControlCard';
import { TrackMapCard } from '../components/TrackMapCard';
import { Button, buttonClass, Card, ColorDot, cx, Empty, rankClass, Table, Td, Th, Tr } from '../components/ui';

// Classifica della sessione in corso, aggiornata in tempo reale dalla telemetria.
export function LivePage() {
  const live = useLive();
  const pedals = usePedalHistory();
  const [summary, setEditing] = useLineupSummary(live?.lineup?.stage);

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
        className={cx(live.lineup && lineupWaiting(live.lineup) && 'border-accent')}
        summary={live.lineup && summary && <LineupSummary lineup={live.lineup} onEdit={() => setEditing(true)} />}
        footer={live.lineup && !summary && <LineupSection lineup={live.lineup} onDone={() => setEditing(false)} />}
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
      {live.drivers.length ? (
        <LiveBoard session={live.session} drivers={live.drivers} players={live.players} events={live.events} finished={live.finished} pedals={pedals} sameData />
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
  const settings: { label: string; value: ReactNode }[] = [
    { label: 'Telemetria UDP', value: 'Attiva' },
    { label: 'Modalità broadcast UDP', value: 'Disattivata' },
    {
      label: 'Indirizzo IP UDP',
      value: (
        <>
          <span className="font-semibold">{primary ?? 'IP di questo PC'}</span>
          {others.length > 0 && <span className="text-muted"> (oppure {others.join(', ')})</span>}
          <span className="text-muted"> · 127.0.0.1 se il gioco gira su questo PC</span>
        </>
      ),
    },
    { label: 'Porta UDP', value: <span className="font-semibold">{t?.port || 20777}</span> },
    { label: 'Frequenza di invio UDP', value: '20 Hz' },
    { label: 'Formato UDP', value: '2025, oppure 2026 con il DLC 2026 Season Pack' },
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
        {settings.map(({ label, value }) => (
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
  // Le schede delle auto mostrano gli stessi dati per tutti i giocatori: quelli che il
  // gioco non invia per il giocatore 2 in split-screen si nascondono anche al giocatore 1.
  sameData?: boolean;
}

// Toglie a ogni auto i dati riservati se almeno una li ha nascosti dal gioco.
function equalizePlayers(players: PlayerCar[]): PlayerCar[] {
  if (!players.some((car) => car.restricted)) return players;
  return players.map((car) => (car.restricted ? car : { ...car, restricted: true, tyreSets: [], setup: null }));
}

// Classifica ridotta a sinistra di mappa e auto dei giocatori; estesa mostra
// tutti i dati e nasconde mappa e auto, lasciando duello e direzione gara. È
// usata anche dal replay, che passa la sua pianta (loadMap) e il suo storico dei pedali.
export function LiveBoard({ session, drivers, players, events, finished, pedals, loadMap, mapPoints, sameData }: LiveBoardProps) {
  const cars = sameData ? equalizePlayers(players) : players;
  // Il gioco calcola il rientro dai box solo per il giocatore 1: con sameData non si mostra se c'è anche il 2.
  const showPitRejoin = session.isRace && !finished && !(sameData && cars.some((car) => car.restricted));
  const [expanded, setExpanded] = useState(false);
  // Mappa standard o grande quanto tutto lo spazio a disposizione: si cambia cliccandola, e la scelta si ricorda.
  const [bigMap, setBigMap] = useState(() => localStorage.getItem('bigTrackMap') === '1');
  function toggleMap() {
    localStorage.setItem('bigTrackMap', bigMap ? '0' : '1');
    setBigMap(!bigMap);
  }
  // Classifica estesa con il duello: direzione gara gli sta accanto, alta uguale.
  const besideDuel = expanded && players.length === 2;
  // Con la mappa standard (e la classifica ridotta) mappa, duello e schede delle auto sono
  // celle di una griglia a colonne uguali: stessa larghezza, e alte uguali quelle sulla stessa
  // riga. Con la mappa grande la mappa prende tutta la larghezza e il duello va dopo le auto.
  const smallMap = !bigMap && !expanded;
  const duel = players.length === 2 && <DuelCard session={session} drivers={drivers} className="mb-0" />;

  return (
    // La mappa è quadrata, standard o grande: la classifica si allunga quanto serve per tutti i piloti.
    <div className={cx('mb-4 grid items-start gap-4', !expanded && 'md:grid-cols-[20rem_1fr]')}>
      {/* min-w-0: senza, la colonna della griglia si allarga quanto la tabella estesa
          e la fa uscire dalla pagina invece di farla scorrere. */}
      <Card className="mb-0 min-w-0 p-0">
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
      {/* min-w-0 tiene la colonna larga quanto lo spazio della pagina, così la mappa grande (w-full) si ferma lì
          invece di allargarla. */}
      <div
        className={cx(
          'min-w-0 gap-4',
          smallMap
            ? 'grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))]'
            : cx('flex flex-wrap', besideDuel ? 'items-stretch' : 'items-start'),
        )}
      >
        {/* Nascosta e non smontata, così la mappa già scaricata resta pronta per quando si riduce. */}
        <TrackMapCard
          className={cx('mb-0 aspect-square w-full', expanded && 'hidden')}
          onClick={toggleMap}
          title={bigMap ? 'Clicca per rimpicciolire la mappa' : 'Clicca per ingrandire la mappa'}
          uid={session.uid}
          mapPoints={mapPoints ?? session.mapPoints}
          drivers={drivers}
          marshalZones={session.marshalZones}
          sectors={session.sectors}
          loadMap={loadMap}
        />
        {smallMap && duel}
        {players.length > 0 && (
          // Le schede ridotte affiancate sono alte uguali. Se una è dettagliata (aria-expanded)
          // ognuna prende una riga intera, con la sua altezza.
          // Nascoste e non smontate, come la mappa: restano aperte o chiuse come le si era lasciate.
          <div
            className={cx(
              smallMap
                // Le schede diventano celle della griglia di fuori (contents).
                ? 'contents has-[[aria-expanded=true]]:*:col-span-full'
                : cx(
                  'grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-4 has-[[aria-expanded=true]]:grid-cols-1',
                  // Con due auto una riga intera, così stanno affiancate; con una sola sta accanto
                  // alla mappa, ma se è dettagliata va sotto su una riga intera.
                  players.length > 1 ? 'w-full' : 'min-w-72 flex-1 has-[[aria-expanded=true]]:basis-full',
                ),
              expanded && 'hidden',
            )}
          >
            {cars.map((car) => (
              <PlayerCarCard
                key={car.controller}
                car={car}
                playerName={drivers.find((d) => d.carIndex === car.carIndex)?.playerName}
                pedals={pedals.filter((p) => p.controller === car.controller)}
                pitRejoin={showPitRejoin && car.controller === 1 ? session.pitRejoin : 0}
                className="mb-0"
              />
            ))}
          </div>
        )}
        {!smallMap && duel}
        {/* Con la classifica estesa restano solo lei e il duello, sulla stessa riga e alti uguali. */}
        <RaceControlCard
          events={events}
          drivers={drivers}
          fill={besideDuel}
          className={cx('mb-0', expanded ? 'min-w-72 flex-1' : 'col-span-full w-full')}
        />
      </div>
    </div>
  );
}

// indicator sostituisce il pallino verde della diretta (il replay mostra il suo);
// actions sono pulsanti da mostrare in fondo a destra; summary va a sinistra nella
// riga delle previsioni (i piloti confermati), footer sotto a tutto (la scelta dei piloti).
export function SessionCard({ live, session: s, indicator, actions, summary, footer, className }: {
  live: LiveSnapshot;
  session: LiveSession;
  indicator?: ReactNode;
  actions?: ReactNode;
  summary?: ReactNode;
  footer?: ReactNode;
  className?: string;
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
  const pitWindow = s.isRace && s.pitWindowIdeal > 0 && !live.finished;

  return (
    <Card className={cx('flex flex-wrap items-center justify-between gap-4', className)}>
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
      {(forecast.length > 0 || pitWindow || summary) && (
        <div className="flex w-full flex-wrap items-center gap-x-4 gap-y-1 border-t border-line pt-3 text-sm">
          {summary}
          {(forecast.length > 0 || pitWindow) && (
            // Piloti a sinistra, meteo e pit stop a destra, divisi da una riga verticale.
            <span className={cx('ml-auto flex flex-wrap items-center gap-x-4 gap-y-1', summary && 'border-l border-line pl-4')}>
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
              {pitWindow && (
                <span>
                  <span className="text-muted">Finestra pit stop</span>{' '}
                  <span className="font-semibold tabular-nums">
                    giri {s.pitWindowIdeal}–{s.pitWindowLatest}
                  </span>
                </span>
              )}
            </span>
          )}
        </div>
      )}
      {footer}
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
  const body = useRef<HTMLTableSectionElement>(null);
  usePositionAnimation(body, drivers);

  return (
    <table className="w-full">
      <tbody ref={body}>
        {drivers.map((d, i) => {
          const [toLeader, toAhead] = driverGap(session, d, drivers[i - 1], fastest);
          // Accanto al primo: il giro in gara, il miglior tempo in prova e qualifica.
          const leaderInfo = session.isRace ? `Giro ${d.lap}` : formatLapTime(d.bestLapMs);
          return (
            <Tr key={d.carIndex} data-car={d.carIndex}>
              <Td className={cx(rankClass(d.position), compact, 'w-8 pl-4 text-right')}>{d.position}</Td>
              <Td className={cx(compact, 'max-w-0 w-full', d.controller > 0 && 'font-bold')}>
                <span className="flex items-center gap-2">
                  <span className="h-8 w-1 shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
                  {/* Nome sopra, giocatore e indicatori sotto: la riga c'è sempre, così
                      l'altezza non cambia quando un indicatore compare o sparisce (DRS). */}
                  <span className="flex min-w-0 flex-col leading-tight">
                    <span className="truncate" title={d.team}>{d.name}</span>
                    <span className="flex h-4 items-center gap-2 overflow-hidden">
                      {d.controller > 0 && <PlayerTag driver={d} />}
                      <DriverBadges driver={d} fastestLap={!!d.bestLapMs && d.bestLapMs === fastest} className="text-xs" />
                    </span>
                  </span>
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

// Classifica estesa. Per stare nella pagina senza scorrere: celle più strette,
// intestazioni brevi (il nome completo nel tooltip), la scuderia sotto il nome
// del pilota e i nomi che si accorciano se manca spazio.
function DriversTable({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  const fastest = fastestLap(drivers);
  const sectors = fastestSectors(drivers);
  const body = useRef<HTMLTableSectionElement>(null);
  usePositionAnimation(body, drivers);

  return (
    <Table className="[&_td]:px-2 [&_th]:px-2 [&_tr>:first-child]:pl-4 [&_tr>:last-child]:pr-4">
      <thead>
        <tr>
          <Th>Pos</Th>
          <Th>Pilota</Th>
          {session.isRace && <Th num>Giro</Th>}
          <Th num title="Distacco dal primo">Dist.</Th>
          <Th num title="Distacco dall'auto davanti">Int.</Th>
          <Th num title="Ultimo giro">Ultimo</Th>
          <Th num title="Settori dell'ultimo giro: viola il migliore della sessione, verde il migliore personale">
            Settori
          </Th>
          <Th num title="Miglior giro">Migliore</Th>
          <Th>Gomme</Th>
          <Th num title="Soste ai box">Soste</Th>
          <Th num title="Velocità massima allo speed trap">Vel.</Th>
          <Th>Stato</Th>
        </tr>
      </thead>
      <tbody ref={body}>
        {drivers.map((d, i) => {
          const [toLeader, toAhead] = driverGap(session, d, drivers[i - 1], fastest);
          return (
            <Tr key={d.carIndex} data-car={d.carIndex}>
              <Td className={rankClass(d.position)}>
                {d.position}
                {session.isRace && <PositionsGained driver={d} className="ml-1.5 text-xs" />}
              </Td>
              {/* max-w-0 e w-full: la colonna prende lo spazio che resta e il nome si accorcia. */}
              <Td className={cx('w-full max-w-0', d.controller > 0 && 'font-bold')}>
                <span className="flex items-center gap-2">
                  <span className="w-6 shrink-0 text-right text-sm text-muted tabular-nums">{d.raceNumber || ''}</span>
                  <span className="h-8 w-1 shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
                  {/* Nome sopra, scuderia sotto: la riga si alza ma la colonna resta stretta. */}
                  <span className="flex min-w-0 flex-col leading-tight">
                    <span className="flex items-center gap-2">
                      <span className="truncate">{d.name}</span>
                      {d.controller > 0 && <PlayerTag driver={d} />}
                      {/* Il giro più veloce è già in viola nella colonna Migliore. */}
                      <DriverBadges driver={d} fastestLap={false} className="text-xs" />
                    </span>
                    {d.team && <span className="truncate text-xs font-normal text-muted">{d.team}</span>}
                  </span>
                </span>
              </Td>
              {session.isRace && <Td num>{d.lap}</Td>}
              <Td num>{toLeader}</Td>
              <Td num className="text-muted">{toAhead}</Td>
              <Td num>{formatLapTime(d.lastLapMs)}</Td>
              <Td num className="text-sm">
                <Sectors driver={d} fastest={sectors} />
              </Td>
              <Td num className={cx(d.bestLapMs && d.bestLapMs === fastest && 'font-bold text-purple-400')}>
                {formatLapTime(d.bestLapMs)}
              </Td>
              <Td><Tyre compound={d.tyre} age={d.tyreAge} stints={d.stints} /></Td>
              <Td num>{d.pitStops}</Td>
              <Td num className="text-sm text-muted">{d.speedTrap ? Math.round(d.speedTrap) : '–'}</Td>
              {/* Lo stato può andare a capo: di solito è vuoto, a volte ha più voci. */}
              <Td className="min-w-24 whitespace-normal"><DriverStatus driver={d} /></Td>
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

// Settori dell'ultimo giro: viola il migliore della sessione, verde il migliore personale.
function Sectors({ driver: d, fastest }: { driver: LiveDriver; fastest: number[] }) {
  if (!d.lastSectors.some(Boolean)) return <span className="text-muted">–</span>;
  return (
    <span className="inline-flex gap-1.5">
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
  return <span className="flex flex-wrap gap-x-2 text-sm">{parts}</span>;
}
