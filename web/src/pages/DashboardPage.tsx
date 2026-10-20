import type { ReactNode } from 'react';
import { api } from '../lib/api';
import { formatDate, formatDateTime } from '../lib/format';
import { isLive, sessionSummary } from '../lib/live';
import { href, VIEW_LABELS, type View } from '../lib/route';
import type { CareerStats, Tournament } from '../lib/types';
import { useLive } from '../lib/useLive';
import { useLoad } from '../lib/useLoad';
import { LiveDot } from '../components/LiveBadge';
import { ColorDot, cx, rankClass } from '../components/ui';

// Pagina iniziale: una card per ogni sezione, con un riassunto e il link per aprirla.
export function DashboardPage({ tournamentId, tournaments }: { tournamentId: number | null; tournaments: Tournament[] }) {
  const live = useLive();
  const { data: standings } = useLoad(
    () => (tournamentId ? api.standings(tournamentId) : Promise.resolve(null)),
    [tournamentId],
  );
  const { data: players } = useLoad(() => api.players(), []);
  const { data: stats } = useLoad(api.stats, []);
  // Come nella pagina delle registrazioni, le sessioni non terminate non si contano.
  const { data: sessions } = useLoad(
    async () => (await api.sessions()).filter((s) => s.hasClassification || s.recording),
    [],
  );

  const races = standings?.races ?? [];
  const sprints = races.filter((r) => r.type === 'sprint').length;
  const lastRace = races.at(-1);
  const active = isLive(live);
  const unassigned = sessions?.filter((s) => !s.race && !s.recording).length ?? 0;
  // "Nessun torneo" raccoglie le gare libere: non è un torneo da contare.
  const realTournaments = tournaments.filter((t) => !t.free);
  const ended = realTournaments.filter((t) => t.ended_at && !t.active).length;
  const activeTournament = realTournaments.find((t) => t.active);

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {/* Live su tutta la larghezza; sotto, la classifica a sinistra e gare e piloti impilati a destra,
          poi le registrazioni su tutta la larghezza e sotto tornei e statistiche affiancati. */}
      <DashboardCard view="live" highlight={active} className="sm:col-span-2">
        <div className="flex items-center gap-2 text-lg font-bold">
          <LiveDot active={live?.status === 'live'} />
          {active ? sessionSummary(live) : 'Nessuna sessione in corso'}
        </div>
        {active ? (
          <Podium rows={live.drivers.slice(0, 3).map((d) => ({ key: d.carIndex, rank: d.position, name: d.playerName || d.name }))} />
        ) : (
          <p className="text-sm text-muted">La classifica compare qui appena il gioco invia la telemetria.</p>
        )}
      </DashboardCard>

      <DashboardCard view="standings" className="sm:row-span-2">
        {!tournamentId ? (
          <p className="text-muted">Crea un torneo per iniziare.</p>
        ) : !standings ? null : standings.standings.length ? (
          <>
            <p className="text-sm text-muted">{standings.tournament.name}</p>
            <Podium
              rows={standings.standings.slice(0, 5).map((s) => ({
                key: s.player_id, rank: s.rank, name: s.name, value: `${s.points} pt`,
              }))}
            />
          </>
        ) : (
          <p className="text-muted">Nessun risultato ancora.</p>
        )}
      </DashboardCard>

      <DashboardCard view="races">
        {!tournamentId ? (
          <p className="text-muted">Crea un torneo per aggiungere le gare.</p>
        ) : standings && (
          <>
            <BigNumber value={races.length} label={races.length === 1 ? 'gara' : 'gare'}>
              {sprints > 0 && `di cui ${sprints} sprint`}
            </BigNumber>
            {lastRace && (
              <p className="text-sm text-muted">
                Ultima: {lastRace.track}{lastRace.date && ` · ${formatDate(lastRace.date)}`}
              </p>
            )}
          </>
        )}
      </DashboardCard>

      <DashboardCard view="players">
        {players && (
          <>
            <BigNumber value={players.length} label={players.length === 1 ? 'pilota' : 'piloti'} />
            {players.length > 0 && (
              <p className="truncate text-sm text-muted">{players.map((p) => p.name).join(', ')}</p>
            )}
          </>
        )}
      </DashboardCard>

      <DashboardCard view="recordings" className="sm:col-span-2">
        {sessions && (
          <>
            <BigNumber value={sessions.length} label={sessions.length === 1 ? 'registrazione' : 'registrazioni'}>
              {unassigned > 0 && `${unassigned} senza torneo`}
            </BigNumber>
            {sessions[0] && (
              <p className="text-sm text-muted">
                Ultima: {[sessions[0].track, sessions[0].type].filter(Boolean).join(' – ')} · {formatDateTime(sessions[0].startedAt)}
              </p>
            )}
          </>
        )}
      </DashboardCard>

      <DashboardCard view="tournaments">
        <BigNumber value={realTournaments.length} label={realTournaments.length === 1 ? 'torneo' : 'tornei'}>
          {ended > 0 && `${ended} ${ended === 1 ? 'terminato' : 'terminati'}`}
        </BigNumber>
        {activeTournament ? (
          <p className="text-sm text-muted">
            In corso: <span className="font-semibold text-fg">{activeTournament.name}</span>
            {activeTournament.leader && ` · in testa ${activeTournament.leader.name} (${activeTournament.leader.points} pt)`}
          </p>
        ) : (
          <p className="text-sm text-muted">Nessun torneo in corso.</p>
        )}
      </DashboardCard>

      <DashboardCard view="stats">
        {stats && (stats.career.length ? (
          <ul className="flex flex-col gap-1">
            <StatLeader label="Titoli" career={stats.career} pick={(c) => c.titles} />
            <StatLeader label="Vittorie" career={stats.career} pick={(c) => c.wins} />
            <StatLeader label="Pole" career={stats.career} pick={(c) => c.poles} />
            <StatLeader label="Giri veloci" career={stats.career} pick={(c) => c.fastestLaps} />
          </ul>
        ) : (
          <p className="text-muted">Le statistiche compaiono dopo le prime gare.</p>
        ))}
      </DashboardCard>
    </div>
  );
}

