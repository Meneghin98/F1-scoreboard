import { useLayoutEffect, useRef } from 'react';
import { api } from '../lib/api';
import { rulesSummary } from '../lib/f1';
import { raceLabels } from '../lib/format';
import type { Race, Standing } from '../lib/types';
import { href } from '../lib/route';
import { useLoad } from '../lib/useLoad';
import { PointsChart } from '../components/PointsChart';
import { Card, ColorDot, cx, dnfClass, Empty, Loading, rankClass, Table, TeamTag, Td, Th, Tr } from '../components/ui';

export function StandingsPage({ tournamentId }: { tournamentId: number }) {
  const { data } = useLoad(() => api.standings(tournamentId), [tournamentId]);
  const tableRef = useStickyColumns(FIXED_LEFT, FIXED_RIGHT, data);
  if (!data) return <Loading />;

  const { tournament, races, standings } = data;
  if (!standings.length) {
    return <Empty>Nessun risultato ancora. Aggiungi una gara e inserisci i risultati.</Empty>;
  }

  const labels = raceLabels(races);
  const leader = standings[0].points;
  const playersMode = tournament.points_mode === 'players';
  return (
    <>
      <Card>
        {!tournament.free && <p className="mb-2 text-sm text-muted">{rulesSummary(tournament)}</p>}
        <div ref={tableRef}>
          <Table className="border-separate border-spacing-0">
            <thead>
              <tr>
                <Th className={stickyClass}>Pos</Th>
                <Th className={cx(stickyClass, 'border-r')}>Pilota</Th>
                {races.map((race) => (
                  <Th key={race.id} num title={race.track}>{labels.get(race.id)}</Th>
                ))}
                <Th num className={cx(stickyClass, 'border-l')}>Vitt.</Th>
                <Th num className={stickyClass}>Podi</Th>
                <Th num className={stickyClass} title="Qualifiche vinte">Pole</Th>
                <Th num className={stickyClass} title="Giri più veloci nei Gran Premi">GV</Th>
                <Th num className={stickyClass}>DNF</Th>
                <Th num className={stickyClass} title="Punti di distacco dal primo">Dist.</Th>
                <Th num className={stickyClass}>Punti</Th>
              </tr>
            </thead>
            <tbody>
              {standings.map((s) => (
                <Tr key={s.player_id} className="group hover:bg-surface-2">
                  <Td className={cx(rankClass(s.rank), stickyClass)}>{s.rank}</Td>
                  <Td className={cx(stickyClass, 'border-r')}>
                    <a href={href('player', s.player_id)} className="flex items-center gap-2 hover:underline">
                      <ColorDot color={s.color} />
                      {s.name} <TeamTag team={s.team} />
                    </a>
                  </Td>
                  {races.map((race) => <RaceCell key={race.id} standing={s} race={race} playersMode={playersMode} />)}
                  <Td num className={cx(stickyClass, 'border-l')}>{s.wins}</Td>
                  <Td num className={stickyClass}>{s.podiums}</Td>
                  <Td num className={stickyClass}>{s.poles}</Td>
                  <Td num className={cx(stickyClass, s.fastestLaps > 0 && 'text-purple-400')}>{s.fastestLaps}</Td>
                  <Td num className={stickyClass}>{s.dnf}</Td>
                  <Td num className={cx(stickyClass, 'text-muted')}>{s.points < leader ? `−${leader - s.points}` : ''}</Td>
                  <Td num className={cx(stickyClass, 'text-lg font-extrabold')}>{s.points}</Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </div>
      </Card>
      <PointsChart races={races} standings={standings} labels={labels} />
    </>
  );
}

// Colonne che restano ferme mentre scorrono quelle delle gare: posizione e
// pilota a sinistra, statistiche e punti a destra.
const FIXED_LEFT = 2;
const FIXED_RIGHT = 7;
const stickyClass = 'sticky z-10 bg-surface group-hover:bg-surface-2';

// Gli offset left/right delle celle sticky dipendono dalla larghezza delle
// colonne vicine, che si conosce solo dopo il render: li misura e li aggiorna
// quando la tabella cambia dimensione.
function useStickyColumns(left: number, right: number, deps: unknown) {
  const ref = useRef<HTMLDivElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: deps (i dati) rifà partire l'effetto quando la tabella compare: al primo render non c'è ancora.
  useLayoutEffect(() => {
    const table = ref.current?.querySelector('table');
    if (!table) return;
    const update = () => {
      const widths = Array.from(table.rows[0]?.cells ?? [], (c) => c.getBoundingClientRect().width);
      for (const row of Array.from(table.rows)) {
        const cells = row.cells;
        let offset = 0;
        for (let i = 0; i < left; i++) {
          cells[i].style.left = `${offset}px`;
          offset += widths[i];
        }
        offset = 0;
        for (let i = cells.length - 1; i >= cells.length - right; i--) {
          cells[i].style.right = `${offset}px`;
          offset += widths[i];
        }
      }
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(table);
    return () => observer.disconnect();
  }, [left, right, deps]);
  return ref;
}

// Punti presi nella gara, con la posizione in piccolo (tra i soli piloti, in
// modalità "players") e un segno per i punti della pole o del giro veloce.
function RaceCell({ standing, race, playersMode }: { standing: Standing; race: Race; playersMode: boolean }) {
  const r = standing.perRace[race.id];
  if (!r) return <Td num className="text-muted">–</Td>;
  if (r.dnf) return <Td num className={dnfClass} title="Ritirato">DNF</Td>;
  const pole = race.type === 'qualifying';
  const title = [
    r.position === null ? 'Non classificato' : `${r.position}° ${playersMode ? 'tra i piloti' : 'posto'}`,
    r.bonus > 0 && `${pole ? 'pole' : 'giro veloce'} +${r.bonus}`,
  ].filter(Boolean).join(' · ');
  return (
    <Td num title={title}>
      {r.points}
      {r.position !== null && <span className="ml-1 text-xs text-muted">{r.position}°</span>}
      {r.bonus > 0 && (
        <span className={cx('ml-1 text-[10px] font-bold', pole ? 'text-gold' : 'text-purple-400')}>{pole ? 'POLE' : 'GV'}</span>
      )}
    </Td>
  );
}
