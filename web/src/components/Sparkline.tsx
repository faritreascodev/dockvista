import { useId } from "react";

interface SparklineProps {
  values: number[];
  /** Fixed top of the scale (e.g. 100 for percentages); defaults to the series max. */
  max?: number | undefined;
  className?: string;
  /** Tailwind text-* class; the line and fill use currentColor. */
  tone?: string;
  filled?: boolean;
}

const W = 100;
const H = 32;

/**
 * Dependency-free SVG sparkline. It stretches to its container
 * (preserveAspectRatio="none") and keeps a constant stroke width, so the
 * same component works in a table cell and in a wide chart.
 */
export function Sparkline({ values, max, className = "h-8 w-full", tone = "text-accent", filled = true }: SparklineProps) {
  const gradientId = useId();

  if (values.length < 2) {
    return <div className={`${className} border-b border-dashed border-edge`} />;
  }

  const top = Math.max(max ?? Math.max(...values), 1e-9);
  const step = W / (values.length - 1);
  const points = values.map((v, i) => {
    const y = H - (Math.min(Math.max(v, 0), top) / top) * (H - 2) - 1;
    return `${(i * step).toFixed(2)},${y.toFixed(2)}`;
  });
  const line = `M${points.join(" L")}`;
  const area = `${line} L${W},${H} L0,${H} Z`;

  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className={`${className} ${tone}`} aria-hidden="true">
      {filled && (
        <>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="currentColor" stopOpacity="0.28" />
              <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={area} fill={`url(#${gradientId})`} />
        </>
      )}
      <path
        d={line}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
