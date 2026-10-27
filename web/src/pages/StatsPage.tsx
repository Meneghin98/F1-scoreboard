import { api } from '../lib/api';
import { formatLapTime, formatUtcDate } from '../lib/format';
import { href } from '../lib/route';
import type { CareerStats, StatsResponse, Tournament, TrackRecord } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { ALL_TOURNAMENTS, TournamentSelect, useTournamentFilter } from '../components/TournamentFilter';
import { Card, ColorDot, cx, Empty, Loading, PageToolbar, rankClass, Table, Td, Th, Tr } from '../components/ui';

// Statistiche di tutti i tornei o di uno solo: carriera dei piloti, record dei circuiti e testa a testa.
export function StatsPage({ tournaments }: { tournaments: Tournament[] }) {
  const [filter, setFilter] = useTournamentFilter(tournaments, [ALL_TOURNAMENTS]);
  const tournamentId = filter === ALL_TOURNAMENTS ? undefined : Number(filter);
  const { data } = useLoad(() => api.stats(tournamentId), [tournamentId]);
  const single = tournamentId !== undefined;

  return (
    <>
      <PageToolbar>
        <TournamentSelect tournaments={tournaments} value={filter} onChange={setFilter}>
          <option value={ALL_TOURNAMENTS}>Tutti i tornei</option>
        </TournamentSelect>
      </PageToolbar>
      {!data ? (
        <Loading />
      ) : !data.career.length ? (
        <Empty>
          {single
            ? 'Nessun risultato in questo torneo.'
            : 'Nessun risultato ancora: le statistiche compaiono dopo le prime gare.'}
        </Empty>
      ) : (
        <>
          <CareerCard career={data.career} single={single} />
          <RecordsCard records={data.records} single={single} />
          <HeadToHeadCard h2h={data.headToHead} />
        </>
      )}
    </>
  );
}

// single: statistiche di un solo torneo, dove i titoli non hanno senso (colonna vuota).
function CareerCard({ career, single }: { career: CareerStats[]; single: boolean }) {
  return (
    <Card>
      <h2 className="mb-1 font-bold">Carriera</h2>
      <p className="mb-3 text-sm text-muted">
        {single
          ? 'Tutte le gare del torneo. I punti sono quelli della sua classifica; '
          : 'Tutte le gare, di tutti i tornei. I punti sono la somma delle classifiche dei tornei (senza "Nessun torneo"); '}
        vittorie, podi e piazzamenti sono quelli d'arrivo, IA compresa.
      </p>
      <Table>
        <thead>
          <tr>
            <Th>Pilota</Th>
            <Th num title="Tornei vinti (terminati da primi in classifica)">Titoli</Th>
            <Th num>Punti</Th>
            <Th num title="Gran Premi corsi">GP</Th>
            <Th num>Vittorie</Th>
            <Th num>Podi</Th>
            <Th num title="Qualifiche vinte">Pole</Th>
            <Th num title="Giri più veloci nei Gran Premi">Giri veloci</Th>
            <Th num title="Sprint vinte su quelle corse">Sprint</Th>
            <Th num>Ritiri</Th>
            <Th num title="Media dei piazzamenti nei Gran Premi finiti">Media</Th>
            <Th num title="Miglior piazzamento in un Gran Premio">Migliore</Th>
          </tr>
        </thead>
        <tbody>
          {career.map((c) => (
            <Tr key={c.player_id}>
              <Td>
                <a href={href('player', c.player_id)} className="flex items-center gap-2 font-semibold hover:underline">
                  <ColorDot color={c.color} />
                  {c.name}
                </a>
              </Td>
              <Td num className={cx(!single && c.titles > 0 && 'font-bold text-gold')}>{single ? '' : c.titles || '–'}</Td>
              <Td num className="font-extrabold">{c.points}</Td>
              <Td num>{c.races}</Td>
              <Td num>{c.wins}</Td>
              <Td num>{c.podiums}</Td>
              <Td num>{c.poles}</Td>
              <Td num className={cx(c.fastestLaps > 0 && 'text-purple-400')}>{c.fastestLaps}</Td>
              <Td num className="text-muted">{c.sprintWins}/{c.sprints}</Td>
              <Td num>{c.dnf}</Td>
              <Td num>{c.avgFinish === null ? '–' : c.avgFinish.toLocaleString('it-IT', { maximumFractionDigits: 1 })}</Td>
              <Td num className={rankClass(c.bestFinish)}>{c.bestFinish ?? '–'}</Td>
            </Tr>
          ))}
        </tbody>
      </Table>
    </Card>
  );
}

