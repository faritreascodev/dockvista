import { AlertTriangle } from "lucide-react";

export function ErrorBanner({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div
      role="alert"
      className="flex items-center justify-between gap-3 rounded-lg border border-bad/30 bg-bad/10 px-4 py-3 text-sm text-bad"
    >
      <span className="flex items-center gap-2">
        <AlertTriangle className="h-4 w-4 shrink-0" />
        {message}
      </span>
      {onRetry && (
        <button
          onClick={onRetry}
          className="rounded-md border border-bad/40 px-2.5 py-1 text-xs font-medium transition hover:bg-bad/10"
        >
          Retry
        </button>
      )}
    </div>
  );
}
