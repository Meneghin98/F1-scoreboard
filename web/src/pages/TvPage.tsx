import { useEffect, useRef, useState } from 'react';
import { DuelCard } from '../components/DuelCard';
import { LiveDot } from '../components/LiveBadge';
import { EVENT_KIND, EventText } from '../components/RaceControlCard';
import { TrackMapCard } from '../components/TrackMapCard';
import { Card, ColorDot, cx, rankClass } from '../components/ui';
import { formatClock, formatGap, formatLapTime } from '../lib/format';
import { controllerLabel, isLive, isLoading, SAFETY_CAR, teamColor, TYRE_CLASS } from '../lib/live';
import { href } from '../lib/route';
import type { LiveDriver, LiveSession, LiveSnapshot, RaceEvent } from '../lib/types';
import { useLive } from '../lib/useLive';
import { WaitingCard } from './LivePage';

// Modalità TV: la sessione in corso a schermo intero, con caratteri grandi da
// leggere dal divano. Niente header né menu; il mouse fa comparire i comandi.
export function TvPage() {
  const live = useLive();
  const [fullscreen, setFullscreen] = useState(!!document.fullscreenElement);
  useEffect(() => {
    const onChange = () => setFullscreen(!!document.fullscreenElement);
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  return (
    <div className="group flex h-screen flex-col gap-4 overflow-hidden p-5">
      {/* Comandi visibili solo passando con il mouse: in TV non devono distrarre. */}
      <div className="fixed top-3 right-3 z-10 flex gap-2 opacity-0 transition-opacity group-hover:opacity-100">
        {!fullscreen && (
          <button
            type="button"
            className="rounded-md border border-line bg-surface px-3 py-1.5 text-sm hover:text-fg"
            onClick={() => void document.documentElement.requestFullscreen()}
          >
            Schermo intero
          </button>
        )}
        <a href={href('live')} className="rounded-md border border-line bg-surface px-3 py-1.5 text-sm text-muted hover:text-fg">
          Esci
        </a>
      </div>

      {!live ? (
        <p className="m-auto text-2xl text-muted">Connessione al server in corso…</p>
      ) : !isLive(live) ? (
        <div className="m-auto w-full max-w-3xl"><WaitingCard telemetry={live.telemetry} /></div>
      ) : isLoading(live.session) ? (
        <p className="m-auto flex items-center gap-3 text-3xl font-bold"><LiveDot active /> Sessione in caricamento…</p>
      ) : (
        <>
          <TvHeader live={live} session={live.session} />
          <div className="grid min-h-0 flex-1 grid-cols-[minmax(30rem,2fr)_3fr] gap-4">
            <div className="flex min-h-0 flex-col gap-4">
              <TvStandings session={live.session} drivers={live.drivers} />
              <TvEvents events={live.events} drivers={live.drivers} />
            </div>
            <div className="flex min-h-0 flex-col gap-4">
              <TrackMapCard
                className="mb-0 min-h-0 flex-1"
                uid={live.session.uid}
                mapPoints={live.session.mapPoints}
                drivers={live.drivers}
                marshalZones={live.session.marshalZones}
              />
              {live.players.length === 2 && <DuelCard session={live.session} drivers={live.drivers} className="mb-0 flex-none" />}
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function TvHeader({ live, session: s }: { live: LiveSnapshot; session: LiveSession }) {
  let progress = '';
  if (live.finished) progress = 'Finita';
  else if (s.isRace && s.totalLaps > 0) progress = `Giro ${Math.max(s.currentLap, 1)}/${s.totalLaps}`;
  else if (s.timeLeft > 0) progress = formatClock(s.timeLeft);
  const yellow = s.marshalZones.some((z) => z.flag === 3);

  return (
    <div className="flex items-center justify-between gap-6 rounded-lg border border-line bg-surface px-6 py-3">
      <div className="flex items-center gap-4">
        <LiveDot active={live.status === 'live'} />
        <div>
          <div className="text-3xl font-extrabold">{s.track}</div>
          <div className="text-lg text-muted">{s.type}</div>
        </div>
      </div>
      <div className="flex items-center gap-4">
        {s.safetyCar > 0 && !live.finished && (
          <span className="rounded-md bg-sprint-bg px-4 py-1.5 text-2xl font-extrabold text-sprint">{SAFETY_CAR[s.safetyCar]}</span>
        )}
        {yellow && !live.finished && (
          <span className="rounded-md bg-sprint-bg px-4 py-1.5 text-2xl font-extrabold text-sprint">Bandiera gialla</span>
        )}
        {progress && <span className="text-5xl font-black tabular-nums">{progress}</span>}
      </div>
    </div>
  );
}

// Altezza minima di una riga della classifica, in em del suo testo: sotto si nascondono piloti.
const MIN_ROW_EM = 1.5;

// Se la classifica non ci sta tutta si tengono i giocatori, il primo e chi è
// più vicino ai giocatori (a pari distanza chi sta davanti).
function visibleDrivers(drivers: LiveDriver[], capacity: number): LiveDriver[] {
  if (drivers.length <= capacity) return drivers;
  const players = drivers.filter((d) => d.controller > 0).map((d) => d.position);
  const priority = (d: LiveDriver) => {
    if (d.controller > 0) return -2;
    if (d.position === 1) return -1;
    return players.length ? Math.min(...players.map((p) => Math.abs(p - d.position))) : d.position;
  };
  const kept = new Set(
    [...drivers]
      .sort((a, b) => priority(a) - priority(b) || a.position - b.position)
      .slice(0, Math.max(capacity, players.length))
      .map((d) => d.carIndex),
  );
  return drivers.filter((d) => kept.has(d.carIndex));
}

// Classifica a caratteri grandi: posizione, pilota, distacco e gomme.
function TvStandings({ session, drivers }: { session: LiveSession; drivers: LiveDriver[] }) {
  const fastest = Math.min(...drivers.map((d) => d.bestLapMs).filter(Boolean));
  // Quante righe ci stanno: l'altezza della scheda dipende dalla pagina, non dalle righe.
  const ref = useRef<HTMLDivElement>(null);
  const [capacity, setCapacity] = useState(Number.POSITIVE_INFINITY);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new ResizeObserver(() => {
      const row = el.firstElementChild;
      if (!row) return;
      const style = getComputedStyle(el);
      const height = el.clientHeight - Number.parseFloat(style.paddingTop) - Number.parseFloat(style.paddingBottom);
      const rowHeight = Number.parseFloat(getComputedStyle(row).fontSize) * MIN_ROW_EM;
      setCapacity(Math.max(1, Math.floor(height / rowHeight)));
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const shown = visibleDrivers(drivers, capacity);

  return (
    <Card ref={ref} className="mb-0 flex min-h-0 flex-1 flex-col p-2">
      {shown.map((d, i) => {
        const gap = d.position === 1
          ? (session.isRace ? `Giro ${d.lap}` : formatLapTime(d.bestLapMs))
          : session.isRace ? formatGap(d.gapToLeaderMs) : d.bestLapMs ? formatGap(d.bestLapMs - fastest) : '–';
        return (
          <div
            key={d.carIndex}
            className={cx(
              'flex min-h-0 flex-1 items-center gap-3 rounded-md px-3 text-[clamp(0.9rem,2.1vh,1.6rem)]',
              d.controller > 0 && 'bg-surface-2',
              // Piloti nascosti tra questa riga e la precedente.
              i > 0 && d.position !== shown[i - 1].position + 1 && 'rounded-t-none border-t border-dashed border-line',
            )}
          >
            <span className={cx(rankClass(d.position), 'w-[2.2em] text-right')}>{d.position}</span>
            <span className="h-[1.1em] w-1.5 shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
            <span className={cx('flex min-w-0 flex-1 items-center gap-2', d.controller > 0 && 'font-bold')}>
              {/* Con un giocatore scelto anche il pilota del gioco, che si accorcia per primo. */}
              <span className="min-w-0 truncate">{d.playerName || d.name}</span>
              {d.playerName && d.name && (
                <span className="min-w-0 shrink-[10] truncate text-[0.75em] font-normal text-muted">{d.name}</span>
              )}
              {d.controller > 0 && (
                <span className="inline-flex shrink-0 items-center gap-1 text-[0.7em] font-bold text-accent-text">
                  <ColorDot color={d.playerColor} />
                  {controllerLabel(d.controller)}
                </span>
              )}
              {d.inPit && <span className="shrink-0 text-[0.7em] font-bold text-sprint">PIT</span>}
            </span>
            <span className="tabular-nums text-muted">{gap}</span>
            {d.tyre && (
              <span
                className={cx(
                  'inline-flex size-[1.4em] shrink-0 items-center justify-center rounded-full border-2 text-[0.7em] font-extrabold',
                  TYRE_CLASS[d.tyre],
                )}
              >
                {d.tyre}
              </span>
            )}
          </div>
        );
      })}
    </Card>
  );
}

// Gli ultimi messaggi della direzione gara, dal più recente.
function TvEvents({ events, drivers }: { events: RaceEvent[]; drivers: LiveDriver[] }) {
  const last = events.slice(-4).reverse();
  if (!last.length) return null;
  return (
    <Card className="mb-0 flex flex-none flex-col gap-1.5 text-lg">
      {last.map((e) => (
        <div key={e.id} className="flex gap-3">
          <span className={cx('w-32 shrink-0 text-sm leading-7 font-bold uppercase', EVENT_KIND[e.kind]?.className)}>
            {EVENT_KIND[e.kind]?.label ?? e.kind}
          </span>
          <span className="truncate"><EventText event={e} drivers={drivers} /></span>
        </div>
      ))}
    </Card>
  );
}
