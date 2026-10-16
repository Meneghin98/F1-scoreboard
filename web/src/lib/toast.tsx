import { createContext, use, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';

const TOAST_DURATION_MS = 2500;

interface ToastApi {
  toast: (message: string) => void;
  // Esegue un'azione asincrona e mostra l'eventuale errore come toast.
  run: (action: () => Promise<void>) => Promise<void>;
}

const ToastContext = createContext<ToastApi | null>(null);

interface ToastState {
  text: string;
  id: number; // cambia a ogni toast, anche con lo stesso testo, così l'effetto lo rimostra in cima
  visible: boolean; // a fine durata diventa false ma il testo resta, per la dissolvenza in uscita
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ToastState | null>(null);
  const timer = useRef<number | undefined>(undefined);
  const ref = useRef<HTMLDivElement>(null);

  const toast = useCallback((text: string) => {
    setState((prev) => ({ text, id: (prev?.id ?? 0) + 1, visible: true }));
    clearTimeout(timer.current);
    timer.current = window.setTimeout(
      () => setState((prev) => prev && { ...prev, visible: false }),
      TOAST_DURATION_MS,
    );
  }, []);

  const run = useCallback(async (action: () => Promise<void>) => {
    try {
      await action();
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err));
    }
  }, [toast]);

  // Il toast è un popover: come i dialog modali vive nel "top layer" del browser,
  // dove vince l'ultimo elemento mostrato. Nasconderlo e rimostrarlo lo porta
  // sopra un eventuale dialog aperto, cosa che nessuno z-index può fare.
  const id = state?.id;
  const visible = state?.visible ?? false;
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (el.matches(':popover-open')) el.hidePopover();
    if (visible) el.showPopover();
  }, [id, visible]);

  const value = useMemo(() => ({ toast, run }), [toast, run]);

  return (
    <ToastContext value={value}>
      {children}
      <div
        ref={ref}
        popover="manual"
        role="status"
        className={[
          // inset-auto annulla il posizionamento a schermo intero che il browser dà ai popover.
          'fixed inset-auto bottom-6 left-1/2 -translate-x-1/2 rounded-md border border-l-4 border-line border-l-accent',
          'bg-surface-2 px-5 py-3 text-fg shadow-xl shadow-black/40',
          // Dissolvenza: transition-discrete fa durare anche la chiusura del popover
          // (display/overlay), starting: imposta il punto di partenza dell'apertura.
          'opacity-0 transition-[opacity,translate,display,overlay] transition-discrete duration-300',
          'translate-y-2 open:translate-y-0 open:opacity-100 starting:open:translate-y-2 starting:open:opacity-0',
        ].join(' ')}
      >
        {state?.text}
      </div>
    </ToastContext>
  );
}

export function useToast(): ToastApi {
  const ctx = use(ToastContext);
  if (!ctx) throw new Error('useToast va usato dentro <ToastProvider>');
  return ctx;
}
