'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';

export default function ImpersonationBanner() {
  const router = useRouter();
  const [isImpersonating, setIsImpersonating] = useState(false);
  const [impersonatedUser, setImpersonatedUser] = useState<string>('');

  useEffect(() => {
    const checkImpersonation = () => {
      const impersonating = localStorage.getItem('impersonating') === 'true';
      const user = localStorage.getItem('impersonated_user') || '';
      setIsImpersonating(impersonating);
      setImpersonatedUser(user);
    };

    checkImpersonation();

    // Listen for storage changes (in case impersonation starts/stops in another tab)
    window.addEventListener('storage', checkImpersonation);
    return () => window.removeEventListener('storage', checkImpersonation);
  }, []);

  const handleExitImpersonation = () => {
    // Restore the original admin token
    const adminToken = localStorage.getItem('admin_token');
    if (adminToken) {
      localStorage.setItem('jwt_token', adminToken);
      localStorage.removeItem('admin_token');
    }

    // Clear impersonation flags
    localStorage.removeItem('impersonating');
    localStorage.removeItem('impersonated_user');

    // Redirect to admin panel
    router.push('/admin/users');
    router.refresh();
  };

  if (!isImpersonating) {
    return null;
  }

  return (
    <div className="bg-orange-600 text-white px-4 py-2 flex items-center justify-between shadow-md">
      <div className="flex items-center gap-3">
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <div>
          <span className="font-semibold">Admin Mode:</span> Viewing as <span className="font-mono">{impersonatedUser}</span>
        </div>
      </div>
      <button
        onClick={handleExitImpersonation}
        className="px-4 py-1 bg-white text-orange-600 hover:bg-orange-50 font-medium rounded transition-colors"
      >
        Exit & Return to Admin
      </button>
    </div>
  );
}
