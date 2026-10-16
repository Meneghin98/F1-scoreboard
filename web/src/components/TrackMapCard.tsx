import { teamColor } from '../lib/live';
import type { LiveDriver, TrackMap } from '../lib/types';
import { useTrackMap } from '../lib/useTrackMap';
import { Card, Empty } from './ui';

// Oltre questa distanza (metri) due punti consecutivi non si collegano: in
// mezzo c'è un tratto non ancora visto.
const MAX_GAP = 40;

// Pianta del circuito con la posizione delle auto in tempo reale.
export function TrackMapCard({ uid, mapPoints, drivers }: { uid: string; mapPoints: number; drivers: LiveDriver[] }) {
  const map = useTrackMap(uid, mapPoints);

  return (
    <Card className="p-2">
      {map && map.points.length > 1 ? (
        <TrackMapSvg map={map} drivers={drivers} />
      ) : (
        <Empty>La mappa si disegna man mano che le auto percorrono il circuito…</Empty>
      )}
    </Card>
  );
}

function TrackMapSvg({ map, drivers }: { map: TrackMap; drivers: LiveDriver[] }) {
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

  return (
    <svg
      viewBox={`${minX - pad} ${minZ - pad} ${maxX - minX + 2 * pad} ${maxZ - minZ + 2 * pad}`}
      className="mx-auto block max-h-[32rem] w-full"
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
      {cars.map((d) => (
        <g
          key={d.carIndex}
          // Gli aggiornamenti arrivano 4 volte al secondo: la transizione fa scorrere le auto.
          style={{ transform: `translate(${d.x}px, ${d.z}px)`, transition: 'transform 250ms linear' }}
        >
          <title>{`${d.position}. ${d.name}`}</title>
          <circle r={r} fill={teamColor(d.team)} stroke={d.isPlayer ? 'white' : 'var(--color-bg)'} strokeWidth={r * 0.25} />
          <text
            textAnchor="middle"
            dominantBaseline="central"
            fontSize={r * 1.1}
            fontWeight={800}
            fill="var(--color-bg)"
          >
            {d.position}
          </text>
        </g>
      ))}
    </svg>
  );
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
