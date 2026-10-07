import { useCallback, useState } from "react";
import { createPortal } from "react-dom";
import { CheckCircle2, Info, XCircle, X } from "lucide-react";
import { ToastContext, type ToastKind } from "./toastContext";

interface ToastItem {
  id: number;
  kind: ToastKind;
  message: string;
}

let nextId = 1;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const push = useCallback((kind: ToastKind, message: string) => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, kind, message }]);
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, kind === "info" ? 8000 : 5000);
  }, []);

  const dismiss = (id: number) => setToasts((prev) => prev.filter((t) => t.id !== id));

  return (
    <ToastContext.Provider value={{ push }}>
      {children}
      {createPortal(
        <div className="fixed bottom-4 right-4 z-[100] flex w-80 flex-col gap-2">
          {toasts.map((t) => (
            <div
              key={t.id}
              role="status"
              className="flex items-start gap-2.5 rounded-lg border border-edge-strong bg-panel px-3 py-2.5 text-sm text-ink shadow-xl animate-rise-in"
            >
              {t.kind === "success" ? (
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-ok" />
              ) : t.kind === "info" ? (
                <Info className="mt-0.5 h-4 w-4 shrink-0 text-info" />
              ) : (
                <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-bad" />
              )}
              <span className="flex-1">{t.message}</span>
              <button onClick={() => dismiss(t.id)} aria-label="Dismiss" className="shrink-0 opacity-60 hover:opacity-100">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  );
}

