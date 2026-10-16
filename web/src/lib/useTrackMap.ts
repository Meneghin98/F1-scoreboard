import { useEffect, useRef, useState } from 'react';
import { api } from './api';
import type { TrackMap } from './types';

// Intervallo minimo tra due download della pianta mentre si sta ancora
// completando (all'inizio della sessione cambia a ogni aggiornamento).
const REFRESH_MS = 2000;

// Scarica la pianta del circuito della sessione uid e la riscarica quando il
// server ne ha ricostruiti altri punti. Restituisce null finché non c'è.
export function useTrackMap(uid: string, points: number): TrackMap | null {
  const [map, setMap] = useState<{ uid: string; data: TrackMap } | null>(null);
  const last = useRef({ uid: '', points: 0, at: 0 });

  useEffect(() => {
    if (!points || (last.current.uid === uid && last.current.points === points)) return;
    const load = async () => {
      last.current = { uid, points, at: Date.now() };
      try {
        const data = await api.liveTrack();
        setMap({ uid, data });
      } catch {
        // Si riprova al prossimo cambiamento: la mappa non è indispensabile.
      }
    };
    const wait = last.current.uid === uid ? last.current.at + REFRESH_MS - Date.now() : 0;
    if (wait <= 0) {
      void load();
      return;
    }
    const timer = setTimeout(load, wait);
    return () => clearTimeout(timer);
  }, [uid, points]);

  return map?.uid === uid ? map.data : null;
}
