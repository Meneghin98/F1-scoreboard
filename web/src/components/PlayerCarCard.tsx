import { useState, type ReactNode } from 'react';
import { ERS_MODE, FUEL_MIX, severity } from '../lib/live';
import type { PlayerCar } from '../lib/types';
import type { PedalSample } from '../lib/useLive';
import { CarDiagram } from './CarDiagram';
import { PedalChart } from './PedalChart';
import { Card, cx } from './ui';

// Statistiche dell'auto del giocatore. Ridotta mostra solo i dati principali;
// un clic la apre a tutta larghezza (sotto la mappa) con tutti i dettagli.
// pedals è lo storico di acceleratore e freno per il grafico.
export function PlayerCarCard({ car, pedals, className }: { car: PlayerCar; pedals: PedalSample[]; className?: string }) {
  const [detailed, setDetailed] = useState(false);
  const toggle = () => setDetailed(!detailed);

  return (
    <Card
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
        'flex cursor-pointer flex-col gap-4 hover:border-muted',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
        // Ridotta sta accanto alla mappa; dettagliata occupa una riga intera e va sotto.
        detailed ? 'w-full' : 'min-w-72 flex-1',
        className,
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-bold">La tua auto</h2>
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
      {detailed ? <Details car={car} pedals={pedals} /> : <Summary car={car} />}
    </Card>
  );
}

// Vista ridotta: velocità, carburante, ERS, usura delle gomme e danno peggiore.
function Summary({ car }: { car: PlayerCar }) {
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
      </div>
    </>
  );
}

// Vista dettagliata: il disegno dell'auto con gomme e danni, accanto guida, carburante ed ERS.
function Details({ car, pedals }: { car: PlayerCar; pedals: PedalSample[] }) {
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
            <Stat label="Ripartizione freni" value={`${car.brakeBias}%`} />
            <Stat label="Temp. motore" value={`${car.engineTemp}°C`} />
          </div>
        </Section>
  
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
