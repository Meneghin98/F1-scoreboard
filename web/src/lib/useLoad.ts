import { useCallback, useEffect, useRef, useState, type DependencyList } from 'react';
import { useToast } from './toast';
import { useDataVersion } from './useLive';

// Carica dei dati quando cambiano le dipendenze e restituisce una funzione per ricaricarli.
// Gli errori finiscono nel toast; data resta null finché il primo caricamento non è finito.
// I dati si ricaricano da soli anche quando cambiano sul server: una gara salvata
// dalla telemetria, o una modifica fatta da un'altra pagina o da un altro PC.
export function useLoad<T>(load: () => Promise<T>, deps: DependencyList) {
  const { run } = useToast();
  const [data, setData] = useState<T | null>(null);
  const latest = useRef(0);

  const reload = useCallback(
    () => {
      const request = ++latest.current;
      return run(async () => {
        const result = await load();
        // Se nel frattempo è partita una richiesta più recente, questa risposta è superata.
        if (request === latest.current) setData(result);
      });
    },
    // biome-ignore lint/correctness/useExhaustiveDependencies: deps sono le dipendenze di load, che cambia a ogni render e quindi non è tra le dipendenze.
    deps,
  );

  useEffect(() => {
    void reload();
  }, [reload]);

  // La prima versione ricevuta non è un cambiamento: i dati sono appena stati caricati.
  const version = useDataVersion();
  const seen = useRef(version);
  useEffect(() => {
    if (version === undefined) return;
    if (seen.current !== undefined && seen.current !== version) void reload();
    seen.current = version;
  }, [version, reload]);

  return { data, reload };
}
