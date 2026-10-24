// Elementi grafici condivisi: tengono in un solo posto le classi Tailwind che si ripetono.
import { clsx, type ClassValue } from 'clsx';
import { useEffect, useRef, type ButtonHTMLAttributes, type ComponentProps, type KeyboardEvent, type ReactNode } from 'react';
import { twMerge } from 'tailwind-merge';

// Unisce le classi ignorando i valori falsi: cx('a', cond && 'b').
// In caso di conflitto vince l'ultima, così <Card className="mb-0"> sostituisce
// davvero il mb-4 di default invece di dipendere dall'ordine nel CSS generato.
export function cx(...classes: ClassValue[]): string {
  return twMerge(clsx(classes));
}

// Stile comune di input e select; fieldSmClass ha la stessa altezza di <Button size="sm">.
const fieldBase =
  'rounded-md border border-line bg-surface-2 focus:outline-2 focus:-outline-offset-1 focus:outline-accent';
export const fieldClass = cx(fieldBase, 'px-3 py-2');
export const fieldSmClass = cx(fieldBase, 'h-8 px-2.5 text-sm');

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost';
  size?: 'md' | 'sm';
}

// Classi del pulsante, usabili anche su un link (<a className={buttonClass(...)}>).
export function buttonClass(variant: ButtonProps['variant'] = 'primary', size: ButtonProps['size'] = 'md') {
  return cx(
    // Il bordo c'è sempre (trasparente nel primary), così tutte le varianti hanno la stessa altezza.
    'inline-flex cursor-pointer items-center justify-center rounded-md border font-semibold',
    'disabled:cursor-default disabled:opacity-50',
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
    variant === 'primary' && 'border-transparent bg-accent text-white hover:brightness-110',
    variant === 'ghost' && 'border-line text-muted hover:border-muted hover:text-fg',
    size === 'md' ? 'px-4 py-2' : 'h-8 px-3 text-sm',
  );
}

export function Button({ variant, size, className, ...props }: ButtonProps) {
  return <button className={cx(buttonClass(variant, size), className)} {...props} />;
}

export function Card({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cx('mb-4 rounded-lg border border-line bg-surface p-4', className)} {...props} />;
}

// Card che si espande e si riduce con un clic, anche da tastiera (Invio o spazio):
// expandableProps vanno sulla Card, expandableClass nelle sue classi.
export function expandableProps(expanded: boolean, onToggle: () => void) {
  return {
    role: 'button' as const,
    tabIndex: 0,
    'aria-expanded': expanded,
    onClick: onToggle,
    onKeyDown: (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        onToggle();
      }
    },
  };
}
export const expandableClass = cx(
  'scroll-mt-4 cursor-pointer hover:border-muted', // scroll-mt: un po' di margine sopra quando ci si scorre
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
);

// Espansa, una card cambia posto o dimensione: la pagina scorre fino a lei. Solo
// quando si espande, non se è già espansa all'apertura della pagina (la mappa
// grande si ricorda). Il ref va sulla Card.
export function useScrollOnExpand(expanded: boolean) {
  const ref = useRef<HTMLDivElement>(null);
  const wasExpanded = useRef(expanded);
  useEffect(() => {
    if (expanded && !wasExpanded.current) ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    wasExpanded.current = expanded;
  }, [expanded]);
  return ref;
}

// Scritta che dice cosa fa il clic su una card espandibile.
export function ExpandHint({ expanded, className }: { expanded: boolean; className?: string }) {
  return <span className={cx('text-xs text-muted', className)}>{expanded ? 'Riduci ▴' : 'Espandi ▾'}</span>;
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="p-8 text-center text-muted">{children}</p>;
}

// Segnaposto mentre una pagina aspetta i dati dal server.
export function Loading() {
  return <p className="p-8 text-center text-muted">Caricamento…</p>;
}

// Pallino del colore di un pilota.
export function ColorDot({ color, className }: { color: string; className?: string }) {
  if (!color) return null;
  return <span className={cx('inline-block size-2.5 shrink-0 rounded-full', className)} style={{ background: color }} />;
}

export function TeamTag({ team }: { team: string | null }) {
  return team ? <span className="text-sm text-muted">{team}</span> : null;
}

// --- Tabelle --------------------------------------------------------------

export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className="overflow-x-auto">
      <table className={cx('w-full', className)}>{children}</table>
    </div>
  );
}

const cellClass = 'border-b border-line px-3 py-2.5 text-left whitespace-nowrap';
const numClass = 'text-right tabular-nums';

export function Th({ num, className, ...props }: ComponentProps<'th'> & { num?: boolean }) {
  return (
    <th
      className={cx(cellClass, 'text-xs font-bold tracking-wider text-muted uppercase', num && numClass, className)}
      {...props}
    />
  );
}

export function Td({ num, className, ...props }: ComponentProps<'td'> & { num?: boolean }) {
  return <td className={cx(cellClass, num && numClass, className)} {...props} />;
}

export function Tr(props: ComponentProps<'tr'>) {
  return <tr className="hover:bg-surface-2" {...props} />;
}

// Colonna della posizione: oro, argento e bronzo per il podio.
export function rankClass(position: number | null): string {
  const colors: Record<number, string> = { 1: 'text-gold', 2: 'text-silver', 3: 'text-bronze' };
  return cx('w-10 font-extrabold', position !== null && colors[position]);
}

export const dnfClass = 'font-bold text-accent-text';
export { Dialog } from './Dialog';

// Barra in cima a una pagina, fuori dalle card e uguale in tutte le pagine:
// i filtri a sinistra, informazioni e azioni sulla pagina a destra.
export function PageToolbar({ children, actions }: { children: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2">
      {children}
      {actions && <div className="ml-auto flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

// Filtro della barra della pagina, con l'etichetta davanti.
export function Filter({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <span className="text-muted">{label}</span>
      {children}
    </label>
  );
}
