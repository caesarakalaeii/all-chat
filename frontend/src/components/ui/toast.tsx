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


import { Toast as ToastPrimitive } from '@base-ui/react/toast'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toastManager } from '@/lib/toast'

function ToastList() {
  const { toasts } = ToastPrimitive.useToastManager()
  return (
    <>
      {toasts.map((toast) => (
        <ToastPrimitive.Root
          key={toast.id}
          toast={toast}
          className={cn(
            'min-w-[280px] rounded-xl border border-border bg-surface-2 px-4 py-3 shadow-xl',
            'animate-fade-in border-l-4',
            toast.type === 'success' && 'border-l-kick',
            toast.type === 'error' && 'border-l-youtube',
            toast.type === 'info' && 'border-l-tiktok',
            (!toast.type || toast.type === 'warning') && 'border-l-border'
          )}
        >
          <div className="flex items-start justify-between gap-2">
            <div>
              {toast.title && (
                <ToastPrimitive.Title className="text-sm font-medium text-text">
                  {toast.title}
                </ToastPrimitive.Title>
              )}
              {toast.description && (
                <ToastPrimitive.Description className="mt-0.5 text-xs text-text-sub">
                  {toast.description}
                </ToastPrimitive.Description>
              )}
            </div>
            <ToastPrimitive.Close
              className="shrink-0 text-text-sub transition-colors hover:text-text"
              aria-label="Close notification"
            >
              <X className="size-4" />
            </ToastPrimitive.Close>
          </div>
        </ToastPrimitive.Root>
      ))}
    </>
  )
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  return (
    <ToastPrimitive.Provider toastManager={toastManager} timeout={4000}>
      {children}
      <ToastPrimitive.Viewport className="fixed right-4 bottom-4 z-50 flex w-auto max-w-sm flex-col-reverse gap-2">
        <ToastList />
      </ToastPrimitive.Viewport>
    </ToastPrimitive.Provider>
  )
}
