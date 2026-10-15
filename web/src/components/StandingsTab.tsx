import { api } from '../lib/api';
import { raceLabels } from '../lib/format';
import type { Race, Standing } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { Card, dnfClass, Empty, rankClass, Table, TeamTag, Td, Th, Tr } from './ui';

export function StandingsTab({ tournamentId }: { tournamentId: number }) {
  const { data } = useLoad(() => api.standings(tournamentId), [tournamentId]);
  if (!data) return null;

  const { races, standings } = data;
  if (!standings.length) {
    return <Empty>Nessun risultato ancora. Aggiungi una gara e inserisci i risultati.</Empty>;
  }

  const labels = raceLabels(races);
  return (
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
            <Th num>DNF</Th>
            <Th num>Punti</Th>
          </tr>
        </thead>
        <tbody>
          {standings.map((s) => (
            <Tr key={s.player_id}>
              <Td className={rankClass(s.rank)}>{s.rank}</Td>
              <Td>{s.name} <TeamTag team={s.team} /></Td>
              {races.map((race) => <RaceCell key={race.id} standing={s} race={race} />)}
              <Td num>{s.wins}</Td>
              <Td num>{s.podiums}</Td>
              <Td num>{s.dnf}</Td>
              <Td num className="text-lg font-extrabold">{s.points}</Td>
            </Tr>
          ))}
        </tbody>
      </Table>
    </Card>
  );
}

function RaceCell({ standing, race }: { standing: Standing; race: Race }) {
  const r = standing.perRace[race.id];
  if (!r) return <Td num className="text-muted">–</Td>;
  if (r.dnf) return <Td num className={dnfClass} title="Ritirato">DNF</Td>;
  return <Td num title={`${r.position}° posto`}>{r.points}</Td>;
}
