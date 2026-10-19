import type { ReactNode } from 'react';
import { api } from '../lib/api';
import { raceTypeLabel } from '../lib/f1';
import { formatDate, formatLapTime, formatUtcDate } from '../lib/format';
import { href } from '../lib/route';
import { useLoad } from '../lib/useLoad';
import { buttonClass, Card, ColorDot, cx, dnfClass, Empty, Loading, rankClass, Table, TeamTag, Td, Th, Tr } from '../components/ui';

// Profilo di un pilota: numeri di carriera, tornei, migliori giri per
// circuito, ultimi risultati e confronti diretti con gli altri.
export function PlayerPage({ playerId }: { playerId: number }) {
  const { data: p } = useLoad(() => api.playerStats(playerId), [playerId]);
  if (!p) return <Loading />;
  const c = p.career;

  return (
    <>
      <Card className="flex flex-wrap items-center gap-x-8 gap-y-4">
        <h2 className="flex items-center gap-3 text-2xl font-extrabold">
          <ColorDot color={p.player.color} className="size-4" />
          {p.player.name}
          <TeamTag team={p.player.team} />
        </h2>
        <div className="flex flex-wrap gap-x-8 gap-y-3">
          <Tile label="Titoli" value={c.titles} highlight={c.titles > 0} />
          <Tile label="Punti" value={c.points} />
          <Tile label="Vittorie" value={c.wins} />
          <Tile label="Podi" value={c.podiums} />
          <Tile label="Pole" value={c.poles} />
          <Tile label="Giri veloci" value={c.fastestLaps} />
          <Tile label="Gran Premi" value={c.races} />
          <Tile label="Ritiri" value={c.dnf} />
          <Tile
            label="Media arrivo"
            value={c.avgFinish === null ? '–' : c.avgFinish.toLocaleString('it-IT', { maximumFractionDigits: 1 })}
          />
        </div>
      </Card>

      <div className="mb-4 grid gap-4 lg:grid-cols-2">
        <Section title="Tornei" className="mb-0">
          {p.tournaments.length ? (
            <Table>
              <thead><tr><Th>Torneo</Th><Th num>Pos</Th><Th num>Punti</Th><Th num>Vittorie</Th></tr></thead>
              <tbody>
                {p.tournaments.map((t) => (
                  <Tr key={t.id}>
                    <Td>
                      {t.name}
                      {(t.active || t.ended) && <span className="ml-2 text-xs text-muted">{t.active ? 'in corso' : 'terminato'}</span>}
                    </Td>
                    <Td num className={rankClass(t.rank)}>{t.rank}° <span className="text-xs font-normal text-muted">di {t.of}</span></Td>
                    <Td num className="font-bold">{t.points}</Td>
                    <Td num>{t.wins}</Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          ) : <p className="text-sm text-muted">Nessun torneo.</p>}
        </Section>

        <Section title="Testa a testa" className="mb-0">
          {p.rivals.length ? (
            <Table>
              <thead><tr><Th>Rivale</Th><Th num>Davanti</Th><Th num>Dietro</Th></tr></thead>
              <tbody>
                {p.rivals.map((r) => (
                  <Tr key={r.id}>
                    <Td>
                      <a href={href('player', r.id)} className="flex items-center gap-2 hover:underline">
                        <ColorDot color={r.color} />{r.name}
                      </a>
                    </Td>
                    <Td num className={cx(r.ahead > r.behind && 'font-bold text-green-400')}>{r.ahead}</Td>
                    <Td num className={cx(r.behind > r.ahead && 'font-bold text-accent-text')}>{r.behind}</Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          ) : <p className="text-sm text-muted">Nessuna gara corsa con altri piloti.</p>}
        </Section>
      </div>

      <Section title="Migliori giri">
        {p.bestLaps.length ? (
          <Table>
            <thead><tr><Th>Circuito</Th><Th num>Tempo</Th><Th>Sessione</Th><Th /></tr></thead>
            <tbody>
              {p.bestLaps.map((l) => (
                <Tr key={l.trackId}>
                  <Td>{l.track}</Td>
                  <Td num className={cx(l.record && 'font-bold text-purple-400')} title={l.record ? 'Record del circuito' : undefined}>
                    {formatLapTime(l.lapMs)}{l.record && ' ★'}
                  </Td>
                  <Td className="text-sm text-muted">{l.sessionType} · {formatUtcDate(l.date)}</Td>
                  <Td className="text-right">
                    <a href={href('replay', l.sessionId)} className={buttonClass('ghost', 'sm')}>▶ Replay</a>
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        ) : <p className="text-sm text-muted">I giri arrivano dalle registrazioni della telemetria.</p>}
      </Section>

      <Section title="Ultimi risultati">
        {p.recent.length ? (
          <Table>
            <thead><tr><Th>Data</Th><Th>Gara</Th><Th>Torneo</Th><Th num>Pos</Th></tr></thead>
            <tbody>
              {p.recent.map((r) => (
                <Tr key={r.raceId}>
                  <Td className="text-sm text-muted">{r.date ? formatDate(r.date) : '–'}</Td>
                  <Td>
                    {r.track}
                    {r.type !== 'race' && <span className="ml-2 text-xs text-muted uppercase">{raceTypeLabel(r.type)}</span>}
                    {r.fastestLap && <span className="ml-2 text-xs font-bold text-purple-400" title="Giro più veloce">GV</span>}
                  </Td>
                  <Td className="text-sm text-muted">{r.tournamentName}</Td>
                  <Td num className={r.dnf ? dnfClass : rankClass(r.position)}>{r.dnf ? 'DNF' : r.position ?? '–'}</Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        ) : <Empty>Nessun risultato.</Empty>}
      </Section>
    </>
  );
}

function Tile({ label, value, highlight }: { label: string; value: number | string; highlight?: boolean }) {
  return (
    <div>
      <div className={cx('text-2xl font-extrabold tabular-nums', highlight && 'text-gold')}>{value}</div>
      <div className="text-xs font-bold tracking-wider text-muted uppercase">{label}</div>
    </div>
  );
}

function Section({ title, className, children }: { title: string; className?: string; children: ReactNode }) {
  return (
    <Card className={className}>
      <h2 className="mb-3 font-bold">{title}</h2>
      {children}
    </Card>
  );
}
