import { useEffect, useId, useRef, type ReactNode } from 'react';

interface Props {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
}

// Finestra modale basata su <dialog>: il browser gestisce focus, tasto Esc e sfondo.
// Il contenuto viene montato solo quando è aperta, così i form ripartono puliti ogni volta.
export function Dialog({ open, title, onClose, children }: Props) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      // Esc o dialog.close(): riallinea lo stato del componente padre.
      onClose={onClose}
      // Il click arriva su <dialog> solo fuori dal riquadro interno, cioè sullo sfondo.
      onClick={(e) => e.target === e.currentTarget && onClose()}
      className="m-auto w-full max-w-md rounded-lg border border-line bg-surface text-fg shadow-2xl shadow-black/50 backdrop:bg-black/60"
    >
      {open && (
        <div className="p-6">
          <h2 id={titleId} className="mb-4 text-lg font-bold">{title}</h2>
          {children}
        </div>
      )}
    </dialog>
  );
}
