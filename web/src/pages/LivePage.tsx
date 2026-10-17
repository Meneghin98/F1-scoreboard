import { useState, type ReactNode } from 'react';
import { formatClock, formatGap, formatLapTime } from '../lib/format';
import { isLive, SAFETY_CAR, teamColor, TYRE_CLASS, WEATHER } from '../lib/live';
import type { LiveDriver, LiveSession, LiveSnapshot, PlayerCar, TrackMap } from '../lib/types';
import { useLive, usePedalHistory, type PedalSample } from '../lib/useLive';
import { LiveDot } from '../components/LiveBadge';
import { LineupCard } from '../components/LineupCard';
import { PlayerCarCard } from '../components/PlayerCarCard';
import { TrackMapCard } from '../components/TrackMapCard';
import { Button, Card, cx, Empty, rankClass, Table, TeamTag, Td, Th, Tr } from '../components/ui';

// Classifica della sessione in corso, aggiornata in tempo reale dalla telemetria.
export function LivePage() {
  const live = useLive();
  const pedals = usePedalHistory();

  if (!live) return <Empty>Connessione al server in corso…</Empty>;
  if (!isLive(live)) {
    return (
      <Card>
        <Empty>Nessuna sessione in corso.</Empty>
      </Card>
    );
  }

  return (
    <>
      <SessionCard live={live} session={live.session} />
      {live.lineup && <LineupCard lineup={live.lineup} />}
      {live.drivers.length ? (
        <LiveBoard session={live.session} drivers={live.drivers} player={live.player} pedals={pedals} />
      ) : (
        <Empty>In attesa dei dati dei piloti…</Empty>
      )}
    </>
  );
}

interface LiveBoardProps {
  session: LiveSession;
  drivers: LiveDriver[];
  player: PlayerCar | null;
  pedals: PedalSample[];
  loadMap?: () => Promise<TrackMap>;
  mapPoints?: number; // punti della pianta, se diversi da quelli della sessione
}

// Classifica ridotta a sinistra di mappa e auto del giocatore; estesa mostra
// tutti i dati e nasconde la mappa. È usata anche dal replay, che passa la sua
// pianta (loadMap) e il suo storico dei pedali.
export function LiveBoard({ session, drivers, player, pedals, loadMap, mapPoints }: LiveBoardProps) {
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
      {/* La scheda dell'auto si affianca alla mappa se c'è spazio, altrimenti va sotto. */}
      <div className="flex flex-wrap items-start gap-4">
        {/* Nascosta e non smontata, così la mappa già scaricata resta pronta per quando si riduce. */}
        <TrackMapCard
          className={cx('mb-0 h-80 w-full md:size-88', expanded && 'hidden')}
          uid={session.uid}
          mapPoints={mapPoints ?? session.mapPoints}
          drivers={drivers}
          loadMap={loadMap}
        />
        {player && <PlayerCarCard car={player} pedals={pedals} className="mb-0" />}
      </div>
    </div>
  );
}

// indicator sostituisce il pallino verde della diretta (il replay mostra il suo).
export function SessionCard({ live, session: s, indicator }: { live: LiveSnapshot; session: LiveSession; indicator?: ReactNode }) {
  let progress = '';
  if (live.finished) progress = 'Finita';
  else if (s.isRace && s.totalLaps > 0) progress = `Giro ${Math.max(s.currentLap, 1)}/${s.totalLaps}`;
  else if (s.timeLeft > 0) progress = `${formatClock(s.timeLeft)} rimanenti`;

  const facts = [
    WEATHER[s.weather],
    `Pista ${s.trackTemp}°C`,
    `Aria ${s.airTemp}°C`,
  ].filter(Boolean);

  return (
    <Card className="flex flex-wrap items-center justify-between gap-4">
      <div className="flex items-center gap-3">
        {indicator ?? <LiveDot active />}
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
        {s.paused && !live.finished && <span className="text-sm font-semibold text-muted">In pausa</span>}
        <span className="text-sm text-muted">{facts.join(' · ')}</span>
        {progress && <span className="text-lg font-extrabold tabular-nums">{progress}</span>}
      </div>
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
              <Td className={cx(compact, 'max-w-0 w-full', d.isPlayer && 'font-bold')}>
                <span className="flex items-center gap-2">
                  <span className="h-4 w-1 shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
                  <span className="truncate" title={d.team}>{d.name}</span>
                  {d.isPlayer && <PlayerTag driver={d} />}
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

function DriversTable({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  const fastest = fastestLap(drivers);

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
          <Th num>Miglior giro</Th>
          <Th>Gomme</Th>
          <Th num>Soste</Th>
          <Th>Stato</Th>
        </tr>
      </thead>
      <tbody>
        {drivers.map((d, i) => {
          const [toLeader, toAhead] = driverGap(session, d, drivers[i - 1], fastest);
          return (
            <Tr key={d.carIndex}>
              <Td className={rankClass(d.position)}>{d.position}</Td>
              <Td className={cx(d.isPlayer && 'font-bold')}>
                <span className="mr-2 inline-block w-6 text-right text-sm text-muted tabular-nums">{d.raceNumber || ''}</span>
                {d.name} <TeamTag team={d.team || null} />
                {d.isPlayer && <PlayerTag driver={d} className="ml-2" />}
              </Td>
              {session.isRace && <Td num>{d.lap}</Td>}
              <Td num>{toLeader}</Td>
              <Td num className="text-muted">{toAhead}</Td>
              <Td num>{formatLapTime(d.lastLapMs)}</Td>
              <Td num className={cx(d.bestLapMs && d.bestLapMs === fastest && 'font-bold text-purple-400')}>
                {formatLapTime(d.bestLapMs)}
              </Td>
              <Td><Tyre compound={d.tyre} age={d.tyreAge} /></Td>
              <Td num>{d.pitStops}</Td>
              <Td><DriverStatus driver={d} /></Td>
            </Tr>
          );
        })}
      </tbody>
    </Table>
  );
}

// Etichetta del giocatore: il pilota scelto in "Chi guida?", altrimenti "Tu".
function PlayerTag({ driver, className }: { driver: LiveDriver; className?: string }) {
  return (
    <span className={cx('shrink-0 text-xs font-bold text-accent-text uppercase', className)}>
      {driver.playerName || 'Tu'}
    </span>
  );
}

function Tyre({ compound, age }: { compound: string; age: number }) {
  if (!compound) return <span className="text-muted">–</span>;
  return (
    <span className="flex items-center gap-2" title={`${age} giri`}>
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
  // "In pista" è lo stato normale: si mostrano solo gli altri (arrivato, ritirato…).
  if (d.status !== 'In pista') parts.push(<span key="status" className="text-muted">{d.status}</span>);
  return <span className="flex gap-2 text-sm">{parts}</span>;
}
