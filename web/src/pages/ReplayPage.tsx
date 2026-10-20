import { memo, useCallback, useEffect, useRef, useState, type PointerEvent } from 'react';
import { api } from '../lib/api';
import { formatElapsed } from '../lib/format';
import type { RaceEventKind, ReplayEvent, ReplayFrame, ReplayInfo, ReplayLap } from '../lib/types';
import { Button, Card, cx, Empty } from '../components/ui';
import { RaceAnalysis } from '../components/RaceAnalysis';
import { EVENT_KIND } from '../components/RaceControlCard';
import { LiveBoard, SessionCard } from './LivePage';

// Velocità di riproduzione proposte.
const SPEEDS = [0.5, 1, 2, 4, 8, 16];
// Di quanto si va avanti o indietro con i pulsanti e con le frecce.
const SKIP_MS = 10_000;
// Ogni quanto avanza il cursore durante la riproduzione.
const TICK_MS = 100;
// Il server ha uno stato ogni 250 ms, come gli aggiornamenti della pagina Live:
// non serve chiederlo più spesso.
const FRAME_MS = 250;

// Replay di una registrazione: la stessa vista della pagina Live, con in più
// la barra per fermare, riprendere, spostarsi e cambiare velocità.
export function ReplayPage({ sessionId }: { sessionId: number }) {
  const [state, setState] = useState<{ info?: ReplayInfo; error?: string }>({});

  // La prima apertura legge tutta la registrazione sul server: può richiedere qualche secondo.
  useEffect(() => {
    let cancelled = false;
    api.replay(sessionId).then(
      (info) => !cancelled && setState({ info }),
      (e: Error) => !cancelled && setState({ error: e.message }),
    );
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  if (state.error) return <Card><Empty>Impossibile aprire il replay: {state.error}</Empty></Card>;
  if (!state.info) return <Card><Empty>Preparazione del replay…</Empty></Card>;
  return <ReplayPlayer sessionId={sessionId} info={state.info} />;
}

function ReplayPlayer({ sessionId, info }: { sessionId: number; info: ReplayInfo }) {
  const duration = info.durationMs;
  const [pos, setPos] = useState(0); // istante mostrato, ms dall'inizio
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [frame, setFrame] = useState<ReplayFrame | null>(null);
  const [error, setError] = useState('');
  const dragging = useRef(false); // mentre si trascina la timeline il cursore non avanza da solo

  const seek = useCallback((ms: number) => setPos(Math.min(Math.max(ms, 0), duration)), [duration]);
  // Fine del giro indicato (0 = partenza): quando il primo inizia il successivo.
  const seekLap = useCallback(
    (lap: number) => seek(info.laps.find((l) => l.lap === lap + 1)?.atMs ?? duration),
    [info.laps, seek, duration],
  );

  function togglePlay() {
    if (playing) {
      setPlaying(false);
      return;
    }
    // Finito il replay, "riproduci" riparte dall'inizio.
    if (pos >= duration) setPos(0);
    setPlaying(true);
  }

  // Avanzamento del cursore: il tempo reale trascorso moltiplicato per la velocità.
  useEffect(() => {
    if (!playing) return;
    let last = performance.now();
    const timer = setInterval(() => {
      const now = performance.now();
      const step = (now - last) * speed;
      last = now;
      if (!dragging.current) setPos((p) => Math.min(p + step, duration));
    }, TICK_MS);
    return () => clearInterval(timer);
  }, [playing, speed, duration]);

  useEffect(() => {
    if (playing && pos >= duration) setPlaying(false);
  }, [playing, pos, duration]);

  // Scarica lo stato dell'istante mostrato. Una richiesta alla volta: se nel
  // frattempo il cursore si è spostato (riproduzione veloce, trascinamento) si
  // chiede subito l'ultimo istante, saltando quelli intermedi.
  const wanted = useRef(0);
  const loading = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const slot = Math.floor(pos / FRAME_MS);
  // biome-ignore lint/correctness/useExhaustiveDependencies: si scarica un fotogramma per slot (FRAME_MS), non a ogni cambio di pos; pos si legge com'è in quel momento.
  useEffect(() => {
    wanted.current = pos;
    if (loading.current) return;
    loading.current = true;
    void (async () => {
      try {
        for (;;) {
          const t = wanted.current;
          const f = await api.replayFrame(sessionId, t);
          if (!mounted.current) return;
          setFrame(f);
          setError('');
          if (wanted.current === t) return;
        }
      } catch (e) {
        if (!mounted.current) return;
        setError((e as Error).message);
        setPlaying(false);
      } finally {
        loading.current = false;
      }
    })();
  }, [slot, sessionId]);

  // Scorciatoie: spazio riproduce o ferma, le frecce spostano di SKIP_MS, Home e Fine vanno agli estremi.
  const onKey = useRef<(e: KeyboardEvent) => void>(() => {});
  onKey.current = (e) => {
    const target = e.target as HTMLElement;
    if (e.ctrlKey || e.metaKey || e.altKey || target.closest('input, select, textarea')) return;
    // Su un pulsante lo spazio lo preme: non deve anche fermare il replay.
    if (e.key === ' ' && target.closest('button, a, [role="button"]')) return;
    const actions: Record<string, () => void> = {
      ' ': togglePlay,
      ArrowLeft: () => seek(pos - SKIP_MS),
      ArrowRight: () => seek(pos + SKIP_MS),
      Home: () => seek(0),
      End: () => seek(duration),
    };
    const action = actions[e.key];
    if (!action) return;
    e.preventDefault();
    action();
  };
  useEffect(() => {
    const listener = (e: KeyboardEvent) => onKey.current(e);
    window.addEventListener('keydown', listener);
    return () => window.removeEventListener('keydown', listener);
  }, []);

  const lap = lapAt(info.laps, pos);
  const speedLabel = (s: number) => `${s.toLocaleString('it-IT')}×`;

  return (
    <>
      {/* Resta in vista anche scorrendo la classifica estesa. */}
      <Card className="sticky top-0 z-10 flex flex-col gap-3 shadow-lg shadow-bg/60">
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            // Larghezza fissa e su una riga: passando da "Riproduci" a "Pausa" i pulsanti accanto non si spostano.
            className="w-30 whitespace-nowrap"
            onClick={togglePlay}
            title={playing ? 'Ferma (spazio)' : 'Riproduci (spazio)'}
          >
            {playing ? '❚❚ Pausa' : '▶ Riproduci'}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => seek(pos - SKIP_MS)} title="Indietro di 10 secondi (←)">
            « 10 s
          </Button>
          <Button variant="ghost" size="sm" onClick={() => seek(pos + SKIP_MS)} title="Avanti di 10 secondi (→)">
            10 s »
          </Button>
          <span className="ml-2 text-sm tabular-nums">
            <span className="font-semibold">{formatElapsed(pos / 1000)}</span>
            <span className="text-muted"> / {formatElapsed(duration / 1000)}</span>
            {lap && <span className="ml-2 text-muted">Giro {lap.lap}</span>}
          </span>
          <div className="ml-auto flex items-center gap-1" role="group" aria-label="Velocità">
            <span className="mr-1 text-sm text-muted">Velocità</span>
            {SPEEDS.map((s) => (
              <Button
                key={s}
                variant="ghost"
                size="sm"
                aria-pressed={speed === s}
                className={cx('px-2 tabular-nums', speed === s && 'border-accent text-fg')}
                onClick={() => setSpeed(s)}
              >
                {speedLabel(s)}
              </Button>
            ))}
          </div>
        </div>
        <Timeline
          pos={pos}
          duration={duration}
          laps={info.laps}
          events={info.events}
          onSeek={seek}
          onDrag={(active) => (dragging.current = active)}
        />
        {error && <p className="text-sm text-accent-text">Errore nel caricamento del replay: {error}</p>}
      </Card>

      <ReplayBoard frame={frame} sessionId={sessionId} mapPoints={info.mapPoints} />
      {info.analysis && <MemoAnalysis analysis={info.analysis} onSeekLap={seekLap} />}
    </>
  );
}

// Messaggi segnati sulla timeline e loro colore (come l'etichetta nella direzione
// gara). Sorpassi e DRS sono troppo frequenti: restano solo nell'elenco.
const MARKER_CLASS: Partial<Record<RaceEventKind, string>> = {
  penalty: 'bg-accent-text',
  collision: 'bg-orange-400',
  fastestLap: 'bg-purple-400',
  retirement: 'bg-accent-text',
  safetyCar: 'bg-sprint',
  redFlag: 'bg-accent-text',
  flag: 'bg-fg',
  flashback: 'bg-muted',
};

// L'analisi non dipende dal cursore: non va ridisegnata 10 volte al secondo.
const MemoAnalysis = memo(RaceAnalysis);

// Giro del primo in classifica all'istante ms, null prima della partenza o fuori dalle gare.
function lapAt(laps: ReplayLap[], ms: number): ReplayLap | null {
  let found: ReplayLap | null = null;
  for (const l of laps) {
    if (l.atMs > ms) break;
    found = l;
  }
  return found;
}

interface TimelineProps {
  pos: number;
  duration: number;
  laps: ReplayLap[];
  events: ReplayEvent[];
  onSeek: (ms: number) => void;
  onDrag: (active: boolean) => void;
}

// Barra del tempo: un clic sposta il replay, trascinando si scorre. Le tacche
// segnano l'inizio dei giri del primo in classifica.
function Timeline({ pos, duration, laps, events, onSeek, onDrag }: TimelineProps) {
  const [hover, setHover] = useState<number | null>(null);
  const ratio = (ms: number) => (duration ? Math.min(Math.max(ms / duration, 0), 1) : 0);
  const timeAt = (e: PointerEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    return ((e.clientX - rect.left) / rect.width) * duration;
  };
  // Con tanti giri le etichette si toccherebbero: se ne mostra una ogni 5.
  const labelEvery = laps.length > 20 ? 5 : 1;
  const hoverLap = hover === null ? null : lapAt(laps, hover);
  const markers = events.filter((e) => MARKER_CLASS[e.kind]);
  // Passando sopra la barra si leggono i messaggi vicini al cursore.
  const nearby = hover === null ? [] : markers.filter((e) => Math.abs(e.atMs - hover) <= duration * 0.01).slice(0, 4);

  return (
    <div className="pt-1 pb-4">
      <div
        role="slider"
        tabIndex={0}
        aria-label="Posizione nel replay"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration / 1000)}
        aria-valuenow={Math.round(pos / 1000)}
        aria-valuetext={formatElapsed(pos / 1000)}
        className="group relative h-6 cursor-pointer touch-none rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId);
          onDrag(true);
          onSeek(timeAt(e));
        }}
        onPointerMove={(e) => {
          setHover(Math.min(Math.max(timeAt(e), 0), duration));
          if (e.currentTarget.hasPointerCapture(e.pointerId)) onSeek(timeAt(e));
        }}
        onPointerUp={() => onDrag(false)}
        onPointerCancel={() => onDrag(false)}
        onPointerLeave={() => setHover(null)}
      >
        <div className="absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-surface-2 transition-[height] group-hover:h-2">
          <div className="h-full rounded-full bg-accent" style={{ width: `${ratio(pos) * 100}%` }} />
        </div>
        {laps.map((l, i) => (
          // Il primo giro inizia con la partenza: la sua tacca coincide quasi con l'inizio della barra.
          <div key={l.lap} className="pointer-events-none absolute top-1/2" style={{ left: `${ratio(l.atMs) * 100}%` }}>
            <div className="h-3.5 w-px -translate-y-1/2 bg-muted/70" />
            {(i % labelEvery === 0 || i === laps.length - 1) && (
              <span className="absolute top-2.5 -translate-x-1/2 text-[10px] text-muted tabular-nums">{l.lap}</span>
            )}
          </div>
        ))}
        {/* Sopra la barra i messaggi della direzione gara (penalità, contatti, safety car…). */}
        {markers.map((e) => (
          <div
            key={e.id}
            className={cx('pointer-events-none absolute top-0 size-1.5 -translate-x-1/2 rounded-full', MARKER_CLASS[e.kind])}
            style={{ left: `${ratio(e.atMs) * 100}%` }}
          />
        ))}
        <div
          className="pointer-events-none absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-bg bg-fg"
          style={{ left: `${ratio(pos) * 100}%` }}
        />
        {hover !== null && (
          <div
            className="pointer-events-none absolute bottom-full mb-1 -translate-x-1/2 rounded-md border border-line bg-surface px-2 py-0.5 text-xs whitespace-nowrap tabular-nums shadow-lg"
            style={{ left: `${ratio(hover) * 100}%` }}
          >
            {formatElapsed(hover / 1000)}
            {hoverLap && <span className="text-muted"> · Giro {hoverLap.lap}</span>}
            {nearby.map((e) => (
              <div key={e.id}>
                <span className={cx('font-bold', EVENT_KIND[e.kind]?.className)}>{EVENT_KIND[e.kind]?.label}</span> {e.text}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// Lo stato della sessione all'istante mostrato. Separato dalla barra perché il
// cursore cambia 10 volte al secondo, lo stato solo quando arriva dal server.
const ReplayBoard = memo(function ReplayBoard({
  frame,
  sessionId,
  mapPoints,
}: {
  frame: ReplayFrame | null;
  sessionId: number;
  mapPoints: number;
}) {
  const loadMap = useCallback(() => api.replayTrack(sessionId), [sessionId]);

  if (!frame) return <Empty>Caricamento…</Empty>;
  const snap = frame.snapshot;
  if (!snap.session) return <Card><Empty>Il gioco non ha ancora inviato i dati della sessione.</Empty></Card>;

  return (
    <>
      <SessionCard
        live={snap}
        session={snap.session}
        indicator={
          <span className="rounded-md bg-surface-2 px-2 py-0.5 text-xs font-bold tracking-wider text-muted uppercase">
            Replay
          </span>
        }
      />
      {snap.status === 'paused' && (
        <p className="mb-4 text-sm text-muted">
          In questo tratto il gioco non inviava dati (menù, pausa o caricamento): si vede l'ultimo stato ricevuto.
        </p>
      )}
      {snap.drivers.length ? (
        <LiveBoard
          session={snap.session}
          drivers={snap.drivers}
          players={snap.players}
          events={snap.events}
          pedals={frame.pedals}
          loadMap={loadMap}
          mapPoints={mapPoints}
          finished={snap.finished}
        />
      ) : (
        <Empty>In attesa dei dati dei piloti…</Empty>
      )}
    </>
  );
});
