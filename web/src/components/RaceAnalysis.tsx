import { useState, type MouseEvent } from 'react';
import { formatLapTime } from '../lib/format';
import { controllerLabel } from '../lib/live';
import type { Analysis, AnalysisCar } from '../lib/types';
import { Card, cx, fieldSmClass, Table, Td, Th, Tr } from './ui';

// Colori delle linee dei giocatori nel grafico delle posizioni, in ordine fisso
// (verificati per daltonismo sullo sfondo delle card). Le auto della IA sono
// grigie: fanno da contesto.
const HUMAN_COLORS = ['#3987e5', '#d95926', '#199e70', '#c98500'];
const AI_COLOR = '#4a4a5e';

// Riempimento delle mescole nella strategia gomme: la lettera dentro la barra
// dice la mescola anche senza distinguere i colori.
const TYRE_FILL: Record<string, string> = {
  S: 'bg-red-500',
  M: 'bg-yellow-400',
  H: 'bg-gray-200',
  I: 'bg-green-500',
  W: 'bg-blue-500',
};
const TYRE_NAME: Record<string, string> = { S: 'Morbide', M: 'Medie', H: 'Dure', I: 'Intermedie', W: 'Bagnato' };

// Riassunto della sessione sotto il replay. onSeekLap porta il replay alla fine del giro indicato (0 = partenza).
export function RaceAnalysis({ analysis, onSeekLap }: { analysis: Analysis; onSeekLap: (lap: number) => void }) {
  const humans = analysis.cars.filter((c) => c.human);
  // Il colore del pilota del segnapunti, se scelto; altrimenti uno fisso per giocatore.
  const colorOf = new Map(humans.map((c, i) => [c.carIndex, c.playerColor || HUMAN_COLORS[i % HUMAN_COLORS.length]]));

  return (
    <>
      {analysis.laps > 1 && <PositionsCard analysis={analysis} colorOf={colorOf} onSeekLap={onSeekLap} />}
      <StrategyCard cars={analysis.cars} />
      <LapTimesCard cars={analysis.cars} />
    </>
  );
}

function carLabel(c: AnalysisCar): string {
  return c.playerName ? `${c.name} (${c.playerName})` : c.name;
}

// --- Posizioni giro per giro -----------------------------------------------

const ROW = 18; // altezza di una posizione
const PAD = { top: 24, right: 150, bottom: 8, left: 32 };
const WIDTH = 800;

