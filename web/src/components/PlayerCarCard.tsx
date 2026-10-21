import { useEffect, useRef, useState, type ReactNode } from 'react';
import { controllerLabel, ERS_MODE, FUEL_MIX, severity, TYRE_CLASS } from '../lib/live';
import type { CarSetup, PlayerCar, TyreSet } from '../lib/types';
import type { PedalSample } from '../lib/useLive';
import { CarDiagram } from './CarDiagram';
import { PedalChart } from './PedalChart';
import { Card, cx } from './ui';

// Statistiche dell'auto di un giocatore. Ridotta mostra solo i dati principali;
// un clic la apre a tutta larghezza (sotto la mappa) con tutti i dettagli.
// pedals è lo storico di acceleratore e freno di questa auto per il grafico;
// playerName è il pilota scelto in "Chi guida?", se c'è.
export function PlayerCarCard({
  car,
  playerName,
  pedals,
  pitRejoin = 0,
  className,
}: {
  car: PlayerCar;
  playerName?: string;
  pedals: PedalSample[];
  pitRejoin?: number; // posizione prevista al rientro dai box, 0 se non nota
  className?: string;
}) {
  const [detailed, setDetailed] = useState(false);
  const toggle = () => setDetailed(!detailed);
  const ref = useRef<HTMLDivElement>(null);
  // Aperta, la scheda si sposta (a tutta larghezza, più in basso): la pagina scorre fino a lei.
  useEffect(() => {
    if (detailed) ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, [detailed]);

  return (
    <Card
      ref={ref}
      role="button"
      tabIndex={0}
      aria-expanded={detailed}
      onClick={toggle}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          toggle();
        }
      }}
      className={cx(
        'flex scroll-mt-4 cursor-pointer flex-col gap-4 hover:border-muted', // scroll-mt: un po' di margine sopra quando ci si scorre
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
        // Ridotta sta accanto alla mappa; dettagliata occupa una riga intera (col-span-full nella griglia delle schede).
        detailed ? 'col-span-full w-full' : 'min-w-72 flex-1',
        className,
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-bold">
          <span className="text-accent-text">{controllerLabel(car.controller)}</span>
          {playerName && ` · ${playerName}`}
        </h2>
        <div className="flex items-center gap-2">
          {car.pitLimiter && <Badge className="bg-sprint-bg text-sprint">Limitatore</Badge>}
          {car.drs ? (
            <Badge className="bg-green-950 text-green-400">DRS aperto</Badge>
          ) : (
            car.drsAllowed && <Badge className="bg-surface-2 text-green-400">DRS disponibile</Badge>
          )}
          <span className="text-xs text-muted">{detailed ? 'Riduci ▴' : 'Dettagli ▾'}</span>
        </div>
      </div>
      {detailed ? <Details car={car} pedals={pedals} pitRejoin={pitRejoin} /> : <Summary car={car} pitRejoin={pitRejoin} />}
    </Card>
  );
}

// Vista ridotta: velocità, carburante, ERS, usura delle gomme e danno peggiore.
function Summary({ car, pitRejoin }: { car: PlayerCar; pitRejoin: number }) {
  if (car.restricted) {
    return (
      <>
        <SpeedGear car={car} />
        <RestrictedNote controller={car.controller} />
      </>
    );
  }
  const [worstPart, worstDamage] = damageList(car).reduce((a, b) => (b[1] > a[1] ? b : a));
  const worstWear = Math.max(...car.wheels.map((w) => w.wear));

  return (
    <>
      <SpeedGear car={car} />
      <Bar label="Batteria ERS" value={`${Math.round(car.ersPercent)}%`} ratio={car.ersPercent / 100} color="bg-sprint" />
      <div className="flex flex-col gap-1 text-sm">
        <FuelMargin car={car} />
        <Stat label="Usura gomme (max)" value={`${Math.round(worstWear)}%`} className={severityClass(worstWear, 40, 60)} />
        <Stat
          label="Danni"
          value={worstDamage ? `${worstPart} ${worstDamage}%` : 'Nessuno'}
          className={severityClass(worstDamage, 20, 50)}
        />
        {pitRejoin > 0 && <Stat label="Rientro dai box" value={`P${pitRejoin}`} />}
      </div>
    </>
  );
}

