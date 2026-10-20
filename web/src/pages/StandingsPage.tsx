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
        <Table>
          <thead>
            <tr>
              <Th>Pos</Th>
              <Th>Pilota</Th>
              {races.map((race) => (
                <Th key={race.id} num title={race.track}>{labels.get(race.id)}</Th>
              ))}
              <Th num>Vitt.</Th>
              <Th num>Podi</Th>
              <Th num title="Qualifiche vinte">Pole</Th>
              <Th num title="Giri più veloci nei Gran Premi">GV</Th>
              <Th num>DNF</Th>
              <Th num title="Punti di distacco dal primo">Dist.</Th>
              <Th num>Punti</Th>
            </tr>
          </thead>
          <tbody>
            {standings.map((s) => (
              <Tr key={s.player_id}>
                <Td className={rankClass(s.rank)}>{s.rank}</Td>
                <Td>
                  <a href={href('player', s.player_id)} className="flex items-center gap-2 hover:underline">
                    <ColorDot color={s.color} />
                    {s.name} <TeamTag team={s.team} />
                  </a>
                </Td>
                {races.map((race) => <RaceCell key={race.id} standing={s} race={race} playersMode={playersMode} />)}
                <Td num>{s.wins}</Td>
                <Td num>{s.podiums}</Td>
                <Td num>{s.poles}</Td>
                <Td num className={cx(s.fastestLaps > 0 && 'text-purple-400')}>{s.fastestLaps}</Td>
                <Td num>{s.dnf}</Td>
                <Td num className="text-muted">{s.points < leader ? `−${leader - s.points}` : ''}</Td>
                <Td num className="text-lg font-extrabold">{s.points}</Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      </Card>
      <PointsChart races={races} standings={standings} labels={labels} />
    </>
  );
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
