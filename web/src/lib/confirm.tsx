import { createContext, use, useCallback, useState, type ReactNode } from 'react';
import { Button, cx, Dialog, fieldClass } from '../components/ui';

interface ConfirmOptions {
  title: string;
  message: ReactNode;
  confirmLabel?: string;
  // Per le operazioni da non fare per sbaglio: si conferma solo dopo aver scritto questa parola.
  typeToConfirm?: string;
}

// Mostra il dialog di conferma; la Promise si risolve con true se l'utente conferma.
type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<(ConfirmOptions & { resolve: (ok: boolean) => void }) | null>(null);
  const [typed, setTyped] = useState('');

  const confirm = useCallback<ConfirmFn>(
    (options) => new Promise((resolve) => {
      setTyped('');
      setRequest({ ...options, resolve });
    }),
    [],
  );

  function close(ok: boolean) {
    request?.resolve(ok);
    setRequest(null);
  }

  const word = request?.typeToConfirm;
  const allowed = !word || typed.trim().toUpperCase() === word.toUpperCase();
  return (
    <ConfirmContext value={confirm}>
      {children}
      <Dialog open={request !== null} title={request?.title ?? ''} onClose={() => close(false)}>
        {request && (
          <>
            <p className="text-muted">{request.message}</p>
            {word && (
              <label className="mt-4 flex flex-col gap-1.5 text-sm">
                <span>Per confermare scrivi <strong>{word}</strong></span>
                <input
                  value={typed}
                  onChange={(e) => setTyped(e.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                  className={cx(fieldClass, 'w-48')}
                />
              </label>
            )}
            {/* Annulla viene prima: è lui a ricevere il focus, così Invio non elimina per sbaglio. */}
            <div className="mt-6 flex justify-end gap-2">
              <Button variant="ghost" onClick={() => close(false)}>Annulla</Button>
              <Button disabled={!allowed} onClick={() => close(true)}>{request.confirmLabel ?? 'Conferma'}</Button>
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
