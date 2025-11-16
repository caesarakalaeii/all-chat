'use client';

import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { authApi } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { useAuthStore } from '@/lib/stores/auth-store';

export default function SettingsPage() {
  const router = useRouter();
  const { user, token, loading } = useAuthStore((state) => ({
    user: state.user,
    token: state.token,
    loading: state.loading
  }));
  const init = useAuthStore((state) => state.init);
  const logout = useAuthStore((state) => state.logout);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    init();
  }, [init]);

  useEffect(() => {
    if (!loading && !token) {
      router.replace('/');
    }
  }, [loading, token, router]);

  const handleLogout = () => {
    logout();
    router.push('/');
  };

  const handleDeleteAccount = async () => {
    if (
      !window.confirm(
        'Deleting your account removes all overlays, tokens, and chat sources. This cannot be undone. Continue?'
      )
    ) {
      return;
    }

    setDeleteError(null);
    setDeleteLoading(true);

    try {
      await authApi.deleteAccount();
      logout();
      router.replace('/');
    } catch (error) {
      if (error instanceof ApiError) {
        setDeleteError(error.message);
      } else {
        setDeleteError('Failed to delete account. Please try again.');
      }
    } finally {
      setDeleteLoading(false);
    }
  };

  if (loading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="container mx-auto px-4 py-4 flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-8">
            <Link href="/dashboard" className="text-2xl font-bold">
              All-Chat
            </Link>
            <div className="flex items-center gap-4 text-sm">
              <Link href="/dashboard" className="text-gray-400 hover:text-white transition-colors">
                Dashboard
              </Link>
              <span className="text-white font-semibold">Settings</span>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-3">
              {user.profile_image_url && (
                <Image
                  src={user.profile_image_url}
                  alt={user.display_name}
                  width={32}
                  height={32}
                  className="w-8 h-8 rounded-full object-cover"
                />
              )}
              <span>{user.display_name}</span>
            </div>
            <button onClick={handleLogout} className="text-gray-400 hover:text-white transition-colors">
              Logout
            </button>
          </div>
        </div>
      </nav>

      <main className="container mx-auto px-4 py-10 space-y-8">
        <header>
          <p className="text-sm uppercase tracking-[0.3em] text-gray-500">Settings</p>
          <h1 className="text-3xl font-bold mt-2">Manage your All-Chat account</h1>
          <p className="text-gray-400 mt-2">
            Update account preferences, review legal policies, or permanently delete your data.
          </p>
        </header>

        <section className="grid gap-6 md:grid-cols-2">
          <div className="rounded-2xl border border-gray-800 bg-gray-800/70 p-6 backdrop-blur">
            <h2 className="text-xl font-semibold mb-4">Account Overview</h2>
            <dl className="space-y-3 text-gray-300">
              <div>
                <dt className="text-sm uppercase tracking-wide text-gray-500">Display Name</dt>
                <dd className="text-lg text-white">{user.display_name}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wide text-gray-500">Username</dt>
                <dd className="text-lg text-white">{user.username}</dd>
              </div>
              <div>
                <dt className="text-sm uppercase tracking-wide text-gray-500">Primary Platform</dt>
                <dd className="text-lg capitalize text-white">
                  {user.auth_provider ? user.auth_provider : 'Unknown'}
                </dd>
              </div>
            </dl>
          </div>

          <div className="rounded-2xl border border-gray-800 bg-gray-800/70 p-6 backdrop-blur">
            <h2 className="text-xl font-semibold mb-4">Data & Privacy</h2>
            <p className="text-gray-300 mb-4">
              We keep data collection minimal and transparent. Review the policies below for details about
              how tokens, overlays, and chat metadata are processed.
            </p>
            <div className="flex flex-col gap-3">
              <Link
                href="/legal/privacy"
                className="inline-flex items-center justify-between rounded-lg border border-gray-700 px-4 py-3 text-sm text-gray-200 hover:bg-gray-800 transition-colors"
              >
                <span>Privacy Policy</span>
                <span aria-hidden="true">→</span>
              </Link>
              <Link
                href="/legal/terms"
                className="inline-flex items-center justify-between rounded-lg border border-gray-700 px-4 py-3 text-sm text-gray-200 hover:bg-gray-800 transition-colors"
              >
                <span>Terms of Service</span>
                <span aria-hidden="true">→</span>
              </Link>
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-red-500/30 bg-red-500/10 p-6">
          <h2 className="text-2xl font-semibold text-red-200">Danger Zone</h2>
          <p className="text-red-100/80 mt-3">
            Deleting your account removes all overlays, OAuth grants, and cached chat sources. This action is
            permanent.
          </p>
          {deleteError && (
            <div className="mt-4 rounded-lg border border-red-400 bg-red-500/20 px-4 py-3 text-sm text-red-100">
              {deleteError}
            </div>
          )}
          <button
            onClick={handleDeleteAccount}
            disabled={deleteLoading}
            className="mt-6 inline-flex w-full items-center justify-center rounded-lg border border-red-400 bg-red-600/80 px-4 py-3 font-semibold text-white shadow-lg shadow-red-900/20 transition hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-60 md:w-auto"
          >
            {deleteLoading ? 'Deleting...' : 'Delete my account'}
          </button>
        </section>
      </main>
    </div>
  );
}
