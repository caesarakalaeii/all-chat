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

'use client'

/**
 * The editors of goal and list overlays (issue #955, ADR-0072). Creating the
 * overlay is the premium step (goal_overlay / list_overlay gates); configuring
 * one is free. A save is pushed to the live overlay at once.
 */

import { useEffect, useId, useState } from 'react'

import { ToggleSwitch } from '@/components/appearance/ToggleSwitch'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ApiError } from '@/lib/api/client'
import { overlaysApi } from '@/lib/api/overlays'
import { formatNumber, useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import {
  GOAL_METRICS,
  GOAL_METRIC_PLATFORM,
  LIST_CATEGORIES,
  MAX_LIST_LIMIT,
  type GoalConfig,
  type GoalMetric,
  type GoalState,
  type ListCategory,
  type ListConfig,
} from '@/lib/types/goal'

function SelectField<T extends string>({
  id,
  label,
  value,
  items,
  onChange,
}: {
  id: string
  label: string
  value: T
  items: { value: T; label: string }[]
  onChange: (value: T) => void
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs text-text-sub">
        {label}
      </label>
      <Select
        items={items}
        value={value}
        onValueChange={(next) => {
          if (next !== null) onChange(next as T)
        }}
      >
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

function TextField({
  id,
  label,
  hint,
  ...input
}: { id: string; label: string; hint?: string } & React.ComponentProps<typeof Input>) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs text-text-sub">
        {label}
      </label>
      <Input id={id} aria-describedby={hint ? `${id}-hint` : undefined} {...input} />
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-text-sub">
          {hint}
        </p>
      )}
    </div>
  )
}

function saveError(err: unknown, fallback: string): string {
  return err instanceof ApiError && err.message ? err.message : fallback
}

export function GoalEditor({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
  const ids = useId()
  const [state, setState] = useState<GoalState | null>(null)
  const [goal, setGoal] = useState<GoalConfig | null>(null)
  const [reset, setReset] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    overlaysApi
      .getGoal(overlayId)
      .then((loaded) => {
        setState(loaded)
        setGoal(loaded.goal)
      })
      .catch(() =>
        toastManager.add({ title: t('overlayEditor.goalEditor.loadFailed'), type: 'error' })
      )
  }, [overlayId, t])

  if (!goal || !state) return null
  const polled = goal.metric === 'followers' || goal.metric === 'subscribers'
  const update = (patch: Partial<GoalConfig>) => setGoal({ ...goal, ...patch })

  async function save() {
    if (!goal) return
    setSaving(true)
    try {
      const saved = await overlaysApi.saveGoal(overlayId, goal, reset && !polled)
      setState(saved)
      setReset(false)
      toastManager.add({ title: t('overlayEditor.goalEditor.saved'), type: 'success' })
    } catch (err) {
      toastManager.add({
        title: saveError(err, t('overlayEditor.goalEditor.saveFailed')),
        type: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="space-y-5">
      <div>
        <p className="text-xs text-text-sub">
          {t('overlayEditor.goalEditor.current', {
            current: formatNumber(state.current),
            target: formatNumber(state.goal.target),
          })}
        </p>
      </div>
      {state.status && (
        <p
          role="status"
          className="rounded-md border border-warning/40 bg-warning/10 p-3 text-sm text-text"
        >
          {t(`overlayEditor.goalStatus.${state.status}`)}
        </p>
      )}
      <SelectField<GoalMetric>
        id={`${ids}-metric`}
        label={t('overlayEditor.goalEditor.metricLabel')}
        value={goal.metric}
        items={GOAL_METRICS.map((value) => ({
          value,
          label: t(`overlayEditor.goalMetrics.${value}`),
        }))}
        onChange={(metric) => update({ metric, platform: GOAL_METRIC_PLATFORM[metric] })}
      />
      <p className="text-xs text-text-sub">
        {t(polled ? 'overlayEditor.goalEditor.polledHint' : 'overlayEditor.goalEditor.countedHint')}
      </p>
      <TextField
        id={`${ids}-title`}
        label={t('overlayEditor.goalEditor.titleLabel')}
        value={goal.title}
        maxLength={200}
        onChange={(e) => update({ title: e.target.value })}
      />
      <TextField
        id={`${ids}-target`}
        label={t('overlayEditor.goalEditor.targetLabel')}
        type="number"
        min={1}
        value={goal.target}
        onChange={(e) => update({ target: Math.max(0, Math.round(Number(e.target.value))) })}
        className="w-40"
      />
      <TextField
        id={`${ids}-start`}
        label={t('overlayEditor.goalEditor.startLabel')}
        hint={t(
          polled
            ? 'overlayEditor.goalEditor.startHintPolled'
            : 'overlayEditor.goalEditor.startHintCounted'
        )}
        type="number"
        min={0}
        value={goal.starting_value}
        onChange={(e) =>
          update({ starting_value: Math.max(0, Math.round(Number(e.target.value))) })
        }
        className="w-40"
      />
      {!polled && (
        <ToggleSwitch
          label={t('overlayEditor.goalEditor.resetLabel')}
          checked={reset}
          onChange={setReset}
        />
      )}
      <Button type="button" onClick={() => void save()} disabled={saving || goal.target < 1}>
        {t(saving ? 'overlayEditor.goalEditor.saving' : 'overlayEditor.goalEditor.save')}
      </Button>
    </section>
  )
}

export function ListEditor({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
  const ids = useId()
  const [list, setList] = useState<ListConfig | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    overlaysApi
      .getList(overlayId)
      .then((loaded) => setList(loaded.list))
      .catch(() =>
        toastManager.add({ title: t('overlayEditor.listEditor.loadFailed'), type: 'error' })
      )
  }, [overlayId, t])

  if (!list) return null
  const update = (patch: Partial<ListConfig>) => setList({ ...list, ...patch })

  async function save() {
    if (!list) return
    setSaving(true)
    try {
      await overlaysApi.saveList(overlayId, list)
      toastManager.add({ title: t('overlayEditor.listEditor.saved'), type: 'success' })
    } catch (err) {
      toastManager.add({
        title: saveError(err, t('overlayEditor.listEditor.saveFailed')),
        type: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="space-y-5">
      <div>
        <p className="text-xs text-text-sub">{t('overlayEditor.listEditor.body')}</p>
      </div>
      <SelectField<ListCategory>
        id={`${ids}-category`}
        label={t('overlayEditor.listEditor.categoryLabel')}
        value={list.category}
        items={LIST_CATEGORIES.map((value) => ({
          value,
          label: t(`overlayEditor.listCategories.${value}`),
        }))}
        onChange={(category) => update({ category })}
      />
      <TextField
        id={`${ids}-title`}
        label={t('overlayEditor.listEditor.titleLabel')}
        value={list.title}
        maxLength={200}
        onChange={(e) => update({ title: e.target.value })}
      />
      <TextField
        id={`${ids}-limit`}
        label={t('overlayEditor.listEditor.limitLabel', { max: MAX_LIST_LIMIT })}
        type="number"
        min={1}
        max={MAX_LIST_LIMIT}
        value={list.limit}
        onChange={(e) =>
          update({
            limit: Math.min(MAX_LIST_LIMIT, Math.max(1, Math.round(Number(e.target.value)))),
          })
        }
        className="w-32"
      />
      <Button type="button" onClick={() => void save()} disabled={saving}>
        {t(saving ? 'overlayEditor.listEditor.saving' : 'overlayEditor.listEditor.save')}
      </Button>
    </section>
  )
}
