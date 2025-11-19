/**
 * Monaco CSS Editor Component
 *
 * A code editor component using Monaco Editor (the same editor that powers VS Code)
 * for editing CSS with syntax highlighting, autocomplete, and validation.
 */

'use client';

import { Editor, OnMount } from '@monaco-editor/react';
import { useRef, useEffect } from 'react';

interface MonacoCSSEditorProps {
  value: string;
  onChange: (value: string) => void;
  height?: string;
  placeholder?: string;
  readOnly?: boolean;
}

export default function MonacoCSSEditor({
  value,
  onChange,
  height = '300px',
  placeholder = '/* Enter your custom CSS here */',
  readOnly = false
}: MonacoCSSEditorProps) {
  const editorRef = useRef<any>(null);

  const handleEditorDidMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

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
      readOnly
    });

    // Set CSS language
    monaco.editor.setModelLanguage(editor.getModel()!, 'css');
  };

  const handleEditorChange = (value: string | undefined) => {
    onChange(value || '');
  };

  // Update editor value when prop changes (for external updates like "Load Example")
  useEffect(() => {
    if (editorRef.current) {
      const editor = editorRef.current;
      const currentValue = editor.getValue();
      if (currentValue !== value) {
        editor.setValue(value);
      }
    }
  }, [value]);

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
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
          wrappingIndent: 'indent'
        }}
        loading={
          <div className="flex items-center justify-center h-full bg-gray-900">
            <div className="text-gray-400 text-sm">Loading editor...</div>
          </div>
        }
      />
    </div>
  );
}
