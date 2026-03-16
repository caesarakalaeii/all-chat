export function PremiumBadge({ size = 18, title }: { size?: number; title?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 18 18"
      fill="none"
      aria-label="Premium badge"
      className="inline-block shrink-0"
    >
      {title && <title>{title}</title>}
      <polygon points="5,2 13,2 16,7 9,16 2,7" fill="#a855f7" stroke="#7c3aed" strokeWidth="1" />
      <line x1="2" y1="7" x2="16" y2="7" stroke="#7c3aed" strokeWidth="0.8" />
      <line x1="5" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
      <line x1="13" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
    </svg>
  )
}
