import { Inbox } from "lucide-react";

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-panel-2 text-ink-faint">
        <Inbox className="h-6 w-6" />
      </div>
      <p className="text-sm text-ink-muted">{message}</p>
    </div>
  );
}
