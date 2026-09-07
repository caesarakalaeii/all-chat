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

/**
 * Stories for the primitives added in ADR-0056 (tabs, select, toggle-group,
 * textarea, label, separator).
 *
 * These exist for the axe gate as much as for the docs: `npx vitest --project
 * storybook` runs axe over every story with `a11y: 'error'`, so a primitive
 * without a story has no accessibility coverage at all. Grouped into one file
 * because each is a handful of lines and they are reviewed as a set.
 */

import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

function Primitives() {
  return <TabsDemo />
}

function TabsDemo() {
  return (
    <Tabs defaultValue="sources" className="w-96">
      <TabsList>
        <TabsTrigger value="sources">Sources</TabsTrigger>
        <TabsTrigger value="appearance">Appearance</TabsTrigger>
        <TabsTrigger value="moderation">Moderation</TabsTrigger>
      </TabsList>
      <TabsContent value="sources">Twitch, YouTube and Kick are connected.</TabsContent>
      <TabsContent value="appearance">Pick a theme or write your own CSS.</TabsContent>
      <TabsContent value="moderation">Delete, timeout and ban from the monitor view.</TabsContent>
    </Tabs>
  )
}

const meta = {
  title: 'UI/Primitives',
  component: Primitives,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Primitives>

export default meta
type Story = StoryObj<typeof meta>

export const TabsDefault: Story = {}

export const TabsLine: Story = {
  render: () => (
    <Tabs defaultValue="all" className="w-96">
      <TabsList variant="line" className="w-full justify-start border-b border-border">
        <TabsTrigger value="all">All</TabsTrigger>
        <TabsTrigger value="active">Active</TabsTrigger>
        <TabsTrigger value="banned">Banned</TabsTrigger>
      </TabsList>
    </Tabs>
  ),
}

export const SelectField: Story = {
  render: () => (
    <div className="flex w-72 flex-col gap-2">
      <Label htmlFor="story-select">Default send target</Label>
      <Select defaultValue="twitch">
        <SelectTrigger id="story-select">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="twitch">Twitch</SelectItem>
          <SelectItem value="youtube">YouTube</SelectItem>
          <SelectItem value="kick">Kick</SelectItem>
        </SelectContent>
      </Select>
    </div>
  ),
}

export const ToggleGroupSingle: Story = {
  render: () => (
    <ToggleGroup defaultValue={['left']}>
      <ToggleGroupItem value="left" aria-label="Align left">
        Left
      </ToggleGroupItem>
      <ToggleGroupItem value="center" aria-label="Align center">
        Center
      </ToggleGroupItem>
      <ToggleGroupItem value="right" aria-label="Align right">
        Right
      </ToggleGroupItem>
    </ToggleGroup>
  ),
}

export const TextareaField: Story = {
  render: () => (
    <div className="flex w-72 flex-col gap-2">
      <Label htmlFor="story-textarea">Greeting message</Label>
      <Textarea id="story-textarea" placeholder="Welcome to the stream!" rows={3} />
    </div>
  ),
}

export const TextareaInvalid: Story = {
  render: () => (
    <div className="flex w-72 flex-col gap-2">
      <Label htmlFor="story-textarea-invalid">Greeting message</Label>
      <Textarea id="story-textarea-invalid" aria-invalid defaultValue="" rows={3} />
      <p className="text-sm text-destructive">A greeting is required.</p>
    </div>
  ),
}

export const SeparatorInStack: Story = {
  render: () => (
    <div className="w-72 rounded-xl border border-border bg-surface p-4">
      <p className="text-sm text-text">Overlay settings</p>
      <Separator className="my-3" />
      <p className="text-sm text-text-sub">Chat sources</p>
    </div>
  ),
}
