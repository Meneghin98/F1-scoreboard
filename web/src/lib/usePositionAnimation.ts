import { type RefObject, useLayoutEffect, useRef } from 'react';
import type { LiveDriver } from './types';

const MOVE_MS = 400;
const FLASH_MS = 1200;
const GAINED = 'rgb(74 222 128 / 0.3)';
const LOST = 'rgb(248 113 113 / 0.3)';

// Fa scorrere le righe di una classifica quando cambiano posizione e le
// illumina un momento: verde chi guadagna, rosso chi perde. Le righe sono gli
// elementi dentro container con l'attributo data-car (l'indice dell'auto).
// Se cambia posizione più di metà del gruppo (per esempio un salto nel replay)
// le righe si spostano senza animazione.
export function usePositionAnimation(container: RefObject<HTMLElement | null>, drivers: LiveDriver[]) {
  const last = useRef(new Map<number, { top: number; position: number }>());

  // Dopo ogni aggiornamento, prima che la pagina venga disegnata.
  useLayoutEffect(() => {
    const el = container.current;
    if (!el) return;
    const rows = [...el.querySelectorAll<HTMLElement>('[data-car]')];
    const before = last.current;
    const now = new Map<number, { top: number; position: number }>();
    for (const row of rows) {
      const driver = drivers.find((d) => d.carIndex === Number(row.dataset.car));
      // offsetTop non risente delle animazioni in corso, a differenza di getBoundingClientRect.
      if (driver) now.set(driver.carIndex, { top: row.offsetTop, position: driver.position });
    }
    last.current = now;

    const changed = [...now].filter(([car, { position }]) => {
      const prev = before.get(car);
      return prev && prev.position !== position;
    });
    if (!changed.length || changed.length > drivers.length / 2) return;

    for (const row of rows) {
      const car = Number(row.dataset.car);
      const prev = before.get(car), cur = now.get(car);
      if (!prev || !cur) continue;
      const dy = prev.top - cur.top;
      if (dy) {
        row.animate([{ transform: `translateY(${dy}px)` }, { transform: 'none' }], { duration: MOVE_MS, easing: 'ease-out' });
      }
      if (prev.position !== cur.position) {
        // Senza fotogramma finale il colore torna a quello della riga.
        row.animate([{ backgroundColor: cur.position < prev.position ? GAINED : LOST, offset: 0 }], { duration: FLASH_MS, easing: 'ease-in' });
      }
    }
  });
}
