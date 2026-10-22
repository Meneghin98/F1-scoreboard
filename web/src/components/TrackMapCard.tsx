import type { KeyboardEvent } from 'react';
import { controllerLabel, teamColor } from '../lib/live';
import type { LiveDriver, MarshalZone, TrackMap } from '../lib/types';
import { useTrackMap } from '../lib/useTrackMap';
import { Card, cx, Empty } from './ui';

// Oltre questa distanza (metri) due punti consecutivi non si collegano: in
// mezzo c'è un tratto non ancora visto.
const MAX_GAP = 40;

// Pianta del circuito con la posizione delle auto in tempo reale.
// La mappa riempie la card: le dimensioni le decide chi la usa tramite className.
// Con onClick la card si comporta da pulsante (anche da tastiera), per esempio
// per cambiarne la dimensione; title spiega cosa fa il clic.
// loadMap scarica la pianta: di default quella della sessione live.
export function TrackMapCard({
  uid,
  mapPoints,
  drivers,
  marshalZones = [],
  className,
  onClick,
  title,
  loadMap,
}: {
  uid: string;
  mapPoints: number;
  drivers: LiveDriver[];
  marshalZones?: MarshalZone[]; // settori dei commissari: quelli con bandiera gialla si colorano
  className?: string;
  onClick?: () => void;
  title?: string;
  loadMap?: () => Promise<TrackMap>;
}) {
  const map = useTrackMap(uid, mapPoints, loadMap);
  const button = onClick && {
    role: 'button',
    tabIndex: 0,
    title,
    onClick,
    onKeyDown: (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        onClick();
      }
    },
  };

  return (
    <Card
      {...button}
      className={cx(
        'flex items-center justify-center p-2',
        onClick && cx(
          'cursor-pointer hover:border-muted',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
        ),
        className,
      )}
    >
      {map && map.points.length > 1 ? (
        <TrackMapSvg map={map} drivers={drivers} marshalZones={marshalZones} />
      ) : (
        <Empty>La mappa si disegna man mano che le auto percorrono il circuito…</Empty>
      )}
    </Card>
  );
}

function TrackMapSvg({ map, drivers, marshalZones }: { map: TrackMap; drivers: LiveDriver[]; marshalZones: MarshalZone[] }) {
  const xs = map.points.map((p) => p.x);
  const zs = map.points.map((p) => p.z);
  const minX = Math.min(...xs), maxX = Math.max(...xs);
  const minZ = Math.min(...zs), maxZ = Math.max(...zs);
  const size = Math.max(maxX - minX, maxZ - minZ);
  const pad = size * 0.05;
  const r = size / 50; // raggio dei segnaposto delle auto

  // Le auto ferme in garage o non ancora in gioco sono a (0, 0).
  // Il primo in classifica va disegnato per ultimo, così resta sopra gli altri.
  const cars = drivers.filter((d) => d.x || d.z).reverse();
  const yellow = yellowPath(map, marshalZones);

  return (
    <svg
      viewBox={`${minX - pad} ${minZ - pad} ${maxX - minX + 2 * pad} ${maxZ - minZ + 2 * pad}`}
      className="block size-full"
      role="img"
      aria-label="Mappa del circuito"
    >
      <path
        d={outlinePath(map)}
        fill="none"
        stroke="var(--color-surface-2)"
        strokeWidth={r * 0.9}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {yellow && (
        <path
          d={yellow}
          fill="none"
          stroke="var(--color-sprint)"
          strokeWidth={r * 0.9}
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <title>Bandiera gialla</title>
        </path>
      )}
      <FinishLine map={map} size={r} />
      {cars.map((d) => (
        <g
          key={d.carIndex}
          // Gli aggiornamenti arrivano 4 volte al secondo: la transizione fa scorrere le auto.
          style={{ transform: `translate(${d.x}px, ${d.z}px)`, transition: 'transform 250ms linear' }}
        >
          <title>{`${d.position}. ${d.name}`}</title>
          <circle r={r} fill={teamColor(d.team)} stroke={d.controller ? 'white' : 'var(--color-bg)'} strokeWidth={r * 0.25} />
          <text
            textAnchor="middle"
            dominantBaseline="central"
            fontSize={r * 1.1}
            fontWeight={800}
            fill="var(--color-bg)"
          >
            {d.position}
          </text>
          {/* Sopra le auto dei giocatori il controller, per distinguerle in split-screen. */}
          {d.controller > 0 && (
            <text
              y={-r * 1.9}
              textAnchor="middle"
              dominantBaseline="central"
              fontSize={r * 1.1}
              fontWeight={800}
              fill="white"
              stroke="var(--color-bg)"
              strokeWidth={r * 0.3}
              paintOrder="stroke"
            >
              {controllerLabel(d.controller)}
            </text>
          )}
        </g>
      ))}
    </svg>
  );
}

