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

/**
 * Accessible form-field primitive (Base UI Field).
 *
 * The workhorse of the WCAG form-labeling batches: `Field.Root` wires
 * label ↔ control ↔ description ↔ error associations (htmlFor,
 * aria-describedby, aria-invalid) automatically — no manual id plumbing.
 *
 *   <Field.Root>
 *     <Field.Label>Channel name</Field.Label>
 *     <Field.Control render={<Input />} />   // or any control, e.g. Switch
 *     <Field.Description>Shown under your messages.</Field.Description>
 *     <Field.Error />                        // renders on validation failure
 *   </Field.Root>
 */

import { Field as FieldPrimitive } from '@base-ui/react/field'
import { cn } from '@/lib/utils'

function FieldRoot({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof FieldPrimitive.Root>) {
  return (
    <FieldPrimitive.Root
      data-slot="field"
      className={cn('flex flex-col gap-1', className)}
      {...props}
    />
  )
}

function FieldLabel({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof FieldPrimitive.Label>) {
  return (
    <FieldPrimitive.Label
      data-slot="field-label"
      className={cn('text-sm font-medium text-text', className)}
      {...props}
    />
  )
}

function FieldDescription({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof FieldPrimitive.Description>) {
  return (
    <FieldPrimitive.Description
      data-slot="field-description"
      className={cn('text-sm text-text-sub', className)}
      {...props}
    />
  )
}

function FieldError({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof FieldPrimitive.Error>) {
  return (
    <FieldPrimitive.Error
      data-slot="field-error"
      className={cn('text-sm text-red-400', className)}
      {...props}
    />
  )
}

export const Field = {
  Root: FieldRoot,
  Label: FieldLabel,
  Control: FieldPrimitive.Control,
  Description: FieldDescription,
  Error: FieldError,
}
