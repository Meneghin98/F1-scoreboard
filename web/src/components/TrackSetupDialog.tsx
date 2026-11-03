import type { TrackSetup } from '../lib/types';
import { Dialog } from './ui';

// Una riga dell'assetto: un valore solo, oppure anteriore e posteriore.
type Row = [label: string, value: string] | [label: string, front: string, rear: string];

const deg = (v: number) => `${v.toFixed(2)}°`;
const pct = (v: number) => `${v}%`;
const psi = (v: number) => `${v.toFixed(1)} psi`;

// Le sezioni nell'ordine del menu assetto di F1 25.
function sections(s: TrackSetup): [title: string, rows: Row[]][] {
  return [
    ['Aerodinamica', [['Ala', String(s.frontWing), String(s.rearWing)]]],
    ['Trasmissione', [
      ['Differenziale in accelerazione', pct(s.onThrottle)],
      ['Differenziale in rilascio', pct(s.offThrottle)],
      ['Freno motore', s.engineBraking === null ? '–' : pct(s.engineBraking)],
    ]],
    ['Geometria sospensioni', [
      ['Campanatura', deg(s.frontCamber), deg(s.rearCamber)],
      ['Convergenza', deg(s.frontToe), deg(s.rearToe)],
    ]],
    ['Sospensioni', [
      ['Sospensioni', String(s.frontSuspension), String(s.rearSuspension)],
      ['Barra antirollio', String(s.frontAntiRollBar), String(s.rearAntiRollBar)],
      ['Altezza da terra', String(s.frontHeight), String(s.rearHeight)],
    ]],
    ['Freni', [
      ['Pressione freni', pct(s.brakePressure)],
      ['Ripartizione freni', pct(s.brakeBias)],
    ]],
    ['Pneumatici', [
      ['Pressione sinistra', psi(s.tyrePressures[0]), psi(s.tyrePressures[2])],
      ['Pressione destra', psi(s.tyrePressures[1]), psi(s.tyrePressures[3])],
    ]],
  ];
}

// Assetto consigliato di un circuito, diviso nelle sezioni del gioco.
export function TrackSetupDialog({ track, setup, onClose }: {
  track: string;
  setup: TrackSetup | null; // null: finestra chiusa
  onClose: () => void;
}) {
  return (
    <Dialog open={setup !== null} title={`Assetto · ${track}`} onClose={onClose}>
      {setup && (
        <div className="flex flex-col gap-4 text-sm">
          {sections(setup).map(([title, rows]) => (
            <section key={title}>
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-muted">
                    <th className="text-left font-bold tracking-wide text-fg uppercase">{title}</th>
                    {rows.some((r) => r.length === 3) && (
                      <>
                        <th className="w-20 text-right font-normal">Ant.</th>
                        <th className="w-20 text-right font-normal">Post.</th>
                      </>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {rows.map(([label, ...values]) => (
                    <tr key={label}>
                      <td className="text-muted">{label}</td>
                      {values.map((v, i) => (
                        <td
                          key={i}
                          colSpan={values.length === 1 && rows.some((r) => r.length === 3) ? 2 : undefined}
                          className="text-right font-semibold tabular-nums"
                        >
                          {v}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          ))}
          {setup.notes && <p className="whitespace-pre-line text-muted">{setup.notes}</p>}
        </div>
      )}
    </Dialog>
  );
}
