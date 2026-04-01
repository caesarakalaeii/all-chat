import Link from 'next/link'
import { AppNav } from '@/components/AppNav'

interface LegalLayoutProps {
  title: string
  lastUpdated: string
  children: React.ReactNode
}

export default function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps) {
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              All-Chat Legal
            </p>
            <h1 className="text-3xl font-bold text-text">{title}</h1>
            <p className="text-sm text-text-dim">Last updated: {lastUpdated}</p>
          </div>

          <div className="space-y-10 leading-relaxed text-text-sub">{children}</div>

          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>&copy; {new Date().getFullYear()} All-Chat</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/" className="transition-colors hover:text-text">
                Home
              </Link>
              <Link href="/legal/privacy" className="transition-colors hover:text-text">
                Privacy Policy
              </Link>
              <Link href="/legal/terms" className="transition-colors hover:text-text">
                Terms of Service
              </Link>
              <Link href="/legal/impressum" className="transition-colors hover:text-text">
                Impressum
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