function PositionsCard({
  analysis,
  colorOf,
  onSeekLap,
}: {
  analysis: Analysis;
  colorOf: Map<number, string>;
  onSeekLap: (lap: number) => void;
}) {
  const [hover, setHover] = useState<number | null>(null); // auto sotto il mouse
  const [table, setTable] = useState(false);
  const { cars, laps } = analysis;
  const rows = cars.length;
  const height = PAD.top + rows * ROW + PAD.bottom;
  const plotW = WIDTH - PAD.left - PAD.right;
  const x = (lap: number) => PAD.left + (laps > 1 ? (lap / (laps - 1)) * plotW : 0);
  const y = (pos: number) => PAD.top + (pos - 0.5) * ROW;
  // Le linee dei giocatori si disegnano sopra quelle della IA.
  const ordered = [...cars].sort((a, b) => Number(colorOf.has(a.carIndex)) - Number(colorOf.has(b.carIndex)));
  const hovered = cars.find((c) => c.carIndex === hover);

  // Un clic sul grafico porta il replay al giro più vicino.
  function seek(e: MouseEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const svgX = ((e.clientX - rect.left) / rect.width) * WIDTH;
    const lap = Math.round(((svgX - PAD.left) / plotW) * (laps - 1));
    if (lap >= 0 && lap < laps) onSeekLap(lap);
  }

  return (
    <Card>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-bold">Posizioni giro per giro</h2>
        <div className="flex flex-wrap items-center gap-4 text-sm">
          {cars.filter((c) => colorOf.has(c.carIndex)).map((c) => (
            <span key={c.carIndex} className="flex items-center gap-1.5">
              <span className="w-4 rounded-full" style={{ background: colorOf.get(c.carIndex), height: 3 }} />
              {carLabel(c)}
            </span>
          ))}
          <span className="flex items-center gap-1.5 text-muted">
            <span className="w-4 rounded-full" style={{ background: AI_COLOR, height: 2 }} />
            Altri piloti
          </span>
          <button type="button" className="text-muted underline hover:text-fg" onClick={() => setTable(!table)}>
            {table ? 'Mostra grafico' : 'Mostra tabella'}
          </button>
        </div>
      </div>

      {table ? (
        <PositionsTable analysis={analysis} />
      ) : (
        <div className="relative">
          <svg
            viewBox={`0 0 ${WIDTH} ${height}`}
            className="block w-full cursor-pointer"
            role="img"
            aria-label="Posizione di ogni pilota alla fine di ogni giro"
            onClick={seek}
          >
            {/* Griglia leggera: una linea per giro, etichette sopra. */}
            {Array.from({ length: laps }, (_, lap) => (
              <g key={lap}>
                <line x1={x(lap)} x2={x(lap)} y1={PAD.top - 4} y2={height - PAD.bottom} stroke="var(--color-line)" strokeWidth={1} />
                <text x={x(lap)} y={PAD.top - 10} textAnchor="middle" fontSize={11} fill="var(--color-muted)">
                  {lap === 0 ? 'Via' : lap}
                </text>
              </g>
            ))}
            {Array.from({ length: rows }, (_, i) => (
              <text key={i} x={PAD.left - 10} y={y(i + 1)} textAnchor="end" dominantBaseline="central" fontSize={11} fill="var(--color-muted)">
                {i + 1}
              </text>
            ))}

            {ordered.map((c) => {
              const color = colorOf.get(c.carIndex);
              const active = hover === c.carIndex;
              const d = linePath(c.positions, x, y);
              return (
                <g key={c.carIndex} onPointerEnter={() => setHover(c.carIndex)} onPointerLeave={() => setHover(null)}>
                  <path
                    d={d}
                    fill="none"
                    stroke={active ? 'var(--color-fg)' : color ?? AI_COLOR}
                    strokeWidth={color || active ? 2.5 : 1.5}
                    strokeLinejoin="round"
                    strokeLinecap="round"
                  />
                  {color &&
                    c.positions.map((p, lap) =>
                      p ? <circle key={lap} cx={x(lap)} cy={y(p)} r={4} fill={color} stroke="var(--color-surface)" strokeWidth={2} /> : null,
                    )}
                  {/* Area sensibile più larga della linea. */}
                  <path d={d} fill="none" stroke="transparent" strokeWidth={12} />
                </g>
              );
            })}

            {/* A destra i piloti nell'ordine d'arrivo. */}
            {cars.map((c, i) => (
              <text
                key={c.carIndex}
                x={WIDTH - PAD.right + 10}
                y={y(i + 1)}
                dominantBaseline="central"
                fontSize={11}
                fontWeight={colorOf.has(c.carIndex) || hover === c.carIndex ? 700 : 400}
                fill={colorOf.has(c.carIndex) || hover === c.carIndex ? 'var(--color-fg)' : 'var(--color-muted)'}
                onPointerEnter={() => setHover(c.carIndex)}
                onPointerLeave={() => setHover(null)}
              >
                {c.controller > 0 ? `${controllerLabel(c.controller)} ` : ''}
                {c.name}
              </text>
            ))}
          </svg>
          {hovered && (
            <div className="pointer-events-none absolute top-0 left-1/2 -translate-x-1/2 rounded-md border border-line bg-surface px-2 py-1 text-xs shadow-lg">
              <span className="font-semibold">{carLabel(hovered)}</span>
              <span className="text-muted"> · </span>
              <span className="tabular-nums">
                partito {hovered.positions[0] || '–'}°, {hovered.status === 'Arrivato' ? `arrivato ${hovered.position}°` : hovered.status.toLowerCase()}
              </span>
            </div>
          )}
          <p className="mt-1 text-xs text-muted">Clic su un giro per andarci nel replay.</p>
        </div>
      )}
    </Card>
  );
}

// Percorso della linea di un pilota; si interrompe dove la posizione manca (0).
function linePath(positions: number[], x: (lap: number) => number, y: (pos: number) => number): string {
  let d = '';
  let drawing = false;
  positions.forEach((p, lap) => {
    if (!p) {
      drawing = false;
      return;
    }
    d += `${drawing ? 'L' : 'M'}${x(lap)} ${y(p)}`;
    drawing = true;
  });
  return d;
}

function PositionsTable({ analysis }: { analysis: Analysis }) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <thead>
          <tr>
            <Th>Pilota</Th>
            {Array.from({ length: analysis.laps }, (_, lap) => (
              <Th key={lap} num>{lap === 0 ? 'Via' : lap}</Th>
            ))}
          </tr>
        </thead>
        <tbody>
          {analysis.cars.map((c) => (
            <Tr key={c.carIndex}>
              <Td className={cx('whitespace-nowrap', c.human && 'font-bold')}>{carLabel(c)}</Td>
              {c.positions.map((p, lap) => (
                <Td key={lap} num>{p || '–'}</Td>
              ))}
            </Tr>
          ))}
        </tbody>
      </Table>
    </div>
  );
}

// --- Strategia gomme ---------------------------------------------------------

