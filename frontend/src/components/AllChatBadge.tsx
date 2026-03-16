'use client'

import { InfinityLogo } from '@/components/InfinityLogo'

export function AllChatBadge({ size = 18, title }: { size?: number; title?: string }) {
  return (
    <span title={title} aria-label="All-Chat badge" className="inline-flex items-center shrink-0">
      <InfinityLogo size={size} />
    </span>
  )
}