function RecordsCard({ records, single }: { records: StatsResponse['records']; single: boolean }) {
  return (
    <Card>
      <h2 className="mb-1 font-bold">Record dei circuiti</h2>
      <p className="mb-3 text-sm text-muted">
        {single
          ? 'Il giro più veloce dei piloti su ogni circuito in gara e in qualifica, nelle registrazioni delle gare del torneo.'
          : 'Il giro più veloce dei piloti su ogni circuito in gara e in qualifica, in tutte le sessioni registrate.'}
        {' '}I giri delle prove non contano.
      </p>
      {records.length ? (
        <Table>
          <thead>
            <tr><Th>Circuito</Th><Th num>Gara</Th><Th /><Th num>Qualifica</Th><Th /></tr>
          </thead>
          <tbody>
            {records.map((r) => (
              <Tr key={r.trackId}>
                <Td className="font-semibold">{r.track}</Td>
                <RecordCells record={r.race} main />
                <RecordCells record={r.qualifying} />
              </Tr>
            ))}
          </tbody>
        </Table>
      ) : (
        <p className="text-sm text-muted">I record arrivano dalle registrazioni della telemetria, con i piloti associati ai giocatori.</p>
      )}
    </Card>
  );
}

// Tempo e pilota di un record, con la sessione e il replay. Il record in gara è
// quello principale; quello in qualifica resta in secondo piano.
function RecordCells({ record: r, main }: { record: TrackRecord | null; main?: boolean }) {
  if (!r) return <><Td num className="text-muted">–</Td><Td /></>;
  return (
    <>
      <Td num className={main ? 'text-base font-bold text-purple-400' : 'text-sm text-muted'}>{formatLapTime(r.lapMs)}</Td>
      <Td className={cx(!main && 'text-sm')}>
        <a href={href('player', r.player_id)} className={cx('flex items-center gap-2 hover:underline', !main && 'text-muted')}>
          <ColorDot color={r.color} />
          {r.name}
        </a>
        <div className="flex items-center gap-2 text-xs text-muted">
          {r.sessionType} · {formatUtcDate(r.date)}
          <a href={href('replay', r.sessionId)} className="hover:text-fg hover:underline" title="Replay della sessione">▶ Replay</a>
        </div>
      </Td>
    </>
  );
}

function HeadToHeadCard({ h2h }: { h2h: StatsResponse['headToHead'] }) {
  if (h2h.players.length < 2) return null;
  return (
    <Card>
      <h2 className="mb-1 font-bold">Testa a testa</h2>
      <p className="mb-3 text-sm text-muted">
        Quante volte il pilota della riga è arrivato davanti a quello della colonna, nei Gran Premi e nelle sprint corsi insieme.
      </p>
      <Table>
        <thead>
          <tr>
            <Th />
            {h2h.players.map((p) => (
              <Th key={p.id} num>
                <span className="inline-flex items-center gap-1.5 normal-case"><ColorDot color={p.color} />{p.name}</span>
              </Th>
            ))}
          </tr>
        </thead>
        <tbody>
          {h2h.players.map((row, i) => (
            <Tr key={row.id}>
              <Td className="font-semibold">
                <span className="flex items-center gap-2"><ColorDot color={row.color} />{row.name}</span>
              </Td>
              {h2h.players.map((col, j) => {
                if (i === j) return <Td key={col.id} num className="text-muted">·</Td>;
                const won = h2h.ahead[i][j], lost = h2h.ahead[j][i];
                return (
                  <Td
                    key={col.id}
                    num
                    className={cx(won + lost === 0 ? 'text-muted' : won > lost ? 'font-bold text-green-400' : won < lost ? 'text-accent-text' : '')}
                    title={`${row.name} davanti a ${col.name} ${won} volte, dietro ${lost}`}
                  >
                    {won + lost ? `${won}–${lost}` : '–'}
                  </Td>
                );
              })}
            </Tr>
          ))}
        </tbody>
      </Table>
    </Card>
  );
}
