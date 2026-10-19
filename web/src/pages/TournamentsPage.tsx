import { useState, type FormEvent } from 'react';
import { NewTournamentDialog } from '../components/NewTournamentDialog';
import { Button, Card, ColorDot, cx, fieldClass } from '../components/ui';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { formatUtcDate } from '../lib/format';
import { navigate } from '../lib/route';
import { useToast } from '../lib/toast';
import type { Tournament } from '../lib/types';

interface Props {
  tournaments: Tournament[];
  selectedId: number | null; // torneo scelto in alto
  onSelect: (id: number) => void;
  onChanged: () => Promise<void>; // ricarica l'elenco dopo una modifica
}

// Tutti i tornei con il loro stato: qui si creano, si iniziano, si terminano,
// si rinominano e si eliminano. Nell'header resta solo la scelta del torneo da mostrare.
export function TournamentsPage({ tournaments, selectedId, onSelect, onChanged }: Props) {
  const { run, toast } = useToast();
  const confirm = useConfirm();
  const [creating, setCreating] = useState(false);
  const [renamingId, setRenamingId] = useState<number | null>(null);
  const active = tournaments.find((t) => t.active);

  // Iniziato un torneo, le gare finite vengono salvate lì; terminato, si torna a "Nessun torneo".
  function handleStart(t: Tournament) {
    run(async () => {
      await api.startTournament(t.id);
      await onChanged();
      toast(`"${t.name}" è in corso: le prossime gare verranno salvate qui`);
    });
  }

  async function handleEnd(t: Tournament) {
    const ok = await confirm({
      title: 'Terminare il torneo?',
      message: `Le prossime gare verranno salvate in "Nessun torneo" invece che in "${t.name}". Potrai riprenderlo più avanti.`,
      confirmLabel: 'Termina torneo',
    });
    if (!ok) return;
    run(async () => {
      await api.endTournament(t.id);
      await onChanged();
    });
  }

  async function handleDelete(t: Tournament) {
    const moved = t.recordings === 1
      ? ' La registrazione della sua gara passerà in "Nessun torneo".'
      : t.recordings > 1 ? ` Le ${t.recordings} registrazioni delle sue gare passeranno in "Nessun torneo".` : '';
    const ok = await confirm({
      title: 'Eliminare il torneo?',
      message: `"${t.name}" verrà eliminato insieme a tutte le sue gare e ai risultati.${moved}`,
      confirmLabel: 'Elimina torneo',
    });
    if (!ok) return;
    run(async () => {
      const { movedRecordings } = await api.deleteTournament(t.id);
      await onChanged();
      if (movedRecordings) toast(`${movedRecordings === 1 ? 'Una registrazione è passata' : `${movedRecordings} registrazioni sono passate`} in "Nessun torneo"`);
    });
  }

  function handleRename(t: Tournament, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const name = String(new FormData(e.currentTarget).get('name'));
    run(async () => {
      await api.updateTournament(t.id, name);
      setRenamingId(null);
      await onChanged();
    });
  }

  function openStandings(t: Tournament) {
    onSelect(t.id);
    navigate(t.free ? 'races' : 'standings');
  }

  return (
    <>
      <Card className="flex flex-wrap items-center justify-between gap-4">
        <p className="text-sm text-muted">
          Le gare finite vengono salvate nel torneo in corso; se nessun torneo è in corso, in "Nessun torneo".
        </p>
        <Button onClick={() => setCreating(true)}>+ Nuovo torneo</Button>
      </Card>

      {tournaments.map((t) => (
        <Card key={t.id} className={cx(t.active && 'border-green-500/60', t.id === selectedId && 'bg-surface-2/40')}>
          {renamingId === t.id ? (
            <form className="flex flex-wrap gap-2" onSubmit={(e) => handleRename(t, e)}>
              <input
                name="name"
                required
                pattern=".*\S.*"
                defaultValue={t.name}
                aria-label="Nome del torneo"
                className={cx(fieldClass, 'flex-[1_1_240px]')}
              />
              <Button type="button" variant="ghost" onClick={() => setRenamingId(null)}>Annulla</Button>
              <Button>Salva</Button>
            </form>
          ) : (
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="min-w-0">
                <h2 className="flex flex-wrap items-center gap-2 text-lg font-bold">
                  {t.name}
                  <StatusBadge tournament={t} />
                </h2>
                <p className="text-sm text-muted">{details(t)}</p>
                {t.leader && (
                  <p className="mt-1 flex items-center gap-2 text-sm">
                    <ColorDot color={t.leader.color} />
                    <span className="text-muted">{t.ended_at ? 'Vincitore' : 'In testa'}:</span>
                    <span className="font-semibold">{t.leader.name}</span>
                    <span className="text-muted tabular-nums">{t.leader.points} pt</span>
                  </p>
                )}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="ghost" size="sm" onClick={() => openStandings(t)}>
                  {t.free ? 'Gare' : 'Classifica'}
                </Button>
                {t.active ? (
                  <Button variant="ghost" size="sm" onClick={() => handleEnd(t)}>Termina</Button>
                ) : !t.free && (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={!!active}
                    title={active ? `Termina prima "${active.name}"` : 'Le prossime gare verranno salvate in questo torneo'}
                    onClick={() => handleStart(t)}
                  >
                    {t.ended_at ? 'Riprendi' : 'Inizia'}
                  </Button>
                )}
                {!t.free && (
                  <>
                    <Button variant="ghost" size="sm" onClick={() => setRenamingId(t.id)}>Rinomina</Button>
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(t)}>Elimina</Button>
                  </>
                )}
              </div>
            </div>
          )}
        </Card>
      ))}

      <NewTournamentDialog
        open={creating}
        defaultName={`Torneo F1 2025 #${tournaments.filter((t) => !t.free).length + 1}`}
        canStart={!active}
        onClose={() => setCreating(false)}
        onCreated={async (t) => {
          onSelect(t.id);
          await onChanged();
        }}
      />
    </>
  );
}

function StatusBadge({ tournament: t }: { tournament: Tournament }) {
  const [label, className] = t.free
    ? ['Fuori dai tornei', 'bg-surface-2 text-muted']
    : t.active
      ? ['In corso', 'bg-green-950 text-green-400']
      : t.ended_at
        ? ['Terminato', 'bg-surface-2 text-muted']
        : ['Non iniziato', 'bg-surface-2 text-muted'];
  return <span className={cx('rounded px-2 py-0.5 text-xs font-bold uppercase', className)}>{label}</span>;
}

// "Dal 12/10/2026 al 18/10/2026 · 8 gare · 6 registrazioni".
function details(t: Tournament): string {
  const parts: string[] = [];
  if (t.started_at) {
    parts.push(t.ended_at ? `Dal ${formatUtcDate(t.started_at)} al ${formatUtcDate(t.ended_at)}` : `Iniziato il ${formatUtcDate(t.started_at)}`);
  }
  parts.push(t.races === 1 ? '1 gara' : `${t.races} gare`);
  if (t.recordings) parts.push(t.recordings === 1 ? '1 registrazione' : `${t.recordings} registrazioni`);
  return parts.join(' · ');
}
