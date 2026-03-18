'use client'

import React from 'react'

export interface ToggleSwitchProps {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}

export function ToggleSwitch({ label, checked, onChange }: ToggleSwitchProps): React.ReactElement {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-text-sub">{label}</span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors duration-200 ${
          checked ? 'bg-twitch' : 'bg-surface-alt'
        }`}
      >
        <span
          className={`inline-block h-4 w-4 translate-y-0.5 rounded-full bg-white shadow transition duration-200 ease-in-out ${
            checked ? 'translate-x-4' : 'translate-x-0.5'
          }`}
        />
      </button>
    </div>
  )
}
