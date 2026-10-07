interface SegmentedOption<T extends string> {
  value: T;
  label: string;
  count?: number;
}

export function Segmented<T extends string>({
  value,
  onChange,
  options,
}: {
  value: T;
  onChange: (value: T) => void;
  options: SegmentedOption<T>[];
}) {
  return (
    <div className="inline-flex h-8 items-center rounded-md border border-edge bg-panel p-0.5" role="radiogroup">
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            role="radio"
            aria-checked={active}
            onClick={() => onChange(opt.value)}
            className={`flex h-full items-center gap-1.5 rounded px-2.5 text-xs font-medium transition ${
              active ? "bg-panel-2 text-ink shadow-[inset_0_0_0_1px_rgb(var(--color-edge-strong))]" : "text-ink-muted hover:text-ink"
            }`}
          >
            {opt.label}
            {opt.count !== undefined && (
              <span className={`font-mono text-[10.5px] tabular-nums ${active ? "text-accent" : "text-ink-faint"}`}>{opt.count}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
