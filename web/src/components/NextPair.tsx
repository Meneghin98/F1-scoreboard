import { api } from '../lib/api';
import type { Player, RotationState } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { ColorDot, cx } from './ui';

// Chi prende i controller al prossimo Gran Premio secondo la turnazione, in una
// riga sola: si mostra tra una sessione e l'altra, in Live e in TV. Senza
// torneo in corso, con la turnazione spenta o con meno di due presenti non mostra nulla.
export function NextPair({ rotation, large, className }: { rotation?: RotationState; large?: boolean; className?: string }) {
  const { data: players } = useLoad(api.players, []);
  const next = rotation?.next[0];
  if (!rotation?.tournament || !next || !players) return null;

  const resting = rotation.present.filter((id) => id !== next.p1 && id !== next.p2);
  return (
    <p className={cx(
      'mb-4 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-line bg-surface px-4 py-2',
      large && 'text-2xl',
      className,
    )}>
      <span className={cx('font-bold tracking-wider text-muted uppercase', large ? 'text-base' : 'text-xs')}>
        Prossimi al controller
      </span>
      <PlayerName player={players.find((p) => p.id === next.p1)} controller={1} />
      <PlayerName player={players.find((p) => p.id === next.p2)} controller={2} />
      {resting.length > 0 && (
        <span className="text-muted">
          · {resting.length === 1 ? 'riposa' : 'riposano'}{' '}
          {resting.map((id) => players.find((p) => p.id === id)?.name).filter(Boolean).join(', ')}
        </span>
      )}
    </p>
  );
}

function PlayerName({ player, controller }: { player?: Player; controller: number }) {
  if (!player) return null;
  return (
    <span className="flex items-center gap-1.5 font-semibold">
      <ColorDot color={player.color} className="size-3" />
      {player.name}
      <span className="font-normal text-muted">({controller})</span>
    </span>
  );
}