// Linea del traguardo a scacchi, di traverso al tracciato. Si disegna solo
// quando il tratto del traguardo è già stato visto (distanza dal traguardo ~0).
const FINISH_MAX_D = 20; // metri
const FINISH_COLUMNS = 6;

function FinishLine({ map, size }: { map: TrackMap; size: number }) {
  const [start, next] = map.points;
  if (!next || start.d > FINISH_MAX_D || Math.hypot(next.x - start.x, next.z - start.z) > MAX_GAP) return null;
  const angle = (Math.atan2(next.z - start.z, next.x - start.x) * 180) / Math.PI;
  const sq = size * 0.4; // lato di un quadretto
  const half = (FINISH_COLUMNS * sq) / 2;
  // In coordinate locali X va nel senso di marcia e Y di traverso.
  return (
    <g transform={`translate(${start.x} ${start.z}) rotate(${angle})`}>
      <title>Partenza / Arrivo</title>
      <rect x={-sq} y={-half} width={2 * sq} height={2 * half} fill="white" stroke="var(--color-bg)" strokeWidth={sq * 0.3} />
      {Array.from({ length: FINISH_COLUMNS * 2 }, (_, i) => {
        const row = i % 2, col = Math.floor(i / 2);
        return (row + col) % 2 ? null : <rect key={i} x={(row - 1) * sq} y={col * sq - half} width={sq} height={sq} fill="black" />;
      })}
    </g>
  );
}

// Percorso SVG dei tratti in bandiera gialla, vuoto se non ce ne sono. Ogni
// settore dei commissari va dal suo inizio a quello del successivo; l'ultimo
// può scavalcare il traguardo.
function yellowPath(map: TrackMap, zones: MarshalZone[]): string {
  if (!zones.some((z) => z.flag === 3) || !map.length) return '';
  const inYellow = (fraction: number) =>
    zones.some((z, i) => {
      if (z.flag !== 3) return false;
      const end = zones[(i + 1) % zones.length].start;
      return z.start <= end ? fraction >= z.start && fraction < end : fraction >= z.start || fraction < end;
    });
  let d = '';
  let prev: TrackMap['points'][number] | null = null;
  for (const p of map.points) {
    if (!inYellow(p.d / map.length)) {
      prev = null;
      continue;
    }
    const jump = !prev || Math.hypot(p.x - prev.x, p.z - prev.z) > MAX_GAP;
    d += `${jump ? 'M' : 'L'}${p.x} ${p.z}`;
    prev = p;
  }
  return d;
}

// Percorso SVG del tracciato: X verso destra e Z verso il basso danno la pianta
// con l'orientamento giusto (non specchiata).
function outlinePath({ points }: TrackMap): string {
  const far = (a: TrackMap['points'][number], b: TrackMap['points'][number]) =>
    Math.hypot(b.x - a.x, b.z - a.z) > MAX_GAP;
  let d = '';
  let gaps = false;
  points.forEach((p, i) => {
    const jump = i > 0 && far(points[i - 1], p);
    gaps ||= jump;
    d += `${i === 0 || jump ? 'M' : 'L'}${p.x} ${p.z}`;
  });
  // Z chiude solo l'ultimo tratto: si usa quando il giro è tutto collegato.
  if (!gaps && !far(points[points.length - 1], points[0])) d += 'Z';
  return d;
}
