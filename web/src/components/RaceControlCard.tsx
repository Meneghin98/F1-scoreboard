import type { LiveDriver, RaceEvent, RaceEventKind } from '../lib/types';
import { Card, cx } from './ui';

// Etichetta e colore di ogni tipo di messaggio: il colore accompagna sempre l'etichetta.
export const EVENT_KIND: Record<RaceEventKind, { label: string; className: string }> = {
  penalty: { label: 'Penalità', className: 'text-accent-text' },
  collision: { label: 'Contatto', className: 'text-orange-400' },
  overtake: { label: 'Sorpasso', className: 'text-sky-400' },
  fastestLap: { label: 'Giro veloce', className: 'text-purple-400' },
  retirement: { label: 'Ritiro', className: 'text-accent-text' },
  safetyCar: { label: 'Safety car', className: 'text-sprint' },
  redFlag: { label: 'Bandiera rossa', className: 'text-accent-text' },
  flag: { label: 'Gara', className: 'text-fg' },
  drs: { label: 'DRS', className: 'text-green-400' },
  flashback: { label: 'Flashback', className: 'text-muted' },
};

// Messaggi della direzione gara, dal più recente. In grassetto quelli che
// riguardano un giocatore (in split-screen o umano online).
export function RaceControlCard({
  events,
  drivers,
  className,
}: {
  events: RaceEvent[];
  drivers: LiveDriver[];
  className?: string;
}) {
  const humans = new Set(drivers.filter((d) => d.controller > 0 || !d.ai).map((d) => d.carIndex));

  return (
    <Card className={cx('p-0', className)}>
      <h2 className="border-b border-line px-4 py-2 font-bold">Direzione gara</h2>
      {events.length ? (
        <ol className="max-h-72 overflow-y-auto py-1 text-sm">
          {[...events].reverse().map((e) => {
            const kind = EVENT_KIND[e.kind];
            return (
              <li key={e.id} className="flex gap-3 px-4 py-1">
                <span className="w-8 shrink-0 text-right text-muted tabular-nums">{e.lap ? `G${e.lap}` : ''}</span>
                <span className={cx('w-26 shrink-0 text-xs leading-5 font-bold uppercase', kind?.className)}>
                  {kind?.label ?? e.kind}
                </span>
                <span className={cx(e.cars.some((c) => humans.has(c)) && 'font-semibold')}>{e.text}</span>
              </li>
            );
          })}
        </ol>
      ) : (
        <p className="px-4 py-3 text-sm text-muted">Nessun messaggio per ora.</p>
      )}
    </Card>
  );
}
