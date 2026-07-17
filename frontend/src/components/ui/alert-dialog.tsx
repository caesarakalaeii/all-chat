'use client'

/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Accessible alert dialog (Base UI AlertDialog) for destructive/irreversible
 * confirmations: ban, delete, revoke, disconnect. Unlike ui/dialog it renders
 * role="alertdialog", is NOT dismissed by clicking outside, and has no corner
 * close button — the user must make an explicit choice. Put the cancel/least-
 * destructive action first in DOM order so it receives initial focus.
 */

import { AlertDialog as AlertDialogPrimitive } from '@base-ui/react/alert-dialog'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const alertDialogContentVariants = cva(
  'fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 w-full rounded-xl border border-border bg-surface p-6 shadow-xl text-text',
  {
    variants: {
      size: {
        sm: 'max-w-sm',
        default: 'max-w-md',
        lg: 'max-w-lg',
      },
    },
    defaultVariants: { size: 'default' },
  }
)

function AlertDialogTitle({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Title>) {
  return (
    <AlertDialogPrimitive.Title
      className={cn('text-lg font-semibold text-text', className)}
      {...props}
    />
  )
}

function AlertDialogDescription({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Description>) {
  return (
    <AlertDialogPrimitive.Description
      className={cn('mt-2 text-sm text-text-sub', className)}
      {...props}
    />
  )
}

function AlertDialogContent({
  className,
  size,
  children,
  ...props
}: React.ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Popup> &
  VariantProps<typeof alertDialogContentVariants>) {
  return (
    <AlertDialogPrimitive.Portal>
      <AlertDialogPrimitive.Backdrop className="fixed inset-0 z-40 bg-black/60 backdrop-blur-[8px]" />
      <AlertDialogPrimitive.Popup
        data-slot="alert-dialog-content"
        className={cn(alertDialogContentVariants({ size, className }))}
        {...props}
      >
        {children}
      </AlertDialogPrimitive.Popup>
    </AlertDialogPrimitive.Portal>
  )
}

export const AlertDialog = {
  Root: AlertDialogPrimitive.Root,
  Trigger: AlertDialogPrimitive.Trigger,
  Close: AlertDialogPrimitive.Close,
  Title: AlertDialogTitle,
  Description: AlertDialogDescription,
  Content: AlertDialogContent,
}

export { alertDialogContentVariants }