function StrategyCard({ cars }: { cars: AnalysisCar[] }) {
  // Lo stint in corso (endLap 0) arriva fino all'ultimo giro completato.
  const stintsOf = (c: AnalysisCar) => {
    let start = 1;
    return c.stints.map((s) => {
      const end = s.endLap || Math.max(c.laps.length, start);
      const stint = { tyre: s.tyre, start, end };
      start = end + 1;
      return stint;
    });
  };
  const total = Math.max(1, ...cars.flatMap((c) => stintsOf(c).map((s) => s.end)));
  // Senza soste ai box (gare brevi) il grafico sarebbe una fila di barre uguali.
  if (!cars.some((c) => c.stints.length > 1)) return null;

  return (
    <Card>
      <h2 className="mb-3 font-bold">Strategia gomme</h2>
      <div className="flex flex-col gap-1 text-sm">
        {cars.map((c) => (
          <div key={c.carIndex} className="flex items-center gap-3">
            <span className={cx('w-40 shrink-0 truncate', c.human ? 'font-bold' : 'text-muted')} title={carLabel(c)}>
              {c.position}. {c.name}
            </span>
            <div className="flex h-5 flex-1 gap-0.5">
              {stintsOf(c).map((s, i) => {
                const laps = s.end - s.start + 1;
                return (
                  <div
                    key={i}
                    className={cx(
                      'flex items-center justify-center rounded-sm text-[11px] font-extrabold text-bg',
                      TYRE_FILL[s.tyre] ?? 'bg-muted',
                    )}
                    style={{ width: `${(laps / total) * 100}%` }}
                    title={`${TYRE_NAME[s.tyre] ?? 'Gomme'}: giri ${s.start}–${s.end} (${laps} ${laps === 1 ? 'giro' : 'giri'})`}
                  >
                    {s.tyre}
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

// --- Tempi sul giro ------------------------------------------------------------

function LapTimesCard({ cars }: { cars: AnalysisCar[] }) {
  const withLaps = cars.filter((c) => c.laps.length);
  const [selected, setSelected] = useState(() => (withLaps.find((c) => c.human) ?? withLaps[0])?.carIndex);
  const car = withLaps.find((c) => c.carIndex === selected) ?? withLaps[0];
  if (!car) return null;

  // Migliori della sessione (giri validi di tutti) e del pilota scelto.
  const valid = withLaps.flatMap((c) => c.laps.filter((l) => l.valid));
  const best = (values: number[]) => (values.length ? Math.min(...values) : 0);
  const sessionBest = best(valid.map((l) => l.timeMs));
  const sessionSectors = [0, 1, 2].map((s) => best(valid.map((l) => l.sectors[s]).filter(Boolean)));
  const ownValid = car.laps.filter((l) => l.valid);
  const ownBest = best(ownValid.map((l) => l.timeMs));
  const ownSectors = [0, 1, 2].map((s) => best(ownValid.map((l) => l.sectors[s]).filter(Boolean)));
  const timeClass = (ms: number, session: number, own: number, isValid: boolean) =>
    !isValid ? 'text-muted line-through' : ms === session ? 'font-bold text-purple-400' : ms === own ? 'text-green-400' : '';

  // Mescola usata in ogni giro, dagli stint.
  const tyreAt = (lap: number) => car.stints.find((s) => !s.endLap || lap <= s.endLap)?.tyre ?? '';

  return (
    <Card>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-bold">Tempi sul giro</h2>
        <select
          aria-label="Pilota"
          className={fieldSmClass}
          value={car.carIndex}
          onChange={(e) => setSelected(Number(e.target.value))}
        >
          {withLaps.map((c) => (
            <option key={c.carIndex} value={c.carIndex}>
              {c.position}. {carLabel(c)}
            </option>
          ))}
        </select>
      </div>
      <Table>
        <thead>
          <tr>
            <Th num>Giro</Th>
            <Th num>Tempo</Th>
            <Th num>Settore 1</Th>
            <Th num>Settore 2</Th>
            <Th num>Settore 3</Th>
            <Th>Gomme</Th>
          </tr>
        </thead>
        <tbody>
          {car.laps.map((l, i) => (
            <Tr key={i}>
              <Td num className="text-muted">{i + 1}</Td>
              <Td num className={timeClass(l.timeMs, sessionBest, ownBest, l.valid)}>
                {formatLapTime(l.timeMs)}
                {!l.valid && <span className="sr-only"> (annullato)</span>}
              </Td>
              {l.sectors.map((ms, s) => (
                <Td key={s} num className={timeClass(ms, sessionSectors[s], ownSectors[s], l.valid)}>
                  {ms ? (ms / 1000).toFixed(3) : '–'}
                </Td>
              ))}
              <Td className="text-sm">{TYRE_NAME[tyreAt(i + 1)] ?? '–'}</Td>
            </Tr>
          ))}
        </tbody>
      </Table>
      <p className="mt-2 text-xs text-muted">
        Viola il migliore della sessione, verde il migliore del pilota; barrati i giri annullati.
      </p>
    </Card>
  );
}
