"use client"

import { Toast as ToastPrimitive } from "@base-ui/react/toast"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"
import { toastManager } from "@/lib/toast"

function ToastList() {
  const { toasts } = ToastPrimitive.useToastManager()
  return (
    <>
      {toasts.map((toast) => (
        <ToastPrimitive.Root
          key={toast.id}
          toast={toast}
          className={cn(
            "bg-surface-2 border border-border rounded-xl px-4 py-3 shadow-xl min-w-[280px]",
            "border-l-4 animate-fade-in",
            toast.type === "success" && "border-l-kick",
            toast.type === "error" && "border-l-youtube",
            toast.type === "info" && "border-l-tiktok",
            (!toast.type || toast.type === "warning") && "border-l-border",
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
                <ToastPrimitive.Description className="text-xs text-text-sub mt-0.5">
                  {toast.description}
                </ToastPrimitive.Description>
              )}
            </div>
            <ToastPrimitive.Close
              className="shrink-0 text-text-sub hover:text-text transition-colors"
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
      <ToastPrimitive.Viewport
        className="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 w-auto max-w-sm"
      >
        <ToastList />
      </ToastPrimitive.Viewport>
    </ToastPrimitive.Provider>
  )
}
