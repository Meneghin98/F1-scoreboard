import { formatLapTime } from '../lib/format';
import { controllerLabel, TYRE_CLASS } from '../lib/live';
import type { LiveDriver, LiveSession } from '../lib/types';
import { Card, ColorDot, cx } from './ui';

// In split-screen i due giocatori si sfidano tra loro: chi è davanti, di
// quanto, e il confronto dei giri. In gara conta il distacco in pista, in prova
// e qualifica il miglior giro. Senza due giocatori non mostra nulla.
export function DuelCard({ session, drivers, className }: { session: LiveSession; drivers: LiveDriver[]; className?: string }) {
  const p1 = drivers.find((d) => d.controller === 1);
  const p2 = drivers.find((d) => d.controller === 2);
  if (!p1 || !p2) return null;

  const [ahead, behind] = p1.position < p2.position ? [p1, p2] : [p2, p1];
  let verdict: string;
  if (session.isRace) {
    const laps = ahead.lap - behind.lap;
    const gap = Math.abs(behind.gapToLeaderMs - ahead.gapToLeaderMs);
    verdict = laps > 0 && gap > (ahead.bestLapMs || 90_000)
      ? `davanti di ${laps} ${laps === 1 ? 'giro' : 'giri'}`
      : `davanti di ${formatSeconds(gap)}`;
  } else {
    // Fuori dalla gara le posizioni seguono il miglior giro.
    const [fast, slow] = bestFirst(p1, p2);
    verdict = fast.bestLapMs && slow.bestLapMs
      ? `più veloce di ${formatSeconds(slow.bestLapMs - fast.bestLapMs)}`
      : 'nessun giro completo per entrambi';
    return <DuelLayout p1={p1} p2={p2} leader={fast} verdict={verdict} className={className} />;
  }
  return <DuelLayout p1={p1} p2={p2} leader={ahead} verdict={verdict} className={className} />;
}

function DuelLayout({ p1, p2, leader, verdict, className }: {
  p1: LiveDriver;
  p2: LiveDriver;
  leader: LiveDriver;
  verdict: string;
  className?: string;
}) {
  const rows: [string, (d: LiveDriver) => string, (d: LiveDriver) => number][] = [
    ['Ultimo giro', (d) => formatLapTime(d.lastLapMs), (d) => d.lastLapMs],
    ['Miglior giro', (d) => formatLapTime(d.bestLapMs), (d) => d.bestLapMs],
  ];
  return (
    <Card className={cx('flex min-w-72 flex-1 flex-col gap-3', className)}>
      <h2 className="font-bold">Duello</h2>
      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3">
        <DuelSide driver={p1} />
        <span className="text-sm font-bold text-muted">vs</span>
        <DuelSide driver={p2} align="right" />
      </div>
      <p className="text-center">
        <span className="font-bold" style={{ color: leader.playerColor || undefined }}>{who(leader)}</span>{' '}
        <span className="tabular-nums">{verdict}</span>
      </p>
      <table className="w-full text-sm">
        <tbody>
          {rows.map(([label, show, value]) => {
            const a = value(p1), b = value(p2);
            const better = a && b && a !== b ? (a < b ? 1 : 2) : 0; // tempo più basso = meglio
            return (
              <tr key={label} className="border-t border-line">
                <td className={cx('py-1.5 tabular-nums', better === 1 && 'font-bold text-green-400')}>{show(p1)}</td>
                <td className="py-1.5 text-center text-xs text-muted">{label}</td>
                <td className={cx('py-1.5 text-right tabular-nums', better === 2 && 'font-bold text-green-400')}>{show(p2)}</td>
              </tr>
            );
          })}
          <tr className="border-t border-line">
            <td className="py-1.5"><TyreLabel driver={p1} /></td>
            <td className="py-1.5 text-center text-xs text-muted">Gomme</td>
            <td className="py-1.5 text-right"><TyreLabel driver={p2} /></td>
          </tr>
        </tbody>
      </table>
    </Card>
  );
}

function DuelSide({ driver: d, align }: { driver: LiveDriver; align?: 'right' }) {
  return (
    <div className={cx('min-w-0', align === 'right' && 'text-right')}>
      <div className={cx('flex items-center gap-2', align === 'right' && 'flex-row-reverse')}>
        {/* "1°" e non "P1": P1 e P2 indicano già i controller. */}
        <span className="text-3xl font-extrabold tabular-nums">{d.position}°</span>
        <ColorDot color={d.playerColor} className="size-3" />
      </div>
      <div className="truncate font-semibold">{who(d)}</div>
      <div className="truncate text-xs text-muted">{controllerLabel(d.controller)} · {d.name}</div>
    </div>
  );
}

function TyreLabel({ driver: d }: { driver: LiveDriver }) {
  if (!d.tyre) return <span className="text-muted">–</span>;
  return (
    <span className="inline-flex items-center gap-1.5 tabular-nums">
      <span className={cx('inline-flex size-5 items-center justify-center rounded-full border-2 text-[10px] font-extrabold', TYRE_CLASS[d.tyre])}>
        {d.tyre}
      </span>
      <span className="text-muted">
        {d.tyreAge} {d.tyreAge === 1 ? 'giro' : 'giri'} · {d.pitStops} {d.pitStops === 1 ? 'sosta' : 'soste'}
      </span>
    </span>
  );
}

// Il pilota del segnapunti se scelto, altrimenti il controller.
function who(d: LiveDriver): string {
  return d.playerName || controllerLabel(d.controller);
}

function bestFirst(a: LiveDriver, b: LiveDriver): [LiveDriver, LiveDriver] {
  if (!a.bestLapMs) return [b, a];
  if (!b.bestLapMs) return [a, b];
  return a.bestLapMs <= b.bestLapMs ? [a, b] : [b, a];
}

// 4532 → "4.532 s".
function formatSeconds(ms: number): string {
  return `${(ms / 1000).toFixed(3)} s`;
}
