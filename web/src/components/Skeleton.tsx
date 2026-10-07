export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="divide-y divide-edge">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-4 py-3.5" style={{ opacity: 1 - i * 0.15 }}>
          <div className="h-2.5 w-16 animate-pulse rounded-sm bg-panel-2" />
          <div className="h-2.5 w-36 animate-pulse rounded-sm bg-panel-2" />
          <div className="h-2.5 w-24 animate-pulse rounded-sm bg-panel-2" />
          <div className="ml-auto h-2.5 w-14 animate-pulse rounded-sm bg-panel-2" />
        </div>
      ))}
    </div>
  );
}
