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

// Testo di un messaggio della direzione gara. Il server usa i nomi del gioco:
// per le auto dei giocatori si mostra il pilota del segnapunti, nel suo colore.
export function EventText({ event, drivers }: { event: RaceEvent; drivers: LiveDriver[] }) {
  const players = new Map<string, LiveDriver>();
  for (const car of event.cars) {
    const d = drivers.find((x) => x.carIndex === car);
    if (d?.playerName && d.name) players.set(d.name, d);
  }
  if (!players.size) return event.text;
  const escaped = [...players.keys()].map((name) => name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  // Con il gruppo di cattura split tiene i nomi trovati, nelle posizioni dispari.
  return event.text.split(new RegExp(`(${escaped.join('|')})`)).map((part, i) => {
    const d = i % 2 ? players.get(part) : undefined;
    return d ? (
      <span key={i} className="font-bold" style={{ color: d.playerColor || undefined }}>{d.playerName}</span>
    ) : part;
  });
}

// Messaggi della direzione gara, dal più recente. In grassetto quelli che
// riguardano un giocatore (in split-screen o umano online). Con fill la card
// prende l'altezza della riga in cui sta senza allungarla: la lista scorre dentro.
export function RaceControlCard({
  events,
  drivers,
  fill = false,
  className,
}: {
  events: RaceEvent[];
  drivers: LiveDriver[];
  fill?: boolean;
  className?: string;
}) {
  const humans = new Set(drivers.filter((d) => d.controller > 0 || !d.ai).map((d) => d.carIndex));

  return (
    <Card className={cx('p-0', fill && 'flex flex-col', className)}>
      <h2 className="border-b border-line px-4 py-2 font-bold">Direzione gara</h2>
      {events.length ? (
        // Con fill la lista è fuori dal flusso (absolute), così non conta per l'altezza della riga.
        <div className={cx(fill && 'relative min-h-24 flex-1')}>
          <ol className={cx('overflow-y-auto py-1 text-sm', fill ? 'absolute inset-0' : 'max-h-72')}>
            {[...events].reverse().map((e) => {
              const kind = EVENT_KIND[e.kind];
              return (
                <li key={e.id} className="flex gap-3 px-4 py-1">
                  <span className="w-8 shrink-0 text-right text-muted tabular-nums">{e.lap ? `G${e.lap}` : ''}</span>
                  <span className={cx('w-26 shrink-0 text-xs leading-5 font-bold uppercase', kind?.className)}>
                    {kind?.label ?? e.kind}
                  </span>
                  <span className={cx(e.cars.some((c) => humans.has(c)) && 'font-semibold')}><EventText event={e} drivers={drivers} /></span>
                </li>
              );
            })}
          </ol>
        </div>
      ) : (
        <p className="px-4 py-3 text-sm text-muted">Nessun messaggio per ora.</p>
      )}
    </Card>
  );
}
