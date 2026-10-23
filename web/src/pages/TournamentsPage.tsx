import { useState } from 'react';
import { TournamentDialog } from '../components/TournamentDialog';
import { Button, Card, ColorDot, cx } from '../components/ui';
import { api } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { rulesSummary } from '../lib/f1';
import { formatUtcDate } from '../lib/format';
import { navigate } from '../lib/route';
import { useToast } from '../lib/toast';
import type { Tournament } from '../lib/types';

interface Props {
  tournaments: Tournament[];
  onChanged: () => Promise<void>; // ricarica l'elenco dopo una modifica
}

// Tutti i tornei con il loro stato: qui si creano, si iniziano, si terminano,
// si cambiano nome e regole di punteggio e si eliminano.
export function TournamentsPage({ tournaments, onChanged }: Props) {
  const { run, toast } = useToast();
  const confirm = useConfirm();
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Tournament | null>(null);
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


  return (
    <>
      <Card className="flex flex-wrap items-center justify-between gap-4">
        <p className="text-sm text-muted">
          Le gare finite vengono salvate nel torneo in corso; se nessun torneo è in corso, in "Nessun torneo".
        </p>
        <Button onClick={() => setCreating(true)}>+ Nuovo torneo</Button>
      </Card>

      {tournaments.map((t) => (
        <Card key={t.id} className={cx(t.active && 'border-green-500/60')}>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <h2 className="flex flex-wrap items-center gap-2 text-lg font-bold">
                {t.name}
                <StatusBadge tournament={t} />
              </h2>
              <p className="text-sm text-muted">{details(t)}</p>
              {!t.free && <p className="text-sm text-muted">{rulesSummary(t)}</p>}
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
              <Button variant="ghost" size="sm" onClick={() => navigate('standings', undefined, { torneo: t.id })}>
                Classifica
              </Button>
              {t.free && (
                <Button variant="ghost" size="sm" onClick={() => navigate('races', undefined, { torneo: t.id })}>
                  Gare
                </Button>
              )}
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
                  <Button variant="ghost" size="sm" title="Nome e regole di punteggio" onClick={() => setEditing(t)}>
                    Impostazioni
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => handleDelete(t)}>Elimina</Button>
                </>
              )}
            </div>
          </div>
        </Card>
      ))}

      <TournamentDialog
        open={creating}
        defaultName={`Torneo ${new Date().toLocaleDateString('it-IT')}`}
        canStart={!active}
        onClose={() => setCreating(false)}
        onSaved={onChanged}
      />
      <TournamentDialog
        open={editing !== null}
        tournament={editing ?? undefined}
        onClose={() => setEditing(null)}
        onSaved={onChanged}
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
