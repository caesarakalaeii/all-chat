"use client"

import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"
import { PLATFORM_COLORS, type Platform } from "@/lib/platform-colors"

const badgeVariants = cva(
  "inline-flex items-center rounded-full bg-badge-bg font-mono text-text whitespace-nowrap",
  {
    variants: {
      size: {
        default: "px-2.5 py-0.5 text-xs",
        sm:      "px-2 py-0.5 text-[0.65rem]",
      },
    },
    defaultVariants: { size: "default" },
  }
)

// Generic badge (for non-platform use)
function Badge({
  className,
  size,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ size, className }))}
      {...props}
    />
  )
}

// Platform-coded badge with glow dot
function PlatformBadge({
  platform,
  size,
  className,
}: {
  platform: Platform
  size?: "default" | "sm"
  className?: string
}) {
  const isSystem = platform === "system"
  const glowStyle = isSystem
    ? { backgroundColor: "var(--color-text-dim)", boxShadow: "none" }
    : {
        backgroundColor: `var(--color-${platform})`,
        boxShadow: `var(--shadow-glow-${platform})`,
      }
  const textClass = PLATFORM_COLORS[platform].text

  return (
    <span
      data-slot="platform-badge"
      data-platform={platform}
      className={cn(badgeVariants({ size }), textClass, className)}
    >
      <span
        className="inline-block w-1.5 h-1.5 rounded-full mr-1.5 shrink-0"
        style={glowStyle}
        aria-hidden="true"
      />
      {platform.toUpperCase()}
    </span>
  )
}

export { Badge, PlatformBadge, badgeVariants }
