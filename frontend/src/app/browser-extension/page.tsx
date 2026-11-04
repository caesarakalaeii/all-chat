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

import type { Metadata } from 'next'
import { BrowserExtensionClient } from './BrowserExtensionClient'
import { getTranslations } from '@/lib/i18n'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

// openGraph and twitter replace the layout's objects wholesale, so the
// site-level fields are repeated; the card comes from ./opengraph-image.
export const metadata = {
  title: t('marketing.extensionPage.metaTitle'),
  description: t('marketing.extensionPage.metaDescription'),
  alternates: { canonical: '/browser-extension' },
  openGraph: {
    type: 'website',
    locale: 'en_US',
    url: '/browser-extension',
    siteName: 'All-Chat',
    title: t('marketing.extensionPage.socialTitle'),
    description: t('marketing.extensionPage.metaDescription'),
  },
  twitter: {
    card: 'summary_large_image',
    title: t('marketing.extensionPage.socialTitle'),
    description: t('marketing.extensionPage.metaDescription'),
  },
} satisfies Metadata

export default function BrowserExtensionPage() {
  return <BrowserExtensionClient />
}
