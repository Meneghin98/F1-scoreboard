import { useEffect, useState, type ReactNode } from 'react';
import { setFilters, useFilter } from '../lib/route';
import type { Tournament } from '../lib/types';
import { fieldSmClass, Filter } from './ui';

// Voci dei filtri oltre ai tornei, come compaiono nell'indirizzo.
export const ALL_TOURNAMENTS = 'tutti';
export const UNASSIGNED = 'non-assegnate'; // registrazioni senza gara

export function tournamentLabel(t: Tournament): string {
  if (t.active) return `${t.name} (in corso)`;
  if (t.ended_at) return `${t.name} (terminato)`;
  return t.name;
}

// Torneo mostrato quando non se ne sceglie uno: quello in corso, altrimenti "Nessun torneo".
export function defaultTournament(tournaments: Tournament[]): Tournament | undefined {
  return tournaments.find((t) => t.active) ?? tournaments.find((t) => t.free);
}

// Filtro del torneo di una pagina, nell'indirizzo (?torneo=5): ogni pagina ha il suo.
// Il default è il torneo in corso all'apertura della pagina: se nel frattempo ne
// inizia un altro il filtro non salta. extra sono le voci in più (es. "tutti").
export function useTournamentFilter(
  tournaments: Tournament[],
  extra: readonly string[] = [],
): [string, (value: string) => void] {
  const raw = useFilter('torneo');
  const [opened] = useState(() => String(defaultTournament(tournaments)?.id ?? ''));
  const exists = (v: string) => extra.includes(v) || tournaments.some((t) => String(t.id) === v);
  // Se il torneo di partenza viene eliminato si passa al nuovo default.
  const fallback = exists(opened) ? opened : String(defaultTournament(tournaments)?.id ?? '');
  const valid = raw !== null && exists(raw);

  // Un torneo che non esiste più (eliminato, o un vecchio link) esce dall'indirizzo.
  useEffect(() => {
    if (raw !== null && !valid) setFilters({ torneo: null });
  }, [raw, valid]);

  const value = valid ? raw : fallback;
  return [value, (v) => setFilters({ torneo: v === fallback ? null : v })];
}

interface SelectProps {
  tournaments: Tournament[];
  value: string;
  onChange: (value: string) => void;
  children?: ReactNode; // voci prima dei tornei
}

export function TournamentSelect({ tournaments, value, onChange, children }: SelectProps) {
  return (
    <Filter label="Torneo">
      <select className={fieldSmClass} value={value} onChange={(e) => onChange(e.target.value)}>
        {children}
        {tournaments.map((t) => <option key={t.id} value={t.id}>{tournamentLabel(t)}</option>)}
      </select>
    </Filter>
  );
}
