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

// Riavvii e flashback di ogni gara, in due colonne dopo i piloti: sono della
// sessione, non di un pilota. In cima il totale del torneo.
const COUNTERS: { key: 'restarts' | 'flashbacks'; icon: string; label: string; title: string }[] = [
  { key: 'restarts', icon: '↻', label: 'Riavvii', title: 'Volte che la sessione è stata ricominciata dal gioco' },
  { key: 'flashbacks', icon: '⟲', label: 'Flashback', title: 'Flashback usati nella sessione' },
];
const counterBorder = 'border-l-2';

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
  // Le colonne di riavvii e flashback servono solo se qualche gara ha una registrazione.
  const counters = races.some((r) => r.restarts !== null || r.flashbacks !== null);
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
              {counters && COUNTERS.map((c, i) => (
                <Th key={c.key} className={cx(colClass, i === 0 && counterBorder, 'w-[1%] px-4 whitespace-nowrap tracking-normal normal-case')} title={c.title}>
                  <div className="text-base text-muted">{c.icon}</div>
                  <div className="text-sm text-fg">{c.label}</div>
                </Th>
              ))}
            </tr>
            <tr className="group hover:bg-surface-2">
              <Th className={stickyClass}>Punti</Th>
              {standings.map((s) => (
                <Td key={s.player_id} className={cx(colClass, 'text-lg font-extrabold')}>{s.points}</Td>
              ))}
              {counters && COUNTERS.map((c, i) => (
                <Td key={c.key} className={cx(colClass, i === 0 && counterBorder, 'text-lg font-extrabold')} title={`${c.label} in tutto il torneo`}>
                  {races.reduce((sum, r) => sum + (r[c.key] ?? 0), 0)}
                </Td>
              ))}
            </tr>
            <tr className="group hover:bg-surface-2">
              <Th className={cx(stickyClass, 'border-b-2')} title="Punti di distacco dal primo">Dist.</Th>
              {standings.map((s) => (
                <Td key={s.player_id} className={cx(colClass, 'border-b-2 text-muted')}>
                  {s.points < leader ? `−${leader - s.points}` : ''}
                </Td>
              ))}
              {counters && COUNTERS.map((c, i) => <Td key={c.key} className={cx(colClass, i === 0 && counterBorder, 'border-b-2')} />)}
            </tr>
          </thead>
          <tbody>
            {races.map((race) => (
              <Tr key={race.id} className="group hover:bg-surface-2">
                <Th className={stickyClass} title={race.track}>{labels.get(race.id)}</Th>
                {standings.map((s) => <RaceCell key={s.player_id} standing={s} race={race} playersMode={playersMode} />)}
                {counters && COUNTERS.map((c, i) => {
                  const n = race[c.key];
                  return (
                    <Td key={c.key} className={cx(colClass, i === 0 && counterBorder, n ? 'text-sprint' : 'text-muted')}>
                      {n ?? '–'}
                    </Td>
                  );
                })}
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
                {counters && COUNTERS.map((c, j) => <Td key={c.key} className={cx(colClass, j === 0 && counterBorder, i === 0 && 'border-t-2')} />)}
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
