/**
 * The DockVista mark: an isometric container with a lens on its side,
 * redrawn from assets/logos so it follows the theme and accent.
 */
export function Logo({ className = "h-7 w-7" }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" fill="none" className={`text-ink ${className}`} aria-hidden="true">
      <g stroke="currentColor" strokeWidth="4.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M32 5 55.4 18.5v27L32 59 8.6 45.5v-27Z" />
        <path d="M8.6 18.5 32 32l23.4-13.5M32 32v27" />
      </g>
      <g className="text-accent" stroke="currentColor" strokeLinecap="round">
        <circle cx="44" cy="37" r="7.2" strokeWidth="3.8" />
        <path d="m49.3 42.3 3.2 3.2" strokeWidth="4.5" />
      </g>
      <circle cx="44" cy="37" r="3.2" className="fill-accent" />
      <circle cx="42.7" cy="35.7" r="1.15" className="fill-canvas" />
    </svg>
  );
}