interface DashboardCardProps {
  view: View;
  highlight?: boolean;
  className?: string;
  children: ReactNode;
}

function DashboardCard({ view, highlight, className, children }: DashboardCardProps) {
  return (
    <a
      href={href(view)}
      className={cx(
        'group flex flex-col gap-3 rounded-lg border bg-surface p-5 transition-colors hover:bg-surface-2',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
        highlight ? 'border-green-500/60' : 'border-line hover:border-muted',
        className,
      )}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-bold tracking-wider text-muted uppercase">{VIEW_LABELS[view]}</h2>
        <span className="text-muted transition-transform group-hover:translate-x-1 group-hover:text-fg">→</span>
      </div>
      {children}
    </a>
  );
}

// Il pilota con più titoli, vittorie… in carriera; a parità, il primo della classifica di carriera.
function StatLeader({ label, career, pick }: { label: string; career: CareerStats[]; pick: (c: CareerStats) => number }) {
  const best = career.reduce((a, b) => (pick(b) > pick(a) ? b : a));
  const value = pick(best);
  return (
    <li className="flex items-baseline gap-2">
      <span className="w-24 text-sm text-muted">{label}</span>
      {value > 0 ? (
        <>
          <span className="flex flex-1 items-center gap-2 truncate"><ColorDot color={best.color} />{best.name}</span>
          <span className="font-extrabold tabular-nums">{value}</span>
        </>
      ) : (
        <span className="flex-1 text-muted">–</span>
      )}
    </li>
  );
}

function BigNumber({ value, label, children }: { value: number; label: string; children?: ReactNode }) {
  return (
    <p className="flex items-baseline gap-2">
      <span className="text-4xl font-extrabold tabular-nums">{value}</span>
      <span className="text-lg font-semibold">{label}</span>
      {children && <span className="text-sm text-muted">{children}</span>}
    </p>
  );
}

function Podium({ rows }: { rows: { key: number; rank: number; name: string; value?: string }[] }) {
  return (
    <ol className="flex flex-col gap-1">
      {rows.map((r) => (
        <li key={r.key} className="flex items-baseline gap-2">
          <span className={cx(rankClass(r.rank), 'w-6')}>{r.rank}</span>
          <span className="flex-1 truncate">{r.name}</span>
          {r.value && <span className="font-extrabold tabular-nums">{r.value}</span>}
        </li>
      ))}
    </ol>
  );
}
