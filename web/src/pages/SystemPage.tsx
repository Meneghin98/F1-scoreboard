import { Fragment, type FormEvent, type ReactNode } from 'react';
import { api, autoBackupUrl, backupUrl } from '../lib/api';
import { useConfirm } from '../lib/confirm';
import { formatBytes, formatDateTime } from '../lib/format';
import { isLive, sessionSummary, telemetryProblem } from '../lib/live';
import { href } from '../lib/route';
import { useToast } from '../lib/toast';
import type { BackupInfo, Retention, SystemInfo } from '../lib/types';
import { useLive } from '../lib/useLive';
import { useLoad } from '../lib/useLoad';
import { Button, buttonClass, Card, cx, fieldClass, Loading, Table, Td, Th, Tr } from '../components/ui';

// Una registrazione occupa circa 2 MB al minuto: serve a dire quante ore ci stanno ancora.
const RECORDING_BYTES_PER_HOUR = 2 * 1024 ** 2 * 60;
// Sotto questi valori di spazio libero la barra cambia colore e compare un avviso.
const DISK_WARNING_BYTES = 5 * 1024 ** 3;
const DISK_CRITICAL_BYTES = 1024 ** 3;

// Pagina Sistema: backup del database, spazio su disco, conservazione delle
// registrazioni, stato della telemetria e versione del programma.
export function SystemPage() {
  const { data, reload } = useLoad(() => Promise.all([api.system(), api.backups(), api.settings()]), []);
  if (!data) return <Loading />;
  const [system, backups, settings] = data;

  return (
    <>
      <BackupCard system={system} backups={backups} />
      <div className="grid gap-4 lg:grid-cols-2">
        <DiskCard system={system} />
        <TelemetryCard system={system} />
        <RetentionCard retention={settings.recordings} onSaved={reload} />
        <VersionCard version={system.version} />
      </div>
    </>
  );
}

