import type { Meta, StoryObj } from '@storybook/nextjs-vite'

import { Input } from '@/components/ui/input'

const meta = {
  title: 'UI/Input',
  component: Input,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Input>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: {} }
export const Disabled: Story = { args: { disabled: true } }
export const WithPlaceholder: Story = { args: { placeholder: 'Search...' } }
export const Small: Story = { args: { size: 'sm' } }
