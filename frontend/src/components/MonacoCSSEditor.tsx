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

import { Editor, OnMount } from '@monaco-editor/react'
import { useRef, useEffect } from 'react'

interface MonacoCSSEditorProps {
  value: string
  onChange: (value: string) => void
  height?: string
  placeholder?: string
  readOnly?: boolean
}

export default function MonacoCSSEditor({
  value,
  onChange,
  height = '300px',
  placeholder = '/* Enter your custom CSS here */',
  readOnly = false,
}: MonacoCSSEditorProps) {
  const editorRef = useRef<any>(null)

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
    onChange(value || '')
  }

  // Update editor value when prop changes (for external updates like "Load Example")
  useEffect(() => {
    if (editorRef.current) {
      const editor = editorRef.current
      const currentValue = editor.getValue()
      if (currentValue !== value) {
        editor.setValue(value)
      }
    }
  }, [value])

  return (
    <div className="overflow-hidden rounded-lg border border-border">
      <Editor
        height={height}
        defaultLanguage="css"
        value={value}
        onChange={handleEditorChange}
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
  )
}
