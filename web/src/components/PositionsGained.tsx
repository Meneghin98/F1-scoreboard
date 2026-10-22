import type { LiveDriver } from '../lib/types';
import { cx } from './ui';

// Posizioni guadagnate (▲) o perse (▼) rispetto alla griglia di partenza.
// La dimensione e la spaziatura le decide chi lo usa tramite className.
export function PositionsGained({ driver: d, className }: { driver: LiveDriver; className?: string }) {
  const gained = d.gridPosition ? d.gridPosition - d.position : 0;
  if (!gained) return null;
  return (
    <span
      className={cx('font-semibold whitespace-nowrap', gained > 0 ? 'text-green-400' : 'text-accent-text', className)}
      title={`Partito ${d.gridPosition}°`}
    >
      {gained > 0 ? '▲' : '▼'}{Math.abs(gained)}
    </span>
  );
}
