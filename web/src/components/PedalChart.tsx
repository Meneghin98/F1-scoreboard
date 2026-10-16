import { useState, type PointerEvent } from 'react';
import { PEDAL_WINDOW_MS, type PedalSample } from '../lib/useLive';
import { cx } from './ui';

// Tra due campioni più distanti di così (ms) la linea si interrompe: il gioco era in pausa o nel menù.
const MAX_GAP = 1000;

// Colori dei pedali. Verde e rosso si confondono per chi non distingue i colori:
// per questo il freno ha anche l'area riempita, l'acceleratore solo la linea.
const PEDALS = [
  { key: 'throttle', label: 'Acceleratore', color: '#16a34a', filled: false },
  { key: 'brake', label: 'Freno', color: '#ef4444', filled: true },
] as const;

// Acceleratore e freno negli ultimi secondi, sovrapposti sulla stessa scala 0–100%:
// si vede subito quando si passa da un pedale all'altro.
// Passando sopra col mouse si leggono i valori di un istante.
export function PedalChart({ samples }: { samples: PedalSample[] }) {
  const [hover, setHover] = useState<PedalSample | null>(null);
  const last = samples.at(-1);
  // Il grafico finisce all'ultimo campione: se il gioco va in pausa si ferma anche lui.
  const end = last?.t ?? 0;
  const start = end - PEDAL_WINDOW_MS;
  const xOf = (t: number) => ((t - start) / PEDAL_WINDOW_MS) * 100;

  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const t = start + ((e.clientX - rect.left) / rect.width) * PEDAL_WINDOW_MS;
    let nearest: PedalSample | null = null;
    for (const s of samples) {
      if (!nearest || Math.abs(s.t - t) < Math.abs(nearest.t - t)) nearest = s;
    }
    setHover(nearest);
  };

  return (
    // Un clic sul grafico non deve chiudere la scheda che lo contiene.
    <div className="flex flex-col gap-1.5 text-sm" onClick={(e) => e.stopPropagation()}>
      <div className="flex gap-4">
        {PEDALS.map((p) => (
          <span key={p.key} className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm" style={{ background: p.color }} />
            <span className="text-muted">{p.label}</span>
            <span className="font-semibold tabular-nums">{last ? percent(last[p.key]) : '–'}</span>
          </span>
        ))}
      </div>

      <div
        className="relative h-28 cursor-crosshair rounded-md bg-surface-2/50"
        role="img"
        aria-label={`Acceleratore e freno negli ultimi ${PEDAL_WINDOW_MS / 1000} secondi`}
        onPointerMove={onPointerMove}
        onPointerLeave={() => setHover(null)}
      >
        {/* Il margine sopra e sotto evita che le linee a 0% e 100% finiscano sul bordo. */}
        <svg viewBox="0 -4 100 108" preserveAspectRatio="none" className="block size-full overflow-visible">
          <line x1={0} x2={100} y1={50} y2={50} stroke="var(--color-line)" strokeDasharray="2 3" vectorEffect="non-scaling-stroke" />
          {PEDALS.map((p) =>
            segments(samples).map((seg, i) => {
              const pts = seg.map((s) => `${xOf(s.t)},${100 - s[p.key] * 100}`);
              const first = xOf(seg[0].t), lastX = xOf(seg[seg.length - 1].t);
              return (
                <g key={`${p.key}-${i}`}>
                  {p.filled && <path d={`M${first},100 L${pts.join(' L')} L${lastX},100 Z`} fill={p.color} fillOpacity={0.18} />}
                  <path
                    d={`M${pts.join(' L')}`}
                    fill="none"
                    stroke={p.color}
                    strokeWidth={2}
                    strokeLinejoin="round"
                    vectorEffect="non-scaling-stroke"
                  />
                </g>
              );
            }),
          )}
        </svg>
        {hover && (
          <>
            <div
              className="pointer-events-none absolute inset-y-0 w-px bg-muted"
              style={{ left: `${xOf(hover.t)}%` }}
            />
            <div
              className={cx(
                'pointer-events-none absolute top-1 rounded-md border border-line bg-surface px-2 py-1 text-xs whitespace-nowrap shadow-lg',
                // Il riquadro sta dal lato opposto al bordo più vicino, così non esce dal grafico.
                xOf(hover.t) > 50 ? '-translate-x-full -ml-2' : 'ml-2',
              )}
              style={{ left: `${xOf(hover.t)}%` }}
            >
              <div className="text-muted tabular-nums">{secondsAgo(end - hover.t)}</div>
              {PEDALS.map((p) => (
                <div key={p.key} className="flex items-center gap-1.5">
                  <span className="size-2 rounded-sm" style={{ background: p.color }} />
                  <span className="text-muted">{p.label}</span>
                  <span className="ml-auto pl-2 font-semibold tabular-nums">{percent(hover[p.key])}</span>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      <div className="flex justify-between text-xs text-muted tabular-nums">
        <span>−{PEDAL_WINDOW_MS / 1000} s</span>
        <span>−{PEDAL_WINDOW_MS / 2000} s</span>
        <span>ora</span>
      </div>
    </div>
  );
}

// Divide i campioni nei tratti continui, separati dalle pause.
function segments(samples: PedalSample[]): PedalSample[][] {
  const out: PedalSample[][] = [];
  for (const s of samples) {
    const seg = out.at(-1);
    if (seg && s.t - seg[seg.length - 1].t <= MAX_GAP) seg.push(s);
    else out.push([s]);
  }
  return out;
}

function percent(v: number): string {
  return `${Math.round(v * 100)}%`;
}

function secondsAgo(ms: number): string {
  return ms < 50 ? 'ora' : `−${(ms / 1000).toLocaleString('it-IT', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} s`;
}
