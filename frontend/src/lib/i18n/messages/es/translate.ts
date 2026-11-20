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

import type { LocaleNamespace } from '../../translate'

export const translate: LocaleNamespace<'translate'> = {
  heading: 'Ayuda a traducir All-Chat',
  intro:
    'Elige un idioma y traduce la interfaz, texto a texto. Todo lo que envías se revisa antes de publicarse.',
  gateHeading: 'La traducción es una función para beta testers',
  gateBody:
    'Aporta traducciones como beta tester. Pregunta en Discord si quieres ayudar y aún no estás en la beta.',
  localeLabel: 'Idioma',
  localeNone: 'Aún no hay idiomas',
  localeRequestButton: 'Solicitar un idioma',
  localeRequestTitle: 'Solicitar un idioma',
  localeRequestBody:
    'Los idiomas solicitados aparecen para todo el mundo cuando un administrador los aprueba.',
  localeCodeLabel: 'Código de idioma',
  localeCodePlaceholder: 'de, fr, pt-BR…',
  localeEnglishNameLabel: 'Nombre en inglés',
  localeNativeNameLabel: 'Nombre en el propio idioma',
  localeRequestSubmit: 'Enviar solicitud',
  localeRequestedToast: 'Solicitud enviada: la revisaremos en breve',
  localeRequestFailedToast: 'No se pudo enviar la solicitud',
  namespaceIntro: 'Elige un área para traducir. Tu progreso se guarda por área.',
  namespaceProgress: '{done} de {total} textos',
  sourceLabel: 'Inglés',
  yourTranslationLabel: 'Tu traducción',
  placeholderHint: 'Conserva {placeholder} en tu traducción: se sustituye por un valor.',
  placeholderMismatch: 'A tu traducción le falta {placeholder}.',
  statusPending: 'Pendiente de revisión',
  statusApproved: 'Aprobada',
  statusRejected: 'Cambios solicitados',
  rejectedNote: 'Nota del revisor: {note}',
  saveButton: 'Enviar {count} traducción(es)',
  savedToast: '{count} traducción(es) enviada(s) a revisión',
  saveFailedToast: 'Error al enviar',
  rowError: 'No enviada: {error}',
  backToNamespaces: 'Todas las áreas',
  stringLabel: 'Traducir: {key}',
}
