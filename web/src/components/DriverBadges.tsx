import type { LiveDriver } from '../lib/types';
import { cx } from './ui';

// Indicatori accanto al nome in classifica: ai box, potenza per il sorpasso,
// ala mobile aperta, bandiera blu e giro più veloce. Sono in em: la dimensione
// la decide il testo della riga (o className).
export function DriverBadges({ driver: d, fastestLap, className }: { driver: LiveDriver; fastestLap: boolean; className?: string }) {
  const badges = [
    d.inPit && { key: 'pit', label: 'PIT', title: 'Ai box', className: 'text-sprint' },
    d.overtake && { key: 'ovt', label: 'OVT', title: 'Overtake: potenza extra per sorpassare', className: 'bg-yellow-300/15 text-yellow-300' },
    d.drs && { key: 'drs', label: 'DRS', title: 'Ala mobile aperta', className: 'bg-green-400/15 text-green-400' },
    d.blueFlag && { key: 'blue', label: '⚑', title: 'Bandiera blu: sta per essere doppiato', className: 'text-blue-400' },
    fastestLap && { key: 'fl', label: 'GV', title: 'Giro più veloce', className: 'bg-purple-500/20 text-purple-300' },
  ].filter((b) => !!b);
  if (!badges.length) return null;
  return (
    <span className={cx('inline-flex shrink-0 items-center gap-1 text-[0.7em] font-bold', className)}>
      {badges.map((b) => (
        <span key={b.key} className={cx('rounded px-1 leading-normal', b.className)} title={b.title}>{b.label}</span>
      ))}
    </span>
  );
}
