'use client'

/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslations } from '@/lib/i18n'
import type { PagedList } from '@/hooks/usePagedList'

/**
 * Endless-scroll footer for a usePagedList list: loads the next page when it
 * scrolls into view. It is also a real button, so keyboard and screen-reader
 * users (and browsers without IntersectionObserver) can load more without
 * scrolling. A failed page is not retried automatically; the button retries.
 * Renders nothing once every row is loaded.
 */
export function LoadMore({
  list,
  as: Tag = 'div',
}: {
  list: PagedList<unknown>
  as?: 'div' | 'li'
}) {
  const t = useTranslations()
  const [sentinel, setSentinel] = useState<HTMLElement | null>(null)
  const { hasMore, loadingMore, error, loadMore } = list

  // Re-subscribes after every page (loadMore changes with the row count), and
  // a fresh observer reports the current intersection at once: a sentinel
  // still on screen after a short page keeps loading until the view is full.
  useEffect(() => {
    if (!sentinel || !hasMore || loadingMore || error) return
    if (typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) loadMore()
    })
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [sentinel, hasMore, loadingMore, error, loadMore])

  if (!hasMore) return null
  return (
    <Tag ref={setSentinel} className="flex justify-center px-4 py-3">
      <Button type="button" variant="ghost" size="sm" onClick={loadMore} disabled={loadingMore}>
        {loadingMore
          ? t('admin.list.loadingMore')
          : error
            ? t('admin.list.loadMoreError')
            : t('admin.list.loadMore')}
      </Button>
    </Tag>
  )
}
