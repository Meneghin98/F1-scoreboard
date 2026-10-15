import { createContext, use, useCallback, useMemo, useRef, useState, type ReactNode } from 'react';

interface ToastApi {
  toast: (message: string) => void;
  // Esegue un'azione asincrona e mostra l'eventuale errore come toast.
  run: (action: () => Promise<void>) => Promise<void>;
}

const ToastContext = createContext<ToastApi | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<string | null>(null);
  const timer = useRef<number | undefined>(undefined);

  const toast = useCallback((text: string) => {
    setMessage(text);
    clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setMessage(null), 3500);
  }, []);

  const run = useCallback(async (action: () => Promise<void>) => {
    try {
      await action();
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err));
    }
  }, [toast]);

  const value = useMemo(() => ({ toast, run }), [toast, run]);

  return (
    <ToastContext value={value}>
      {children}
      {message && (
        <div
          role="status"
          className="fixed bottom-6 left-1/2 -translate-x-1/2 rounded-md border border-l-4 border-line border-l-accent bg-surface-2 px-5 py-3 shadow-xl shadow-black/40"
        >
          {message}
        </div>
      )}
    </ToastContext>
  );
}

export function useToast(): ToastApi {
  const ctx = use(ToastContext);
  if (!ctx) throw new Error('useToast va usato dentro <ToastProvider>');
  return ctx;
}
