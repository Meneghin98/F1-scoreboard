import { useSyncExternalStore } from 'react';
import type { LiveSnapshot } from './types';

// Stato della telemetria in tempo reale (Server-Sent Events). La connessione è
// una sola per tutta la pagina, condivisa da tutti i componenti che usano
// useLive: si apre col primo e si chiude quando non ne resta nessuno.

let snapshot: LiveSnapshot | null = null;
let source: EventSource | null = null;
const listeners = new Set<() => void>();

function update(next: LiveSnapshot | null) {
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
    }
  };
}

// Restituisce null finché non arriva il primo aggiornamento o se il server non è raggiungibile.
export function useLive(): LiveSnapshot | null {
  return useSyncExternalStore(subscribe, () => snapshot);
}
