import { useState, type ReactNode } from 'react';
import { ERS_MODE, FUEL_MIX } from '../lib/live';
import type { LiveWheel, PlayerCar } from '../lib/types';
import { Card, cx } from './ui';

const WHEEL_LABELS = ['Ant. sx', 'Ant. dx', 'Post. sx', 'Post. dx'];

// Statistiche dell'auto del giocatore. Ridotta mostra solo i dati principali;
// un clic la apre a tutta larghezza (sotto la mappa) con tutti i dettagli.
export function PlayerCarCard({ car, className }: { car: PlayerCar; className?: string }) {
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
      {detailed ? <Details car={car} /> : <Summary car={car} />}
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

// Vista dettagliata: tutti i dati, divisi in sezioni affiancate.
function Details({ car }: { car: PlayerCar }) {
  const d = car.damage;

  return (
    <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      <Section title="Guida">
        <SpeedGear car={car} />
        <Bar label="Giri motore" value={car.rpm.toLocaleString('it-IT')} ratio={car.maxRpm ? car.rpm / car.maxRpm : 0} color="bg-sky-500" />
        <Bar label="Acceleratore" value={`${Math.round(car.throttle * 100)}%`} ratio={car.throttle} color="bg-green-500" />
        <Bar label="Freno" value={`${Math.round(car.brake * 100)}%`} ratio={car.brake} color="bg-red-500" />
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

      <Section title="Gomme e freni">
        <div className="grid grid-cols-2 gap-2">
          {car.wheels.map((w, i) => (
            <WheelTile key={i} label={WHEEL_LABELS[i]} wheel={w} />
          ))}
        </div>
      </Section>

      <Section title="Danni">
        <div className="flex flex-col gap-1 text-sm">
          {damageList(car).map(([label, pct]) => (
            <Stat key={label} label={label} value={`${pct}%`} className={severityClass(pct, 20, 50)} />
          ))}
        </div>
        {(d.drsFault || d.ersFault) && (
          <div className="flex gap-2">
            {d.drsFault && <Badge className="bg-red-950 text-accent-text">Guasto DRS</Badge>}
            {d.ersFault && <Badge className="bg-red-950 text-accent-text">Guasto ERS</Badge>}
          </div>
        )}
      </Section>
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

function WheelTile({ label, wheel: w }: { label: string; wheel: LiveWheel }) {
  return (
    <div className="rounded-md bg-surface-2 p-2 text-sm">
      <div className="flex items-baseline justify-between">
        <span className="text-xs text-muted">{label}</span>
        <span className={cx('font-bold tabular-nums', severityClass(w.wear, 40, 60))} title="Usura">
          {Math.round(w.wear)}%
        </span>
      </div>
      <div className="tabular-nums" title="Temperatura superficie / interna">
        {w.surfaceTemp}° / {w.innerTemp}°
      </div>
      <div className="flex justify-between text-xs text-muted tabular-nums">
        <span>{w.pressure.toFixed(1)} psi</span>
        <span title="Temperatura freno" className={severityClass(w.brakeDamage, 20, 50)}>
          Freno {w.brakeTemp}°
        </span>
      </div>
    </div>
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

// Giallo oltre la soglia di attenzione, rosso oltre quella critica.
function severityClass(value: number, warn: number, bad: number): string {
  if (value >= bad) return 'text-accent-text';
  if (value >= warn) return 'text-sprint';
  return '';
}
