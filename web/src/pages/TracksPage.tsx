import { useState, type FormEvent, type ReactNode } from 'react';
import { PlayerName } from '../components/NextPair';
import { TrackOutline } from '../components/TrackMapCard';
import { api } from '../lib/api';
import { href } from '../lib/route';
import type { Player, RotationState, Track, TrackMap } from '../lib/types';
import { useLoad } from '../lib/useLoad';
import { useToast } from '../lib/toast';
import { Button, Card, cx, fieldSmClass, Loading, Switch } from '../components/ui';

// Gare proposte di default per un programma nuovo, se le piste bastano.
const DEFAULT_RACES = 6;

// Piste: il programma dei prossimi Gran Premi del torneo in corso, con chi
// corre secondo la turnazione, e i circuiti da non suggerire.
export function TracksPage() {
  const { run } = useToast();
  const { data: tracks, reload: reloadTracks } = useLoad(api.tracks, []);
  const { data: rotation, reload: reloadRotation } = useLoad(api.rotation, []);
  const { data: players } = useLoad(api.players, []);
  // Piante dei circuiti già corsi: le altre piste restano senza disegno.
  const { data: maps } = useLoad(api.trackMaps, []);

  function toggleExcluded(track: Track) {
    if (!tracks) return;
    const excluded = tracks.filter((t) => t.excluded !== (t.id === track.id)).map((t) => t.id);
    run(async () => {
      await api.setExcludedTracks(excluded);
      await reloadTracks();
    });
  }

  if (!tracks || !rotation || !players) return <Loading />;
  return (
    <>
      <PlanCard rotation={rotation} tracks={tracks} maps={maps ?? {}} players={players} onChanged={reloadRotation} />
      <TracksCard tracks={tracks} maps={maps ?? {}} onToggle={toggleExcluded} />
    </>
  );
}

// Programma del torneo in corso: si genera scegliendo quante gare correre.
function PlanCard({ rotation, tracks, maps, players, onChanged }: {
  rotation: RotationState;
  tracks: Track[];
  maps: Record<number, TrackMap>;
  players: Player[];
  onChanged: () => Promise<void>;
}) {
  const { run } = useToast();
  const [busy, setBusy] = useState(false);
  const available = tracks.filter((t) => !t.excluded).length;
  const trackName = (id: number) => tracks.find((t) => t.id === id)?.name ?? `Circuito ${id}`;

  function change(action: () => Promise<unknown>) {
    setBusy(true);
    run(async () => {
      await action();
      await onChanged();
    }).finally(() => setBusy(false));
  }

  function handleGenerate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const races = Number(new FormData(e.currentTarget).get('races'));
    change(() => api.generatePlan(races));
  }

  let body: ReactNode;
  if (!rotation.tournament) {
    body = <p className="text-sm text-muted">Nessun torneo in corso: il programma delle piste si genera nel torneo in corso.</p>;
  } else if (rotation.plan.length === 0) {
    body = <p className="text-sm text-muted">Scegli quante gare correre: le piste le estrae l'app tra quelle da giocare.</p>;
  } else {
    body = (
      <>
        {!rotation.enabled ? (
          <p className="mb-2 text-sm text-muted">La turnazione è spenta in questo torneo: i controller si scelgono in Live.</p>
        ) : !rotation.plan[0].pair && (
          <p className="mb-2 text-sm text-muted">
            Segna almeno due piloti presenti nella pagina <a href={href('players')} className="underline">Piloti</a> per
            sapere chi corre.
          </p>
        )}
        <ol className="flex flex-col divide-y divide-line">
          {rotation.plan.map((race, i) => (
            <li key={i} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-2 text-sm">
              <span className={cx('w-16 shrink-0 font-bold', i === 0 ? 'text-accent' : 'text-muted')}>
                {i === 0 ? 'Prossimo' : `GP ${i + 1}`}
              </span>
              <Thumb map={maps[race.track]} className="size-16 text-fg" />
              <span className="min-w-48 flex-1 font-semibold">{trackName(race.track)}</span>
              {race.pair && (
                <span className="flex flex-wrap items-center gap-x-3">
                  <PlayerName player={players.find((p) => p.id === race.pair?.p1)} controller={1} />
                  <PlayerName player={players.find((p) => p.id === race.pair?.p2)} controller={2} />
                </span>
              )}
            </li>
          ))}
        </ol>
      </>
    );
  }

  return (
    <Card>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-bold">
          Programma{rotation.tournament && <span className="font-normal text-muted"> · {rotation.tournament.name}</span>}
        </h2>
        {rotation.tournament && (
          <form onSubmit={handleGenerate} className="flex items-center gap-2">
            <label className="text-sm text-muted" htmlFor="plan-races">Gare</label>
            <input
              id="plan-races"
              name="races"
              type="number"
              min={1}
              max={available}
              step={1}
              required
              disabled={available === 0}
              defaultValue={rotation.plan.length || Math.min(DEFAULT_RACES, available)}
              className={cx(fieldSmClass, 'w-20')}
            />
            <Button size="sm" disabled={busy || available === 0}>
              {rotation.plan.length ? 'Genera di nuovo' : 'Genera piste'}
            </Button>
            {rotation.plan.length > 0 && (
              <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => change(api.clearPlan)}>
                Cancella
              </Button>
            )}
          </form>
        )}
      </div>
      {body}
    </Card>
  );
}

// Circuiti di F1 25: quelli spenti non vengono mai suggeriti, in nessun torneo.
function TracksCard({ tracks, maps, onToggle }: {
  tracks: Track[];
  maps: Record<number, TrackMap>;
  onToggle: (track: Track) => void;
}) {
  const available = tracks.filter((t) => !t.excluded).length;
  return (
    <Card>
      <h2 className="font-bold">
        Circuiti <span className="font-normal text-muted">· {available} di {tracks.length} da giocare</span>
      </h2>
      <p className="mb-3 text-sm text-muted">Spegni le piste che non volete giocare: il programma non le propone più.</p>
      {/* Una scheda per circuito con il disegno grande, nell'ordine del calendario. */}
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {tracks.map((t) => (
          <li key={t.id} className={cx('flex flex-col gap-2 rounded-lg border border-line bg-surface-2 p-3', t.excluded && 'opacity-50')}>
            <div className="flex h-28 items-center justify-center">
              {maps[t.id] ? (
                <TrackOutline map={maps[t.id]} className="size-full text-fg" />
              ) : (
                <span className="text-xs text-muted" title="Il disegno arriva con la prima gara su questa pista">Mai corsa</span>
              )}
            </div>
            <Switch checked={!t.excluded} onChange={() => onToggle(t)} label={t.name} className="text-left" />
          </li>
        ))}
      </ul>
    </Card>
  );
}

// Disegno del circuito nel programma; senza pianta salvata lascia lo spazio vuoto, così le righe restano allineate.
function Thumb({ map, className }: { map?: TrackMap; className: string }) {
  return map ? <TrackOutline map={map} className={cx('shrink-0', className)} /> : <span className={cx('shrink-0', className)} />;
}
