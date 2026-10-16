import { createContext, use, useCallback, useState, type ReactNode } from 'react';
import { Dialog } from '../components/Dialog';
import { Button } from '../components/ui';

interface ConfirmOptions {
  title: string;
  message: ReactNode;
  confirmLabel?: string;
}

// Mostra il dialog di conferma; la Promise si risolve con true se l'utente conferma.
type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<(ConfirmOptions & { resolve: (ok: boolean) => void }) | null>(null);

  const confirm = useCallback<ConfirmFn>(
    (options) => new Promise((resolve) => setRequest({ ...options, resolve })),
    [],
  );

  function close(ok: boolean) {
    request?.resolve(ok);
    setRequest(null);
  }

  return (
    <ConfirmContext value={confirm}>
      {children}
      <Dialog open={request !== null} title={request?.title ?? ''} onClose={() => close(false)}>
        {request && (
          <>
            <p className="text-muted">{request.message}</p>
            {/* Annulla viene prima: è lui a ricevere il focus, così Invio non elimina per sbaglio. */}
            <div className="mt-6 flex justify-end gap-2">
              <Button variant="ghost" onClick={() => close(false)}>Annulla</Button>
              <Button onClick={() => close(true)}>{request.confirmLabel ?? 'Conferma'}</Button>
            </div>
          </>
        )}
      </Dialog>
    </ConfirmContext>
  );
}

export function useConfirm(): ConfirmFn {
  const ctx = use(ConfirmContext);
  if (!ctx) throw new Error('useConfirm va usato dentro <ConfirmProvider>');
  return ctx;
}
