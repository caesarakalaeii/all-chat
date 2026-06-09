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

import { readFileSync } from 'fs'
import LegalLayout from '@/components/legal/LegalLayout'

export const dynamic = 'force-dynamic'

export const metadata = {
  title: 'Impressum | All-Chat',
  description: 'Legal notice (Impressum) as required by TMG 5.',
  alternates: { canonical: '/legal/impressum' },
}

const IMPRESSUM_PATH = process.env.IMPRESSUM_FILE_PATH || '/etc/allchat/impressum.html'

function loadImpressum(): string | null {
  try {
    return readFileSync(IMPRESSUM_PATH, 'utf-8')
  } catch {
    return null
  }
}

export default function ImpressumPage() {
  const html = loadImpressum()

  return (
    <LegalLayout title="Impressum" lastUpdated="">
      {html ? (
        <div dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <section className="space-y-4">
          <p className="text-text-sub">
            The Impressum for this instance has not been configured yet.
          </p>
          <p className="text-sm text-text-dim">
            If you are the operator: mount a ConfigMap containing your Impressum HTML to{' '}
            <code className="rounded bg-surface-2 px-1.5 py-0.5 text-xs">{IMPRESSUM_PATH}</code>{' '}
            or set the <code className="rounded bg-surface-2 px-1.5 py-0.5 text-xs">IMPRESSUM_FILE_PATH</code>{' '}
            environment variable. See the deployment documentation for details.
          </p>
        </section>
      )}
    </LegalLayout>
  )
}