// Vista dettagliata: il disegno dell'auto con gomme e danni, accanto guida, carburante ed ERS.
function Details({ car, pedals, pitRejoin }: { car: PlayerCar; pedals: PedalSample[]; pitRejoin: number }) {
  const d = car.damage;

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <Section title="Gomme e danni">
        <CarDiagram car={car} />
        {(d.drsFault || d.ersFault) && (
          <div className="flex gap-2">
            {d.drsFault && <Badge className="bg-red-950 text-accent-text">Guasto DRS</Badge>}
            {d.ersFault && <Badge className="bg-red-950 text-accent-text">Guasto ERS</Badge>}
          </div>
        )}
      </Section>

      <div className="flex flex-col gap-6">
        <Section title="Guida">
          <SpeedGear car={car} />
          <Bar label="Giri motore" value={car.rpm.toLocaleString('it-IT')} ratio={car.maxRpm ? car.rpm / car.maxRpm : 0} color="bg-sky-500" />
          <PedalChart samples={pedals} />
          <div className="flex flex-col gap-1 text-sm">
            {!car.restricted && <Stat label="Ripartizione freni" value={`${car.brakeBias}%`} />}
            <Stat label="Temp. motore" value={`${car.engineTemp}°C`} />
          </div>
        </Section>
  
        {car.restricted ? (
          <RestrictedNote controller={car.controller} />
        ) : (
          <Section title="Carburante ed ERS">
            <Bar label="Batteria ERS" value={`${Math.round(car.ersPercent)}%`} ratio={car.ersPercent / 100} color="bg-sprint" />
            <div className="flex flex-col gap-1 text-sm">
              <Stat label="Carburante" value={`${car.fuelKg.toFixed(1)} kg`} />
              <FuelMargin car={car} />
              <Stat label="Miscela" value={FUEL_MIX[car.fuelMix] ?? '–'} />
              <Stat label="Modalità ERS" value={ERS_MODE[car.ersMode] ?? '–'} />
              <Stat label="ERS usata nel giro" value={`${car.ersDeployedMj.toFixed(2)} MJ`} />
              <Stat label="ERS recuperata nel giro" value={`${car.ersHarvestMj.toFixed(2)} MJ`} />
            </div>
          </Section>
        )}
      </div>

      {car.tyreSets.length > 0 && (
        <Section title="Gomme disponibili">
          <TyreSets sets={car.tyreSets} />
          {pitRejoin > 0 && (
            <div className="text-sm">
              <Stat label="Posizione al rientro dai box" value={`P${pitRejoin}`} />
            </div>
          )}
        </Section>
      )}
      {car.setup && (
        <Section title="Assetto">
          <SetupTable setup={car.setup} />
        </Section>
      )}
    </div>
  );
}

// Set di gomme ancora disponibili: mescola, usura e quanto si guadagna o perde
// al giro rispetto al set montato (il primo, evidenziato).
function TyreSets({ sets }: { sets: TyreSet[] }) {
  return (
    <ul className="flex flex-wrap gap-2">
      {sets.map((s, i) => (
        <li
          key={i}
          className={cx(
            'flex items-center gap-2 rounded-md border px-2 py-1 text-sm',
            s.fitted ? 'border-muted bg-surface-2' : 'border-line',
          )}
          title={`${s.lifeSpan} giri di vita rimanenti`}
        >
          <span
            className={cx(
              'inline-flex size-6 items-center justify-center rounded-full border-2 text-xs font-extrabold',
              TYRE_CLASS[s.tyre],
            )}
          >
            {s.tyre}
          </span>
          <span className="flex flex-col leading-tight">
            <span className="tabular-nums">{s.fitted ? 'Montato' : s.wear ? `Usato ${s.wear}%` : 'Nuovo'}</span>
            {s.fitted ? (
              <span className="text-xs text-muted tabular-nums">usura {s.wear}%</span>
            ) : (
              <span className={cx('text-xs tabular-nums', s.deltaMs < 0 ? 'text-green-400' : 'text-muted')}>
                {s.deltaMs > 0 ? '+' : ''}{(s.deltaMs / 1000).toFixed(1)} s/giro
              </span>
            )}
          </span>
        </li>
      ))}
    </ul>
  );
}

