'use client'

import { Input as InputPrimitive } from '@base-ui/react/input'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '@/lib/utils'

const inputVariants = cva(
  'w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-dim transition-all outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      size: {
        default: 'h-9',
        sm: 'h-7 text-xs',
      },
    },
    defaultVariants: {
      size: 'default',
    },
  }
)

type InputProps = Omit<InputPrimitive.Props, 'size'> & VariantProps<typeof inputVariants>

function Input({ className, size, ...props }: InputProps) {
  return (
    <InputPrimitive
      data-slot="input"
      className={cn(inputVariants({ size, className }))}
      {...props}
    />
  )
}

export { Input, inputVariants }