function BackupCard({ system, backups }: { system: SystemInfo; backups: BackupInfo[] }) {
  const { dir, keep } = system.backups;
  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 flex-[1_1_480px]">
          <h2 className="mb-1 font-bold">Backup</h2>
          <p className="text-sm text-muted">
            Una copia completa del database (tornei, gare, piloti, statistiche e impostazioni) fatta adesso: si può
            scaricare anche mentre si gioca. Le registrazioni della telemetria non sono comprese: per conservarle copia
            la loro cartella.
          </p>
        </div>
        <a href={backupUrl} download className={buttonClass()}>Scarica backup</a>
      </div>

      <h3 className="mt-5 mb-1 text-xs font-bold tracking-wider text-muted uppercase">Backup automatici</h3>
      {keep > 0 ? (
        <p className="mb-2 text-sm text-muted">
          Il server ne fa uno al giorno in <Path>{dir}</Path>, tenendo gli ultimi {keep}.
        </p>
      ) : (
        <p className="mb-2 text-sm text-muted">Disattivati: il server è avviato con <code>BACKUP_KEEP=0</code>.</p>
      )}
      {backups.length > 0 && (
        <Table>
          <thead>
            <tr><Th>Data</Th><Th num>Dimensione</Th><Th /></tr>
          </thead>
          <tbody>
            {backups.map((b) => (
              <Tr key={b.name}>
                <Td>{formatDateTime(b.createdAt)}</Td>
                <Td num>{formatBytes(b.sizeBytes)}</Td>
                <Td className="text-right">
                  <a href={autoBackupUrl(b.name)} download className={buttonClass('ghost', 'sm')}>Scarica</a>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      <p className="mt-3 text-sm text-muted">
        Per ripristinare un backup: ferma il server, sostituisci con il backup il file <Path>{system.database.path}</Path>{' '}
        (con lo stesso nome) ed elimina, se ci sono, i file che finiscono con <code>-wal</code> e <code>-shm</code> accanto.
      </p>
    </Card>
  );
}

// Spazio sul disco della cartella dei dati: una barra con lo spazio usato,
// che cambia colore quando lo spazio libero scarseggia, e cosa occupa il segnapunti.
function DiskCard({ system }: { system: SystemInfo }) {
  const { disk, database, recordings, backups } = system;
  const parts: [string, string, number][] = [
    ['Database', database.path, database.sizeBytes],
    [recordings.files === 1 ? '1 registrazione' : `${recordings.files} registrazioni`, recordings.dir, recordings.sizeBytes],
    [backups.count === 1 ? '1 backup automatico' : `${backups.count} backup automatici`, backups.dir, backups.sizeBytes],
  ];

  return (
    <Card className="mb-0">
      <h2 className="mb-3 font-bold">Spazio su disco</h2>
      {disk ? <DiskMeter free={disk.freeBytes} total={disk.totalBytes} /> : (
        <p className="text-sm text-muted">Lo spazio libero non si può leggere su questo sistema operativo.</p>
      )}
      <dl className="mt-4 grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-2 text-sm">
        {parts.map(([label, path, size]) => (
          <Fragment key={path}>
            <dt className="min-w-0">
              {label}
              <span className="block truncate text-xs text-muted" title={path}>{path}</span>
            </dt>
            <dd className="text-right tabular-nums">{formatBytes(size)}</dd>
          </Fragment>
        ))}
      </dl>
    </Card>
  );
}

// Colori della barra per gravità: il riempimento dice lo stato, la traccia è una
// tinta più scura dello stesso colore, così lo stato si legge su tutta la barra.
const METER_LEVELS = {
  ok: { fill: '#3987e5', track: '#0d366b' },
  warning: { fill: '#fab219', track: '#3b2a00' },
  critical: { fill: '#d03b3b', track: '#450a0a' },
};

function DiskMeter({ free, total }: { free: number; total: number }) {
  const used = Math.max(0, total - free);
  const percent = total ? (used / total) * 100 : 0;
  const level = free < DISK_CRITICAL_BYTES ? 'critical' : free < DISK_WARNING_BYTES ? 'warning' : 'ok';
  const colors = METER_LEVELS[level];
  const hours = Math.floor(free / RECORDING_BYTES_PER_HOUR);

  return (
    <>
      <div className="mb-1.5 flex flex-wrap items-baseline justify-between gap-x-4 text-sm">
        <span>
          <span className="text-lg font-bold">{formatBytes(free)}</span> liberi su {formatBytes(total)}
        </span>
        <span className="text-muted">{Math.round(percent)}% usato</span>
      </div>
      <div
        className="h-2 overflow-hidden rounded"
        style={{ background: colors.track }}
        title={`Usati ${formatBytes(used)} su ${formatBytes(total)}`}
      >
        <div className="h-full rounded" style={{ width: `${percent}%`, background: colors.fill }} />
      </div>
      <p className="mt-1.5 text-sm text-muted">
        Ci stanno ancora circa {hours === 1 ? "un'ora" : `${hours.toLocaleString('it-IT')} ore`} di registrazioni.
      </p>
      {level !== 'ok' && (
        <p className="mt-2 flex items-start gap-2 text-sm">
          <span aria-hidden style={{ color: colors.fill }}>⚠</span>
          <span>
            <span className="font-semibold">{level === 'critical' ? 'Spazio quasi finito.' : 'Spazio in esaurimento.'}</span>{' '}
            Elimina le registrazioni che non servono o scegli dopo quanti giorni eliminarle, qui sotto.
          </span>
        </p>
      )}
    </>
  );
}

function TelemetryCard({ system }: { system: SystemInfo }) {
  const live = useLive();
  const t = live?.telemetry;
  const problem = telemetryProblem(t);
  let reception: ReactNode = 'In avvio…';
  if (!live) reception = <span className="text-accent-text">Server non raggiungibile</span>;
  else if (problem) reception = <span className="text-accent-text">{problem.short}</span>;
  else if (t?.port) reception = `In ascolto sulla porta UDP ${t.port}`;

  const rows: [string, ReactNode][] = [
    ['Ricezione', reception],
    ['IP da impostare nel gioco', t?.addresses.length ? t.addresses.join(', ') : '–'],
    [
      'Sessione',
      live && isLive(live)
        ? <a href={href('live')} className="underline hover:text-fg">{sessionSummary(live)}</a>
        : 'Nessuna sessione in corso',
    ],
    [
      'Registrazione',
      system.recordings.enabled ? 'Ogni sessione viene registrata' : <>Disattivata (<code>TELEMETRY_RECORD=false</code>)</>,
    ],
  ];

  return (
    <Card className="mb-0">
      <h2 className="mb-3 font-bold">Telemetria</h2>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
        {rows.map(([label, value]) => (
          <Fragment key={label}>
            <dt className="text-muted">{label}</dt>
            <dd>{value}</dd>
          </Fragment>
        ))}
      </dl>
      {problem && <p className="mt-3 rounded-md border border-accent/60 bg-accent/10 px-3 py-2 text-sm">{problem.long}</p>}
    </Card>
  );
}

// Dopo quanti giorni eliminare i file delle registrazioni. Le nuove impostazioni
// valgono subito: se possono eliminare dei file, prima si chiede conferma.
function RetentionCard({ retention, onSaved }: { retention: Retention; onSaved: () => Promise<void> }) {
  const { run, toast } = useToast();
  const confirm = useConfirm();

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const next: Retention = { days: Number(form.get('days')), keepTournaments: form.has('keepTournaments') };
    const stricter = next.days > 0 &&
      (retention.days === 0 || next.days < retention.days || (retention.keepTournaments && !next.keepTournaments));
    if (stricter) {
      const ok = await confirm({
        title: 'Eliminare le registrazioni vecchie?',
        message: `I file delle registrazioni finite da più di ${next.days === 1 ? 'un giorno' : `${next.days} giorni`}` +
          `${next.keepTournaments ? ', tranne quelle delle gare dei tornei,' : ''} verranno eliminati subito, e poi ogni ora. ` +
          'Sessioni, risultati, statistiche e record restano.',
        confirmLabel: 'Salva ed elimina',
      });
      if (!ok) return;
    }
    run(async () => {
      const saved = await api.saveSettings({ recordings: next });
      const n = saved.deletedFiles;
      toast(n === 0 ? 'Impostazioni salvate'
        : `${n === 1 ? 'Eliminato il file di una registrazione' : `Eliminati i file di ${n} registrazioni`} (${formatBytes(saved.freedBytes)})`);
      await onSaved();
    });
  }

  return (
    <Card className="mb-0">
      <h2 className="mb-1 font-bold">Conservazione delle registrazioni</h2>
      <p className="mb-3 text-sm text-muted">
        Una registrazione occupa circa 2 MB al minuto. Dopo i giorni scelti il server ne elimina il file: sessione,
        risultati, statistiche e record restano, ma non si può più rivedere né scaricare.
      </p>
      {/* key: se le impostazioni cambiano (anche da un altro PC) il modulo riparte dai nuovi valori. */}
      <form key={`${retention.days}-${retention.keepTournaments}`} onSubmit={handleSubmit} className="flex flex-col gap-3">
        <label className="flex flex-wrap items-center gap-2 text-sm">
          Elimina i file dopo
          <input
            type="number"
            name="days"
            min={0}
            max={3650}
            step={1}
            required
            defaultValue={retention.days}
            className={cx(fieldClass, 'w-24')}
          />
          giorni <span className="text-muted">(0 = mai)</span>
        </label>
        <label className="flex items-start gap-2 text-sm">
          <input
            type="checkbox"
            name="keepTournaments"
            defaultChecked={retention.keepTournaments}
            className="mt-0.5 size-4 accent-accent"
          />
          <span>Tieni sempre quelle delle gare dei tornei <span className="text-muted">(non di "Nessun torneo")</span></span>
        </label>
        <div>
          <Button>Salva</Button>
        </div>
      </form>
    </Card>
  );
}

function VersionCard({ version: v }: { version: SystemInfo['version'] }) {
  const built = v.time ? new Date(v.time).toLocaleString('it-IT', { dateStyle: 'medium', timeStyle: 'short' }) : '';
  return (
    <Card className="mb-0">
      <h2 className="mb-3 font-bold">Versione</h2>
      {v.revision ? (
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
          <dt className="text-muted">Commit</dt>
          <dd>
            <code>{v.revision.slice(0, 12)}</code>
            {v.modified && <span className="ml-2 text-muted">con modifiche non committate</span>}
          </dd>
          <dt className="text-muted">Data del commit</dt>
          <dd>{built}</dd>
          <dt className="text-muted">Go</dt>
          <dd>{v.go}</dd>
        </dl>
      ) : (
        <p className="text-sm text-muted">
          Sconosciuta: l'eseguibile è stato compilato senza le informazioni di Git (per esempio con <code>go run</code>).
          {v.go && ` Go ${v.go}.`}
        </p>
      )}
    </Card>
  );
}

// Percorso di un file o di una cartella del server, che può essere lungo.
function Path({ children }: { children: ReactNode }) {
  return <code className="break-all text-fg">{children}</code>;
}
