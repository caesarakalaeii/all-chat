import Link from 'next/link';

interface LegalLayoutProps {
  title: string;
  lastUpdated: string;
  children: React.ReactNode;
}

export default function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps) {
  return (
    <div className="min-h-screen bg-gradient-to-b from-gray-50 to-gray-100 py-12 px-4 text-gray-700">
      <div className="max-w-4xl mx-auto bg-white shadow-xl rounded-2xl border border-gray-100 p-8 md:p-12">
        <div className="mb-8 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-500">All-Chat Legal</p>
          <h1 className="text-4xl font-bold text-gray-900">{title}</h1>
          <p className="text-sm text-gray-500">Last updated: {lastUpdated}</p>
        </div>

        <div className="space-y-10 leading-relaxed text-gray-700">{children}</div>

        <div className="mt-12 pt-6 border-t border-gray-200 flex flex-col gap-3 text-sm text-gray-500 sm:flex-row sm:items-center sm:justify-between">
          <span>© {new Date().getFullYear()} All-Chat</span>
          <div className="flex flex-wrap items-center gap-4">
            <Link href="/" className="hover:text-gray-900">
              Home
            </Link>
            <Link href="/legal/privacy" className="hover:text-gray-900">
              Privacy Policy
            </Link>
            <Link href="/legal/terms" className="hover:text-gray-900">
              Terms of Service
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
