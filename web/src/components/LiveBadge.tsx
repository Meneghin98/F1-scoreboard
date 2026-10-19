import { isLive, sessionSummary, telemetryProblem } from '../lib/live';
import { href } from '../lib/route';
import { useLive } from '../lib/useLive';
import { cx } from './ui';

// Indicatore dello stato della telemetria nell'header: porta alla pagina Live.
export function LiveBadge() {
  const live = useLive();
  if (!live) return null;

  const active = isLive(live);
  const problem = active ? null : telemetryProblem(live.telemetry);
  return (
    <a
      href={href('live')}
      className={cx(
        'flex items-center gap-2 text-sm hover:text-fg',
        active ? 'font-semibold' : problem ? 'font-semibold text-accent-text' : 'text-muted',
      )}
      title={active ? 'Apri la pagina Live' : problem?.long ?? `Il server è in ascolto sulla porta UDP ${live.telemetry?.port || 20777}`}
    >
      <LiveDot active={live.status === 'live'} />
      {active ? sessionSummary(live) : problem?.short ?? 'Telemetria in attesa'}
    </a>
  );
}

export function LiveDot({ active = false }: { active?: boolean }) {
  return (
    <span className="relative flex size-2.5">
      {active && <span className="absolute inline-flex size-full animate-ping rounded-full bg-green-500 opacity-75" />}
      <span className={cx('relative inline-flex size-2.5 rounded-full', active ? 'bg-green-500' : 'bg-muted/60')} />
    </span>
  );
}
