import { api } from '../lib/api';
import type { Player, RotationState } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { ColorDot, cx } from './ui';

// Il prossimo Gran Premio in una riga sola: la pista del programma e chi
// prende i controller secondo la turnazione. Si mostra tra una sessione e
// l'altra, in Live e in TV. Senza torneo in corso, o senza programma e senza
// coppia (turnazione spenta o meno di due presenti), non mostra nulla.
export function NextPair({ rotation, large, className }: { rotation?: RotationState; large?: boolean; className?: string }) {
  const { data: players } = useLoad(api.players, []);
  const { data: tracks } = useLoad(api.tracks, []);
  const next = rotation?.next[0];
  const planned = rotation?.plan[0];
  if (!rotation?.tournament || (!next && !planned) || !players || !tracks) return null;

  const resting = next ? rotation.present.filter((id) => id !== next.p1 && id !== next.p2) : [];
  return (
    <p className={cx(
      'mb-4 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-line bg-surface px-4 py-2',
      large && 'text-2xl',
      className,
    )}>
      <span className={cx('font-bold tracking-wider text-muted uppercase', large ? 'text-base' : 'text-xs')}>
        {planned ? 'Prossimo GP' : 'Prossimi al controller'}
      </span>
      {planned && (
        <span className="font-semibold">
          {tracks.find((t) => t.id === planned.track)?.name}
          {next && <span className="font-normal text-muted"> ·</span>}
        </span>
      )}
      {next && (
        <>
          <PlayerName player={players.find((p) => p.id === next.p1)} controller={1} />
          <PlayerName player={players.find((p) => p.id === next.p2)} controller={2} />
        </>
      )}
      {resting.length > 0 && (
        <span className="text-muted">
          · {resting.length === 1 ? 'riposa' : 'riposano'}{' '}
          {resting.map((id) => players.find((p) => p.id === id)?.name).filter(Boolean).join(', ')}
        </span>
      )}
    </p>
  );
}

// Pilota con il colore e il controller, come "● Marco (1)".
export function PlayerName({ player, controller }: { player?: Player; controller: number }) {
  if (!player) return null;
  return (
    <span className="flex items-center gap-1.5 font-semibold">
      <ColorDot color={player.color} className="size-3" />
      {player.name}
      <span className="font-normal text-muted">({controller})</span>
    </span>
  );
}
