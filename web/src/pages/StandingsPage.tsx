import { api } from '../lib/api';
import { raceLabels } from '../lib/format';
import type { Race, Standing } from '../lib/types';
import { href } from '../lib/route';
import { useLoad } from '../lib/useLoad';
import { PointsChart } from '../components/PointsChart';
import { Card, ColorDot, cx, dnfClass, Empty, Loading, rankClass, Table, TeamTag, Td, Th, Tr } from '../components/ui';

export function StandingsPage({ tournamentId }: { tournamentId: number }) {
  const { data } = useLoad(() => api.standings(tournamentId), [tournamentId]);
  if (!data) return <Loading />;

  const { races, standings } = data;
  if (!standings.length) {
    return <Empty>Nessun risultato ancora. Aggiungi una gara e inserisci i risultati.</Empty>;
  }

  const labels = raceLabels(races);
  const leader = standings[0].points;
  return (
    <>
      <Card>
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
                {races.map((race) => <RaceCell key={race.id} standing={s} race={race} />)}
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

// Punti presi nella gara, con la posizione d'arrivo in piccolo.
function RaceCell({ standing, race }: { standing: Standing; race: Race }) {
  const r = standing.perRace[race.id];
  if (!r) return <Td num className="text-muted">–</Td>;
  if (r.dnf) return <Td num className={dnfClass} title="Ritirato">DNF</Td>;
  return (
    <Td num title={`${r.position}° posto`}>
      {r.points}
      <span className="ml-1 text-xs text-muted">{r.position}°</span>
    </Td>
  );
}
