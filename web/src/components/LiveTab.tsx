import { formatClock, formatGap, formatLapTime } from '../lib/format';
import { isLive, SAFETY_CAR, TYRE_CLASS, WEATHER } from '../lib/live';
import type { LiveDriver, LiveSession, LiveSnapshot } from '../lib/types';
import { useLive } from '../lib/useLive';
import { LiveDot } from './LiveBadge';
import { Card, cx, Empty, rankClass, Table, TeamTag, Td, Th, Tr } from './ui';

// Classifica della sessione in corso, aggiornata in tempo reale dalla telemetria.
export function LiveTab() {
  const live = useLive();

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
      {live.drivers.length ? (
        <Card>
          <DriversTable session={live.session} drivers={live.drivers} />
        </Card>
      ) : (
        <Empty>In attesa dei dati dei piloti…</Empty>
      )}
    </>
  );
}

function SessionCard({ live, session: s }: { live: LiveSnapshot; session: LiveSession }) {
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
        <LiveDot active />
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

function DriversTable({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  // In gara il distacco è quello in pista; in prova e qualifica conta il miglior giro.
  const bestTimes = drivers.map((d) => d.bestLapMs).filter(Boolean);
  const fastest = bestTimes.length ? Math.min(...bestTimes) : 0;
  const gap = (d: LiveDriver, ahead: LiveDriver | undefined): [string, string] => {
    if (d.position === 1) return ['', ''];
    if (session.isRace) return [formatGap(d.gapToLeaderMs), formatGap(d.gapAheadMs)];
    if (!d.bestLapMs) return ['–', '–'];
    return [
      formatGap(d.bestLapMs - fastest),
      ahead?.bestLapMs ? formatGap(d.bestLapMs - ahead.bestLapMs) : '–',
    ];
  };

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
          const [toLeader, toAhead] = gap(d, drivers[i - 1]);
          return (
            <Tr key={d.carIndex}>
              <Td className={rankClass(d.position)}>{d.position}</Td>
              <Td className={cx(d.isPlayer && 'font-bold')}>
                <span className="mr-2 inline-block w-6 text-right text-sm text-muted tabular-nums">{d.raceNumber || ''}</span>
                {d.name} <TeamTag team={d.team || null} />
                {d.isPlayer && <span className="ml-2 text-xs font-bold text-accent-text uppercase">Tu</span>}
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
