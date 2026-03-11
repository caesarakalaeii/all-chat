"use client"

import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const cardVariants = cva(
  "rounded-xl border border-border bg-surface text-text transition-all",
  {
    variants: {
      interactive: {
        true: "hover:scale-[1.02] hover:shadow-lg cursor-pointer",
        false: "",
      },
    },
    defaultVariants: {
      interactive: false,
    },
  }
)

function Card({
  className,
  interactive,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof cardVariants>) {
  return (
    <div
      data-slot="card"
      className={cn(cardVariants({ interactive, className }))}
      {...props}
    />
  )
}

export { Card, cardVariants }
