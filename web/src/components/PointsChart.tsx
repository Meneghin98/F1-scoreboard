import { useState, type PointerEvent } from 'react';
import type { Race, Standing } from '../lib/types';
import { Card } from './ui';

const WIDTH = 800;
const HEIGHT = 300;
const PAD = { top: 16, right: 130, bottom: 28, left: 40 };

// Andamento dei punti in classifica gara dopo gara: una linea per pilota, nel suo colore.
export function PointsChart({ races, standings, labels }: { races: Race[]; standings: Standing[]; labels: Map<number, string> }) {
  const [hover, setHover] = useState<number | null>(null); // indice della gara sotto il mouse
  if (races.length < 2) return null;

  // Punti dopo ogni gara (il primo valore è la partenza, 0 per tutti).
  const series = standings.map((s) => {
    let total = 0;
    return { standing: s, points: [0, ...races.map((r) => (total += s.perRace[r.id]?.points ?? 0))] };
  });
  const maxPoints = Math.max(1, ...series.map((s) => s.points.at(-1) ?? 0));
  const plotW = WIDTH - PAD.left - PAD.right;
  const plotH = HEIGHT - PAD.top - PAD.bottom;
  const x = (i: number) => PAD.left + (i / races.length) * plotW;
  const y = (points: number) => PAD.top + plotH - (points / maxPoints) * plotH;
  const ticks = niceTicks(maxPoints);

  function onMove(e: PointerEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const svgX = ((e.clientX - rect.left) / rect.width) * WIDTH;
    const i = Math.round(((svgX - PAD.left) / plotW) * races.length);
    setHover(i >= 1 && i <= races.length ? i : null);
  }

  // Le etichette a destra, nell'ordine della classifica, non si sovrappongono: ognuna almeno 14 px sotto la precedente.
  const labelsY: number[] = [];
  for (const s of series) {
    const wanted = y(s.points.at(-1) ?? 0);
    labelsY.push(labelsY.length ? Math.max(wanted, labelsY[labelsY.length - 1] + 14) : wanted);
  }

  const hovered = hover === null ? null : races[hover - 1];

  return (
    <Card>
      <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-bold">Andamento della classifica</h2>
        <span className="text-sm text-muted">
          {hovered ? `Dopo ${labels.get(hovered.id)} · ${hovered.track}` : 'Passa sul grafico per i punti dopo ogni gara'}
        </span>
      </div>
      <div className="relative">
        <svg
          viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
          className="block w-full"
          role="img"
          aria-label="Punti in classifica dopo ogni gara"
          onPointerMove={onMove}
          onPointerLeave={() => setHover(null)}
        >
          {ticks.map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={WIDTH - PAD.right} y1={y(t)} y2={y(t)} stroke="var(--color-line)" strokeWidth={1} />
              <text x={PAD.left - 8} y={y(t)} textAnchor="end" dominantBaseline="central" fontSize={11} fill="var(--color-muted)">
                {t}
              </text>
            </g>
          ))}
          {races.map((r, i) => (
            <text key={r.id} x={x(i + 1)} y={HEIGHT - 8} textAnchor="middle" fontSize={11} fill="var(--color-muted)">
              {labels.get(r.id)}
            </text>
          ))}
          {hover !== null && (
            <line x1={x(hover)} x2={x(hover)} y1={PAD.top} y2={PAD.top + plotH} stroke="var(--color-muted)" strokeDasharray="3 3" />
          )}
          {/* Prima gli ultimi in classifica, così le linee dei primi restano sopra. */}
          {[...series].reverse().map(({ standing: s, points }) => (
            <g key={s.player_id}>
              <path
                d={points.map((p, i) => `${i ? 'L' : 'M'}${x(i)} ${y(p)}`).join('')}
                fill="none"
                stroke={s.color || 'var(--color-muted)'}
                strokeWidth={2.5}
                strokeLinejoin="round"
                strokeLinecap="round"
              />
              {hover !== null && <circle cx={x(hover)} cy={y(points[hover])} r={4} fill={s.color || 'var(--color-muted)'} />}
            </g>
          ))}
          {series.map(({ standing: s }, i) => (
            <text
              key={s.player_id}
              x={WIDTH - PAD.right + 10}
              y={labelsY[i]}
              dominantBaseline="central"
              fontSize={12}
              fontWeight={700}
              fill={s.color || 'var(--color-fg)'}
            >
              {s.name} {hover === null ? s.points : series[i].points[hover]}
            </text>
          ))}
        </svg>
      </div>
    </Card>
  );
}

// Valori della griglia orizzontale: circa quattro, a passi "tondi" (5, 10, 25, 50…).
function niceTicks(max: number): number[] {
  const steps = [5, 10, 25, 50, 100, 200, 250, 500, 1000];
  const step = steps.find((s) => max / s <= 5) ?? 1000;
  const ticks = [];
  for (let t = 0; t <= max; t += step) ticks.push(t);
  return ticks;
}
