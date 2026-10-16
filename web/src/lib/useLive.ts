import { useSyncExternalStore } from 'react';
import type { LiveSnapshot } from './types';

// Stato della telemetria in tempo reale (Server-Sent Events). La connessione è
// una sola per tutta la pagina, condivisa da tutti i componenti che usano
// useLive: si apre col primo e si chiude quando non ne resta nessuno.

// Secondi di storico di acceleratore e freno tenuti per il grafico.
export const PEDAL_WINDOW_MS = 20_000;

// Un campione di acceleratore e freno (0..1) all'istante t (ms, Date.now()).
export interface PedalSample {
  t: number;
  throttle: number;
  brake: number;
}

let snapshot: LiveSnapshot | null = null;
let pedals: PedalSample[] = [];
let source: EventSource | null = null;
const listeners = new Set<() => void>();

function update(next: LiveSnapshot | null) {
  // Lo storico riparte da zero quando cambia la sessione o si perde la connessione.
  if (!next?.player || next.session?.uid !== snapshot?.session?.uid) pedals = [];
  if (next?.player) {
    const now = Date.now();
    pedals = [
      ...pedals.filter((p) => p.t >= now - PEDAL_WINDOW_MS),
      { t: now, throttle: next.player.throttle, brake: next.player.brake },
    ];
  }
  snapshot = next;
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  if (!source) {
    source = new EventSource('/api/live/stream');
    source.onmessage = (e) => update(JSON.parse(e.data));
    // EventSource si riconnette da solo quando il server torna disponibile.
    source.onerror = () => update(null);
  }
  return () => {
    listeners.delete(listener);
    if (!listeners.size) {
      source?.close();
      source = null;
      snapshot = null;
      pedals = [];
    }
  };
}

// Restituisce null finché non arriva il primo aggiornamento o se il server non è raggiungibile.
export function useLive(): LiveSnapshot | null {
  return useSyncExternalStore(subscribe, () => snapshot);
}

// Acceleratore e freno del giocatore negli ultimi PEDAL_WINDOW_MS, dal più vecchio.
export function usePedalHistory(): PedalSample[] {
  return useSyncExternalStore(subscribe, () => pedals);
}
