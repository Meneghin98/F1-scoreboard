// Elementi grafici condivisi: tengono in un solo posto le classi Tailwind che si ripetono.
import { clsx, type ClassValue } from 'clsx';
import type { ButtonHTMLAttributes, ComponentProps, ReactNode } from 'react';
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
