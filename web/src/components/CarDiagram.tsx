import { severity, type Severity } from '../lib/live';
import type { LiveWheel, PlayerCar } from '../lib/types';

// Coordinate del disegno (viewBox 400×380): l'auto vista dall'alto, muso in
// su, centrata in x = 200. Ai lati restano le colonne per le etichette.
const BODY = '#3a3a4c';
const PART_FILL: Record<Severity, string> = { ok: '#5b5b72', warn: '#ffc940', bad: '#dc2626' };
const TEXT_FILL: Record<Severity, string> = { ok: 'var(--color-fg)', warn: '#ffc940', bad: '#f05a50' };

// Soglie in percentuale come nel resto della scheda.
const damageLevel = (pct: number) => severity(pct, 20, 50);
const wearLevel = (pct: number) => severity(pct, 40, 60);

// Temperatura interna delle gomme (°C): blu sotto FREDDA, verde intorno a
// IDEALE, rosso sopra CALDA. Valori indicativi per F1 25, da affinare in gioco.
const TYRE_COLD = 70;
const TYRE_IDEAL = 90;
const TYRE_HOT = 110;

function tyreTempColor(temp: number): string {
  const lerp = (from: number, to: number, t: number) => from + (to - from) * Math.min(Math.max(t, 0), 1);
  const hue =
    temp <= TYRE_IDEAL
      ? lerp(210, 120, (temp - TYRE_COLD) / (TYRE_IDEAL - TYRE_COLD))
      : lerp(120, 0, (temp - TYRE_IDEAL) / (TYRE_HOT - TYRE_IDEAL));
  return `hsl(${Math.round(hue)} 70% 45%)`;
}

// Posizione delle ruote (ant. sx, ant. dx, post. sx, post. dx) e lato delle etichette.
const WHEELS = [
  { x: 120, y: 55, w: 30, h: 60, side: 'left' },
  { x: 250, y: 55, w: 30, h: 60, side: 'right' },
  { x: 114, y: 272, w: 36, h: 70, side: 'left' },
  { x: 250, y: 272, w: 36, h: 70, side: 'right' },
] as const;

// Pianta dell'auto del giocatore: ogni parte è colorata in base al danno e
// collegata a un'etichetta con il valore. Le gomme sono riempite col colore
// della temperatura e bordate con quello dell'usura; accanto i valori.
export function CarDiagram({ car }: { car: PlayerCar }) {
  const d = car.damage;
  const fill = (pct: number) => PART_FILL[damageLevel(pct)];

  // Etichette dei danni: [nome, valore, punto della parte, lato, riga].
  const callouts: [string, number, [number, number], 'left' | 'right', number][] = [
    ['Ala ant. sx', d.frontWingLeft, [165, 29], 'left', 150],
    ['Pance', d.sidepod, [162, 205], 'left', 185],
    ['Fondo', d.floor, [141, 245], 'left', 220],
    ['Cambio', d.gearbox, [200, 312], 'left', 255],
    ['Ala ant. dx', d.frontWingRight, [235, 29], 'right', 150],
    ['Motore', d.engine, [200, 265], 'right', 185],
    ['Diffusore', d.diffuser, [200, 336], 'right', 220],
    ['Ala post.', d.rearWing, [200, 360], 'right', 255],
  ];

  return (
    <div className="mx-auto w-full max-w-lg">
      <svg viewBox="0 0 400 380" className="block w-full" role="img" aria-label="Stato dell'auto">
        {/* Linee delle etichette: disegnate per prime, così passano sotto l'auto. */}
        {callouts.map(([name, , [px, py], side, y]) => (
          <polyline
            key={name}
            points={`${side === 'left' ? 100 : 300},${y} ${px},${py}`}
            fill="none"
            stroke="var(--color-line)"
            strokeWidth={1}
          />
        ))}
  
        {/* Bracci delle sospensioni */}
        <g stroke={BODY} strokeWidth={3}>
          <line x1={150} y1={80} x2={186} y2={95} />
          <line x1={250} y1={80} x2={214} y2={95} />
          <line x1={150} y1={300} x2={184} y2={290} />
          <line x1={250} y1={300} x2={216} y2={290} />
        </g>
  
        {/* Fondo, sotto a tutto il resto: tenue, perché è grande e coprirebbe le altre parti */}
        <path
          d="M150 140 H250 L264 170 V325 H136 V170 Z"
          fill={fill(d.floor)}
          fillOpacity={0.25}
          stroke={fill(d.floor)}
          strokeWidth={2}
        >
          <title>{`Fondo: ${d.floor}%`}</title>
        </path>
  
        {/* Ala anteriore, metà sinistra e destra */}
        <rect x={130} y={20} width={70} height={18} rx={3} fill={fill(d.frontWingLeft)}>
          <title>{`Ala anteriore sinistra: ${d.frontWingLeft}%`}</title>
        </rect>
        <rect x={200} y={20} width={70} height={18} rx={3} fill={fill(d.frontWingRight)}>
          <title>{`Ala anteriore destra: ${d.frontWingRight}%`}</title>
        </rect>
  
        {/* Muso e abitacolo */}
        <path d="M193 38 H207 L218 130 H182 Z" fill={BODY} />
        <rect x={180} y={128} width={40} height={124} rx={10} fill={BODY} />
        <ellipse cx={200} cy={168} rx={11} ry={20} fill="var(--color-bg)" />
  
        {/* Pance */}
        <rect x={146} y={165} width={32} height={88} rx={10} fill={fill(d.sidepod)}>
          <title>{`Pance: ${d.sidepod}%`}</title>
        </rect>
        <rect x={222} y={165} width={32} height={88} rx={10} fill={fill(d.sidepod)}>
          <title>{`Pance: ${d.sidepod}%`}</title>
        </rect>
  
        {/* Motore, cambio e diffusore */}
        <rect x={184} y={240} width={32} height={52} rx={6} fill={fill(d.engine)}>
          <title>{`Motore: ${d.engine}%`}</title>
        </rect>
        <rect x={190} y={294} width={20} height={36} rx={4} fill={fill(d.gearbox)}>
          <title>{`Cambio: ${d.gearbox}%`}</title>
        </rect>
        <path d="M164 330 H236 L244 346 H156 Z" fill={fill(d.diffuser)}>
          <title>{`Diffusore: ${d.diffuser}%`}</title>
        </path>
  
        {/* Ala posteriore */}
        <rect x={140} y={352} width={120} height={18} rx={3} fill={fill(d.rearWing)}>
          <title>{`Ala posteriore: ${d.rearWing}%`}</title>
        </rect>
  
        {WHEELS.map((pos, i) => (
          <Wheel key={i} wheel={car.wheels[i]} {...pos} />
        ))}
  
        {/* Etichette dei danni, con un punto sulla parte indicata */}
        {callouts.map(([name, pct, [px, py], side, y]) => {
          const x = side === 'left' ? 96 : 304;
          const anchor = side === 'left' ? 'end' : 'start';
          return (
            <g key={name}>
              <circle cx={px} cy={py} r={2.5} fill="var(--color-fg)" />
              <text x={x} y={y - 3} textAnchor={anchor} fontSize={10} fill="var(--color-muted)">
                {name}
              </text>
              <text x={x} y={y + 11} textAnchor={anchor} fontSize={13} fontWeight={700} fill={TEXT_FILL[damageLevel(pct)]}>
                {pct}%
              </text>
            </g>
          );
        })}
      </svg>
      <TyreTempLegend />
    </div>
  );
}

