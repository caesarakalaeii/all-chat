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
 * Monaco CSS Editor Component
 *
 * A code editor component using Monaco Editor (the same editor that powers VS Code)
 * for editing CSS with syntax highlighting, autocomplete, and validation.
 */

'use client'

// Side-effect import: points the Monaco loader at our self-hosted /monaco/vs
// (see src/lib/monaco.ts + ADR-0040). MUST run before <Editor> mounts, so it
// stays the first import; otherwise the loader falls back to cdn.jsdelivr.net,
// which the app CSP blocks — leaving the editor stuck on "Loading editor...".
import '@/lib/monaco'
import { Editor, OnMount, OnValidate } from '@monaco-editor/react'
import { useRef, useEffect } from 'react'
import { markerSeverityToLevel } from '@/lib/utils/custom-css'
import type { CssIssue } from '@/lib/utils/custom-css'

export type { CssIssue }

interface MonacoCSSEditorProps {
  value: string
  onChange: (value: string) => void
  /** Fired whenever Monaco re-validates the model; use to surface tips for broken CSS. */
  onValidate?: (issues: CssIssue[]) => void
  height?: string
  placeholder?: string
  readOnly?: boolean
}

export default function MonacoCSSEditor({
  value,
  onChange,
  onValidate,
  height = '300px',
  placeholder = '/* Enter your custom CSS here */',
  readOnly = false,
}: MonacoCSSEditorProps) {
  const editorRef = useRef<Parameters<OnMount>[0] | null>(null)
  // Guards the onChange that Monaco fires when WE call setValue() to sync an
  // external value (theme preload, "Reset to theme"). Without this, a preload
  // would look like a user edit and wrongly fork the overlay off its theme.
  const suppressNextChange = useRef(false)

  const handleEditorDidMount: OnMount = (editor, monaco) => {
    editorRef.current = editor

    // Configure editor options
    editor.updateOptions({
      minimap: { enabled: false },
      fontSize: 13,
      lineNumbers: 'on',
      renderLineHighlight: 'all',
      scrollBeyondLastLine: false,
      automaticLayout: true,
      tabSize: 2,
      wordWrap: 'on',
      wrappingIndent: 'indent',
      readOnly,
    })

    // Set CSS language
    monaco.editor.setModelLanguage(editor.getModel()!, 'css')
  }

  const handleEditorChange = (value: string | undefined) => {
    if (suppressNextChange.current) {
      suppressNextChange.current = false
      return
    }
    onChange(value || '')
  }

  const handleValidate: OnValidate = (markers) => {
    if (!onValidate) return
    onValidate(
      markers.map((m) => ({
        line: m.startLineNumber,
        column: m.startColumn,
        message: m.message,
        severity: markerSeverityToLevel(m.severity),
      }))
    )
  }

  // Update editor value when prop changes (theme preload / "Reset to theme").
  // setValue triggers Monaco's change event, so flag it as programmatic first.
  useEffect(() => {
    if (editorRef.current) {
      const editor = editorRef.current
      const currentValue = editor.getValue()
      if (currentValue !== value) {
        suppressNextChange.current = true
        editor.setValue(value)
      }
    }
  }, [value])

  return (
    // role="group" + aria-label mark the editor region for assistive tech;
    // the hint below documents Monaco's built-in keyboard-trap escape
    // (WCAG 2.1.2): Ctrl+M toggles tab-focus mode, Escape releases focus.
    <div role="group" aria-label="Custom CSS editor">
      <p className="mb-1 text-xs text-text-dim">
        Press Ctrl+M to toggle Tab capturing; Escape then Tab leaves the editor.
      </p>
      <div className="overflow-hidden rounded-lg border border-border">
        <Editor
          height={height}
          defaultLanguage="css"
          value={value}
          onChange={handleEditorChange}
          onValidate={handleValidate}
          onMount={handleEditorDidMount}
          theme="vs-dark"
          options={{
            readOnly,
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: 'on',
            renderLineHighlight: 'all',
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
            wordWrap: 'on',
            wrappingIndent: 'indent',
          }}
          loading={
            <div className="flex h-full items-center justify-center bg-bg">
              <div className="text-sm text-text-dim">Loading editor...</div>
            </div>
          }
        />
      </div>
    </div>
  )
}
