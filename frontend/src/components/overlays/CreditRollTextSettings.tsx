'use client'

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

import { useId } from 'react'
import { Input } from '@/components/ui/input'
import { SliderControl } from '@/components/appearance/SliderControl'
import { AdvancedDisclosure } from '@/components/editor/AdvancedDisclosure'
import { type MessageKey, useTranslations } from '@/lib/i18n'
import {
  CREDIT_FONT_ELEMENTS,
  CREDIT_TEXT_GROUPS,
  type CreditFontElement,
  type CreditTextGroupId,
  type CreditTextKey,
  FONT_SCALE_DEFAULT,
  FONT_SCALE_MAX,
  FONT_SCALE_MIN,
  creditTextPlaceholders,
} from '@/lib/utils/creditRollCustomization'

const GROUP_LABELS: Record<CreditTextGroupId, MessageKey> = {
  header: 'overlayEditor.credits.textGroupHeader',
  categories: 'overlayEditor.credits.textGroupCategories',
  values: 'overlayEditor.credits.textGroupValues',
  durationWords: 'overlayEditor.credits.textGroupDurationWords',
  clips: 'overlayEditor.credits.textGroupClips',
  footer: 'overlayEditor.credits.textGroupFooter',
}

// Totals and duration words are many small fragments most streamers never touch;
// they sit behind a disclosure so the common texts stay in view.
const ADVANCED_GROUP: Record<CreditTextGroupId, boolean> = {
  header: false,
  categories: false,
  values: true,
  durationWords: true,
  clips: false,
  footer: false,
}

const SIZE_LABELS: Record<CreditFontElement, MessageKey> = {
  heading: 'overlayEditor.credits.sizeHeading',
  subheading: 'overlayEditor.credits.sizeSubheading',
  session: 'overlayEditor.credits.sizeSession',
  category: 'overlayEditor.credits.sizeCategory',
  entry: 'overlayEditor.credits.sizeEntry',
  footer: 'overlayEditor.credits.sizeFooter',
  nowPlaying: 'overlayEditor.credits.sizeNowPlaying',
}

function TextField({
  textKey,
  value,
  onChange,
}: {
  textKey: CreditTextKey
  value: string
  onChange: (value: string) => void
}) {
  const t = useTranslations()
  const id = useId()
  const placeholders = creditTextPlaceholders(textKey)
  // The default copy with its placeholders left visible, e.g. "Session: {date}".
  const defaultText = t(
    `viewerOverlay.credits.${textKey}`,
    Object.fromEntries(placeholders.map((name) => [name, `{${name}}`]))
  )
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-sm text-text-sub">
        {defaultText}
      </label>
      <Input
        id={id}
        value={value}
        placeholder={defaultText}
        maxLength={300}
        onChange={(e) => onChange(e.target.value)}
        aria-describedby={placeholders.length > 0 ? `${id}-placeholders` : undefined}
      />
      {placeholders.length > 0 && (
        <p id={`${id}-placeholders`} className="text-dim mt-1 text-xs">
          {t('overlayEditor.credits.textPlaceholders', {
            list: placeholders.map((name) => `{${name}}`).join(' '),
          })}
        </p>
      )}
    </div>
  )
}

export function CreditRollTextSettings({
  texts,
  onTextsChange,
}: {
  texts: Record<string, string>
  onTextsChange: (texts: Record<string, string>) => void
}) {
  const t = useTranslations()
  const setText = (key: CreditTextKey, value: string) => {
    const next = { ...texts }
    if (value.trim()) next[key] = value
    else delete next[key]
    onTextsChange(next)
  }

  const renderGroup = (group: (typeof CREDIT_TEXT_GROUPS)[number]) => (
    <fieldset key={group.id} className="space-y-3">
      <legend className="mb-2 text-sm font-medium">{t(GROUP_LABELS[group.id])}</legend>
      {group.keys.map((key) => (
        <TextField
          key={key}
          textKey={key}
          value={texts[key] ?? ''}
          onChange={(value) => setText(key, value)}
        />
      ))}
    </fieldset>
  )

  const common = CREDIT_TEXT_GROUPS.filter((group) => !ADVANCED_GROUP[group.id])
  const advanced = CREDIT_TEXT_GROUPS.filter((group) => ADVANCED_GROUP[group.id])

  return (
    <div className="lanes-panel mb-6 p-6">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <h3 className="text-lg">{t('overlayEditor.credits.textsHeading')}</h3>
          <p className="text-sub mt-1 text-sm">
            {t('overlayEditor.credits.textsHint', { date: '{date}' })}
          </p>
        </div>
        <button
          type="button"
          className="lanes-btn ghost shrink-0"
          disabled={Object.keys(texts).length === 0}
          onClick={() => onTextsChange({})}
        >
          {t('overlayEditor.credits.textsReset')}
        </button>
      </div>
      <div className="space-y-6">{common.map(renderGroup)}</div>
      <AdvancedDisclosure count={advanced.reduce((n, group) => n + group.keys.length, 0)}>
        {advanced.map(renderGroup)}
      </AdvancedDisclosure>
    </div>
  )
}

export function CreditRollSizeSettings({
  fontScales,
  onFontScalesChange,
}: {
  fontScales: Record<string, number>
  onFontScalesChange: (fontScales: Record<string, number>) => void
}) {
  const t = useTranslations()
  const setScale = (element: CreditFontElement, pct: number) => {
    const next = { ...fontScales }
    if (pct === FONT_SCALE_DEFAULT) delete next[element]
    else next[element] = pct
    onFontScalesChange(next)
  }

  return (
    <div className="lanes-panel mb-6 p-6">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <h3 className="text-lg">{t('overlayEditor.credits.sizesHeading')}</h3>
          <p className="text-sub mt-1 text-sm">{t('overlayEditor.credits.sizesHint')}</p>
        </div>
        <button
          type="button"
          className="lanes-btn ghost shrink-0"
          disabled={Object.keys(fontScales).length === 0}
          onClick={() => onFontScalesChange({})}
        >
          {t('overlayEditor.credits.sizesReset')}
        </button>
      </div>
      <div className="space-y-3">
        {CREDIT_FONT_ELEMENTS.map(({ id }) => (
          <SliderControl
            key={id}
            label={t(SIZE_LABELS[id])}
            value={fontScales[id] ?? FONT_SCALE_DEFAULT}
            min={FONT_SCALE_MIN}
            max={FONT_SCALE_MAX}
            step={10}
            unit="%"
            onChange={(pct) => setScale(id, pct)}
          />
        ))}
      </div>
    </div>
  )
}
