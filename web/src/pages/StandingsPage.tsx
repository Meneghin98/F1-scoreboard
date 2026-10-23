import { api } from '../lib/api';
import { rulesSummary } from '../lib/f1';
import { raceLabels } from '../lib/format';
import type { Race, Standing, Tournament } from '../lib/types';
import { href } from '../lib/route';
import { useLoad } from '../lib/useLoad';
import { PointsChart } from '../components/PointsChart';
import { TournamentSelect, useTournamentFilter } from '../components/TournamentFilter';
import { Card, ColorDot, cx, dnfClass, Empty, Loading, PageToolbar, rankClass, Table, Td, TeamTag, Th, Tr } from '../components/ui';

export function StandingsPage({ tournaments }: { tournaments: Tournament[] }) {
  const [filter, setFilter] = useTournamentFilter(tournaments);
  const tournamentId = Number(filter);
  return (
    <>
      <PageToolbar>
        <TournamentSelect tournaments={tournaments} value={filter} onChange={setFilter} />
      </PageToolbar>
      {/* key: cambiando torneo la classifica riparte da zero. */}
      {tournamentId > 0 && <Standings key={tournamentId} tournamentId={tournamentId} />}
    </>
  );
}

// Statistiche di riepilogo, una riga ciascuna sotto le gare.
const STATS: { label: string; title?: string; value: (s: Standing) => number; className?: (s: Standing) => string | false }[] = [
  { label: 'Vitt.', value: (s) => s.wins },
  { label: 'Podi', value: (s) => s.podiums },
  { label: 'Pole', title: 'Qualifiche vinte', value: (s) => s.poles },
  { label: 'GV', title: 'Giri più veloci nei Gran Premi', value: (s) => s.fastestLaps, className: (s) => s.fastestLaps > 0 && 'text-purple-400' },
  { label: 'DNF', value: (s) => s.dnf },
];

// Classifica in verticale: un pilota per colonna (in ordine di classifica),
// una gara per riga. In cima punti e distacco, in fondo le statistiche.
// Sopra c'è il grafico dei punti.
function Standings({ tournamentId }: { tournamentId: number }) {
  const { data } = useLoad(() => api.standings(tournamentId), [tournamentId]);
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
      <PointsChart races={races} standings={standings} labels={labels} />
      <Card>
        {!tournament.free && <p className="mb-2 text-sm text-muted">{rulesSummary(tournament)}</p>}
        <Table className="border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className={stickyClass} />
              {standings.map((s) => (
                <Th key={s.player_id} className={cx(colClass, 'tracking-normal normal-case')}>
                  <div className={cx(rankClass(s.rank), 'w-auto text-base')}>{s.rank}°</div>
                  <a href={href('player', s.player_id)} className="inline-flex items-center gap-1.5 text-sm text-fg hover:underline">
                    <ColorDot color={s.color} />
                    {s.name}
                  </a>
                  <div className="font-normal"><TeamTag team={s.team} /></div>
                </Th>
              ))}
            </tr>
            <tr className="group hover:bg-surface-2">
              <Th className={stickyClass}>Punti</Th>
              {standings.map((s) => (
                <Td key={s.player_id} className={cx(colClass, 'text-lg font-extrabold')}>{s.points}</Td>
              ))}
            </tr>
            <tr className="group hover:bg-surface-2">
              <Th className={cx(stickyClass, 'border-b-2')} title="Punti di distacco dal primo">Dist.</Th>
              {standings.map((s) => (
                <Td key={s.player_id} className={cx(colClass, 'border-b-2 text-muted')}>
                  {s.points < leader ? `−${leader - s.points}` : ''}
                </Td>
              ))}
            </tr>
          </thead>
          <tbody>
            {races.map((race) => (
              <Tr key={race.id} className="group hover:bg-surface-2">
                <Th className={stickyClass} title={race.track}>{labels.get(race.id)}</Th>
                {standings.map((s) => <RaceCell key={s.player_id} standing={s} race={race} playersMode={playersMode} />)}
              </Tr>
            ))}
          </tbody>
          <tfoot>
            {STATS.map((stat, i) => (
              <Tr key={stat.label} className="group hover:bg-surface-2">
                <Th className={cx(stickyClass, i === 0 && 'border-t-2')} title={stat.title}>{stat.label}</Th>
                {standings.map((s) => (
                  <Td key={s.player_id} className={cx(colClass, i === 0 && 'border-t-2', stat.className?.(s))}>
                    {stat.value(s)}
                  </Td>
                ))}
              </Tr>
            ))}
          </tfoot>
        </Table>
      </Card>
    </>
  );
}

// La prima colonna (gare e statistiche) resta ferma mentre scorrono i piloti.
const stickyClass = 'sticky left-0 z-10 border-r bg-surface group-hover:bg-surface-2';
// Colonna di un pilota: tutto centrato sotto il nome.
const colClass = 'text-center tabular-nums';

// Punti presi nella gara, con la posizione in piccolo (tra i soli piloti, in
// modalità "players") e un segno per i punti della pole, del giro veloce o
// della vittoria davanti alla IA.
function RaceCell({ standing, race, playersMode }: { standing: Standing; race: Race; playersMode: boolean }) {
  const r = standing.perRace[race.id];
  if (!r) return <Td className={cx(colClass, 'text-muted')}>–</Td>;
  if (r.dnf) return <Td className={cx(colClass, dnfClass)} title="Ritirato">DNF</Td>;
  const pole = race.type === 'qualifying';
  const title = [
    r.position === null ? 'Non classificato' : `${r.position}° ${playersMode ? 'tra i piloti' : 'posto'}`,
    r.bonus > 0 && `${pole ? 'pole' : 'giro veloce'} +${r.bonus}`,
    r.win_bonus > 0 && `1° assoluto +${r.win_bonus}`,
  ].filter(Boolean).join(' · ');
  return (
    <Td className={colClass} title={title}>
      {r.points}
      {r.position !== null && <span className="ml-1 text-xs text-muted">{r.position}°</span>}
      {r.bonus > 0 && (
        <span className={cx('ml-1 text-[10px] font-bold', pole ? 'text-gold' : 'text-purple-400')}>{pole ? 'POLE' : 'GV'}</span>
      )}
      {r.win_bonus > 0 && <span className="ml-1 text-[10px] font-bold text-accent-text" title="Vittoria davanti alla IA">P1</span>}
    </Td>
  );
}
