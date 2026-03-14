import Link from 'next/link';

interface LegalLayoutProps {
  title: string;
  lastUpdated: string;
  children: React.ReactNode;
}

export default function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps) {
  return (
    <div className="min-h-screen bg-linear-to-b from-slate-50 to-slate-100 px-4 py-12 text-slate-700">
      <div className="mx-auto max-w-4xl rounded-2xl border border-slate-100 bg-white p-8 shadow-xl md:p-12">
        <div className="mb-8 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-500">
            All-Chat Legal
          </p>
          <h1 className="text-4xl font-bold text-slate-900">{title}</h1>
          <p className="text-sm text-slate-500">Last updated: {lastUpdated}</p>
        </div>

        <div className="space-y-10 leading-relaxed text-slate-700">{children}</div>

        <div className="mt-12 flex flex-col gap-3 border-t border-slate-200 pt-6 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between">
          <span>© {new Date().getFullYear()} All-Chat</span>
          <div className="flex flex-wrap items-center gap-4">
            <Link href="/" className="hover:text-slate-900">
              Home
            </Link>
            <Link href="/legal/privacy" className="hover:text-slate-900">
              Privacy Policy
            </Link>
            <Link href="/legal/terms" className="hover:text-slate-900">
              Terms of Service
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