// Assetto dell'auto: valori anteriori e posteriori affiancati.
function SetupTable({ setup: s }: { setup: CarSetup }) {
  const rows: [string, string, string][] = [
    ['Ala', String(s.frontWing), String(s.rearWing)],
    ['Campanatura', `${s.frontCamber.toFixed(2)}°`, `${s.rearCamber.toFixed(2)}°`],
    ['Convergenza', `${s.frontToe.toFixed(2)}°`, `${s.rearToe.toFixed(2)}°`],
    ['Sospensioni', String(s.frontSuspension), String(s.rearSuspension)],
    ['Barra antirollio', String(s.frontAntiRollBar), String(s.rearAntiRollBar)],
    ['Altezza da terra', String(s.frontHeight), String(s.rearHeight)],
    ['Pressione gomme sx', `${s.tyrePressures[0]} psi`, `${s.tyrePressures[2]} psi`],
    ['Pressione gomme dx', `${s.tyrePressures[1]} psi`, `${s.tyrePressures[3]} psi`],
  ];
  return (
    <div className="flex flex-col gap-3 text-sm">
      <table className="w-full">
        <thead>
          <tr className="text-xs text-muted">
            <th className="text-left font-normal" />
            <th className="text-right font-normal">Ant.</th>
            <th className="text-right font-normal">Post.</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([label, front, rear]) => (
            <tr key={label}>
              <td className="text-muted">{label}</td>
              <td className="text-right font-semibold tabular-nums">{front}</td>
              <td className="text-right font-semibold tabular-nums">{rear}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="flex flex-col gap-1">
        <Stat label="Differenziale in accelerazione" value={`${s.onThrottle}%`} />
        <Stat label="Differenziale in rilascio" value={`${s.offThrottle}%`} />
        <Stat label="Pressione freni" value={`${s.brakePressure}%`} />
        <Stat label="Ripartizione freni" value={`${s.brakeBias}%`} />
        <Stat label="Freno motore" value={`${s.engineBraking}%`} />
        <Stat label="Carburante alla partenza" value={`${s.fuelLoad.toFixed(1)} kg`} />
      </div>
    </div>
  );
}

function damageList({ damage: d }: PlayerCar): [string, number][] {
  return [
    ['Ala ant. sx', d.frontWingLeft],
    ['Ala ant. dx', d.frontWingRight],
    ['Ala post.', d.rearWing],
    ['Fondo', d.floor],
    ['Diffusore', d.diffuser],
    ['Pance', d.sidepod],
    ['Cambio', d.gearbox],
    ['Motore', d.engine],
  ];
}

// Al posto dei dati che il gioco non invia per il giocatore 2 in split-screen. Al
// giocatore 1 la pagina live li nasconde per mostrare gli stessi dati nelle due schede.
function RestrictedNote({ controller }: { controller: number }) {
  return (
    <p className="text-sm text-muted">
      In split-screen il gioco non invia carburante, ERS, usura delle gomme, danni e assetto del secondo giocatore
      {controller === 1 ? ': per confrontare le auto alla pari non si mostrano nemmeno per il primo.' : '.'}
    </p>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-xs font-bold tracking-wider text-muted uppercase">{title}</h3>
      {children}
    </section>
  );
}

function SpeedGear({ car }: { car: PlayerCar }) {
  return (
    <div className="flex items-end gap-6">
      <div>
        <span className="text-4xl font-extrabold tabular-nums">{car.speed}</span>
        <span className="ml-1 text-sm text-muted">km/h</span>
      </div>
      <div>
        <span className="text-4xl font-extrabold">{gearLabel(car.gear)}</span>
        <span className="ml-1 text-sm text-muted">marcia</span>
      </div>
    </div>
  );
}

function FuelMargin({ car }: { car: PlayerCar }) {
  return (
    <Stat
      label="Margine carburante"
      value={`${car.fuelLaps > 0 ? '+' : ''}${car.fuelLaps.toFixed(2)} giri`}
      className={cx(car.fuelLaps < 0 && 'text-accent-text')}
    />
  );
}

function Bar({ label, value, ratio, color }: { label: string; value: string; ratio: number; color: string }) {
  return (
    <div className="text-sm">
      <div className="flex justify-between">
        <span className="text-muted">{label}</span>
        <span className="tabular-nums">{value}</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-surface-2">
        <div
          className={cx('h-full rounded-full transition-[width] duration-200', color)}
          style={{ width: `${Math.min(Math.max(ratio, 0), 1) * 100}%` }}
        />
      </div>
    </div>
  );
}

function Stat({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted">{label}</span>
      <span className={cx('font-semibold tabular-nums', className)}>{value}</span>
    </div>
  );
}

function Badge({ className, children }: { className: string; children: string }) {
  return <span className={cx('rounded-md px-2 py-0.5 text-xs font-bold', className)}>{children}</span>;
}

function gearLabel(gear: number): string {
  if (gear < 0) return 'R';
  if (gear === 0) return 'N';
  return String(gear);
}

const SEVERITY_CLASS = { ok: '', warn: 'text-sprint', bad: 'text-accent-text' };

function severityClass(value: number, warn: number, bad: number): string {
  return SEVERITY_CLASS[severity(value, warn, bad)];
}
