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
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// Module scope for the metadata block, so getTranslations rather than the hook.
const t = getTranslations()

export const dynamic = 'force-dynamic'

export const metadata = {
  title: t('metadata.impressum.title'),
  description: t('metadata.impressum.description'),
  alternates: { canonical: '/legal/impressum' },
}

const IMPRESSUM_PATH = process.env.IMPRESSUM_FILE_PATH || '/etc/allchat/impressum.html'

// The environment variable's own name, quoted back to the operator. An
// identifier, not copy.
const IMPRESSUM_ENV_VAR = 'IMPRESSUM_FILE_PATH'

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
    <LegalLayout title={t('legal.impressum.title')} lastUpdated="">
      {html ? (
        <div dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <section className="space-y-4">
          <p className="text-text-sub">{t('legal.impressum.notConfigured')}</p>
          <p className="text-sm text-text-dim">
            {interpolateElements(t('legal.impressum.operatorHint'), {
              path: (
                <code className="rounded bg-surface-2 px-1.5 py-0.5 text-xs">{IMPRESSUM_PATH}</code>
              ),
              variable: (
                <code className="rounded bg-surface-2 px-1.5 py-0.5 text-xs">
                  {IMPRESSUM_ENV_VAR}
                </code>
              ),
            })}
          </p>
        </section>
      )}
    </LegalLayout>
  )
}
