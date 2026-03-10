import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/input.tsx is created in Plan 02
function Input({ className, ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input data-slot="input" className={className} {...props} />
}

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
export const WithPlaceholder: Story = { args: { placeholder: 'Enter text...' } }
