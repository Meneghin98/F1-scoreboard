import { useEffect, useRef, useState } from 'react';
import { EVENT_KIND, EventText } from '../components/RaceControlCard';
import { ColorDot, cx } from '../components/ui';
import { formatClock, formatGap, formatLapTime } from '../lib/format';
import { controllerLabel, isLive, isLoading, SAFETY_CAR, teamColor } from '../lib/live';
import type { LiveDriver, LiveSession, LiveSnapshot, RaceEvent } from '../lib/types';
import { useLive } from '../lib/useLive';

// Per quanto resta sullo schermo un messaggio della direzione gara.
const EVENT_VISIBLE_MS = 8000;

// Sigle degli stati di chi non è più in gara, al posto del distacco.
const STATUS_SHORT: Record<string, string> = {
  'Non arrivato': 'DNF',
  Ritirato: 'RIT',
  Squalificato: 'SQ',
  'Non classificato': 'NC',
};

// Overlay per lo streaming, da aggiungere in OBS come sorgente "Browser" (vedi
// README): classifica a torre, circuito e giro, ultimo messaggio della
// direzione gara. Lo sfondo è trasparente, così sotto si vede il gioco, e non
// ci sono comandi: è solo da guardare. Senza una sessione in corso è vuoto.
export function OverlayPage() {
  const live = useLive();
  useEffect(() => {
    document.documentElement.classList.add('overlay'); // vedi index.css
    return () => document.documentElement.classList.remove('overlay');
  }, []);

  if (!isLive(live) || isLoading(live.session)) return null;
  return (
    // Le misure seguono l'altezza della sorgente in OBS: con 1920×1080 il testo è di circa 19 px.
    <div className="pointer-events-none fixed inset-0 overflow-hidden text-[1.75vh] text-white select-none">
      <Tower live={live} session={live.session} />
      <RaceMessage uid={live.session.uid} events={live.events} drivers={live.drivers} />
    </div>
  );
}

function Tower({ live, session: s }: { live: LiveSnapshot; session: LiveSession }) {
  let progress = '';
  if (live.finished) progress = 'Finale';
  else if (s.isRace && s.totalLaps > 0) progress = `Giro ${Math.max(s.currentLap, 1)}/${s.totalLaps}`;
  else if (s.timeLeft > 0) progress = formatClock(s.timeLeft);
  const fastest = Math.min(...live.drivers.map((d) => d.bestLapMs).filter(Boolean));

  return (
    <div className="absolute top-[4vh] left-[2vh] w-[16em] overflow-hidden rounded-lg bg-bg/90 shadow-lg shadow-black/40">
      <div className="flex items-baseline justify-between gap-[0.6em] border-b-[0.15em] border-accent px-[0.6em] py-[0.35em]">
        <span className="truncate font-extrabold uppercase">
          <span className="font-black text-accent italic">F1</span> {s.track}
        </span>
        <span className="shrink-0 font-extrabold tabular-nums">{progress}</span>
      </div>
      {s.safetyCar > 0 && !live.finished && (
        <div className="bg-sprint px-[0.6em] py-[0.15em] text-center font-extrabold text-black uppercase">
          {SAFETY_CAR[s.safetyCar]}
        </div>
      )}
      <div className="py-[0.2em]">
        {live.drivers.map((d) => <TowerRow key={d.carIndex} driver={d} session={s} fastest={fastest} />)}
      </div>
    </div>
  );
}

// Una riga della torre: posizione, scuderia, pilota e distacco. Le auto della
// IA hanno la sigla del pilota (le prime tre lettere, come in TV), quelle dei
// giocatori il nome del pilota del segnapunti con il suo colore.
function TowerRow({ driver: d, session, fastest }: { driver: LiveDriver; session: LiveSession; fastest: number }) {
  const player = d.controller > 0 || !d.ai;
  const name = d.playerName || (player ? d.name : d.name.slice(0, 3).toUpperCase());
  let gap: string;
  if (STATUS_SHORT[d.status]) gap = STATUS_SHORT[d.status];
  else if (d.inPit) gap = 'PIT';
  else if (session.isRace) gap = d.position === 1 ? 'Leader' : formatGap(d.gapToLeaderMs);
  else if (!d.bestLapMs) gap = '–';
  else gap = d.bestLapMs === fastest ? formatLapTime(d.bestLapMs) : formatGap(d.bestLapMs - fastest);

  return (
    <div className={cx('flex h-[1.45em] items-center gap-[0.45em] px-[0.6em]', player && 'bg-white/10')}>
      <span className="w-[1.3em] text-right font-extrabold tabular-nums">{d.position}</span>
      <span className="h-[0.95em] w-[0.22em] shrink-0 rounded-full" style={{ background: teamColor(d.team) }} />
      <span className={cx('flex min-w-0 flex-1 items-center gap-[0.35em]', player ? 'font-extrabold' : 'font-semibold')}>
        <span className="truncate">{name}</span>
        {d.controller > 0 && (
          <span className="flex shrink-0 items-center gap-[0.25em] text-[0.7em] font-extrabold text-accent-text">
            <ColorDot color={d.playerColor} className="size-[0.8em]" />
            {controllerLabel(d.controller)}
          </span>
        )}
      </span>
      <span className={cx('shrink-0 text-[0.85em] tabular-nums', gap === 'PIT' ? 'font-bold text-sprint' : 'text-white/80')}>
        {gap}
      </span>
    </div>
  );
}

// L'ultimo messaggio della direzione gara, per qualche secondo. Quelli arrivati
// prima di aprire l'overlay non si mostrano; dei sorpassi solo quelli dei giocatori.
function RaceMessage({ uid, events, drivers }: { uid: string; events: RaceEvent[]; drivers: LiveDriver[] }) {
  const players = new Set(drivers.filter((d) => d.controller > 0 || !d.ai).map((d) => d.carIndex));
  const latest = [...events].reverse().find((e) => e.kind !== 'overtake' || e.cars.some((car) => players.has(car)));
  const key = latest ? `${uid}:${latest.id}` : '';
  const seen = useRef(key);
  const [visible, setVisible] = useState<RaceEvent | null>(null);

  // Un messaggio nuovo sostituisce quello mostrato.
  useEffect(() => {
    if (key === seen.current) return;
    seen.current = key;
    if (latest) setVisible(latest);
  }, [key, latest]);
  useEffect(() => {
    if (!visible) return;
    const timer = setTimeout(() => setVisible(null), EVENT_VISIBLE_MS);
    return () => clearTimeout(timer);
  }, [visible]);

  if (!visible) return null;
  const kind = EVENT_KIND[visible.kind];
  return (
    <div
      key={visible.id}
      className="absolute bottom-[4vh] left-[2vh] flex max-w-[40em] items-baseline gap-[0.7em] rounded-lg bg-bg/90 px-[0.8em] py-[0.5em] shadow-lg shadow-black/40 transition-opacity duration-300 starting:opacity-0"
    >
      <span className={cx('shrink-0 text-[0.8em] font-extrabold uppercase', kind?.className)}>{kind?.label ?? visible.kind}</span>
      {/* Per le auto dei giocatori i nomi del segnapunti, come nella torre. */}
      <span className="font-semibold"><EventText event={visible} drivers={drivers} /></span>
    </div>
  );
}