// Legenda dei colori delle gomme: la barra va da 60 a 120 °C.
function TyreTempLegend() {
  const at = (temp: number) => `${tyreTempColor(temp)} ${((temp - 60) / 60) * 100}%`;
  return (
    <div className="mt-2 text-xs text-muted">
      <div className="mb-1">Temperatura interna delle gomme</div>
      <div
        className="h-1.5 rounded-full"
        style={{ background: `linear-gradient(to right, ${at(TYRE_COLD)}, ${at(TYRE_IDEAL)}, ${at(TYRE_HOT)})` }}
      />
      <div className="mt-1 flex justify-between">
        <span>Fredda &lt; {TYRE_COLD}°</span>
        <span>Ideale ~{TYRE_IDEAL}°</span>
        <span>Calda &gt; {TYRE_HOT}°</span>
      </div>
    </div>
  );
}

// Una ruota: riempita col colore della temperatura interna, bordo col colore
// dell'usura, dati scritti a fianco.
function Wheel({ wheel: w, x, y, w: width, h, side }: { wheel: LiveWheel; x: number; y: number; w: number; h: number; side: 'left' | 'right' }) {
  const level = wearLevel(w.wear);
  const tx = side === 'left' ? x - 8 : x + width + 8;
  const anchor = side === 'left' ? 'end' : 'start';
  const cy = y + h / 2;

  return (
    <g>
      <title>{`Usura ${Math.round(w.wear)}% · interna ${w.innerTemp}° · superficie ${w.surfaceTemp}°`}</title>
      {/* Bordo dell'usura e, staccato da una striscia scura, il riempimento della temperatura. */}
      <rect x={x} y={y} width={width} height={h} rx={7} fill="#1a1a22" stroke={PART_FILL[level]} strokeWidth={3} />
      <rect x={x + 4} y={y + 4} width={width - 8} height={h - 8} rx={4} fill={tyreTempColor(w.innerTemp)} />
      <text textAnchor={anchor} fontSize={10} fill="var(--color-muted)">
        <tspan x={tx} y={cy - 16} fontSize={13} fontWeight={700} fill={TEXT_FILL[level]}>
          {Math.round(w.wear)}%
        </tspan>
        <tspan x={tx} y={cy - 2} fill="var(--color-fg)">
          {w.surfaceTemp}° / {w.innerTemp}°
        </tspan>
        <tspan x={tx} y={cy + 11}>{w.pressure.toFixed(1)} psi</tspan>
        <tspan x={tx} y={cy + 24} fill={TEXT_FILL[damageLevel(w.brakeDamage)]}>
          Freno {w.brakeTemp}°
        </tspan>
      </text>
    </g>
  );
}
