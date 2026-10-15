import { useCallback, useEffect, useRef, useState, type DependencyList } from 'react';
import { useToast } from './toast';

// Carica dei dati quando cambiano le dipendenze e restituisce una funzione per ricaricarli.
// Gli errori finiscono nel toast; data resta null finché il primo caricamento non è finito.
export function useLoad<T>(load: () => Promise<T>, deps: DependencyList) {
  const { run } = useToast();
  const [data, setData] = useState<T | null>(null);
  const latest = useRef(0);

  // deps sono le dipendenze di load: load cambia a ogni render, quindi non la usiamo come dipendenza.
  const reload = useCallback(() => {
    const request = ++latest.current;
    return run(async () => {
      const result = await load();
      // Se nel frattempo è partita una richiesta più recente, questa risposta è superata.
      if (request === latest.current) setData(result);
    });
  }, deps);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { data, reload };
}
