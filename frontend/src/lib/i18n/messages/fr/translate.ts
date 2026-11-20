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
  heading: 'Aidez à traduire All-Chat',
  intro:
    "Choisissez une langue et traduisez l'interface, un texte à la fois. Tout ce que vous envoyez est relu avant d'être mis en ligne.",
  gateHeading: 'La traduction est réservée aux bêta-testeurs',
  gateBody:
    "Proposez des traductions en tant que bêta-testeur. Demandez sur le Discord si vous souhaitez aider et que vous n'êtes pas encore dans la bêta.",
  localeLabel: 'Langue',
  localeNone: 'Aucune langue pour le moment',
  localeRequestButton: 'Demander une langue',
  localeRequestTitle: 'Demander une langue',
  localeRequestBody:
    'Les langues demandées apparaissent pour tout le monde une fois approuvées par un administrateur.',
  localeCodeLabel: 'Code de langue',
  localeCodePlaceholder: 'de, fr, pt-BR…',
  localeEnglishNameLabel: 'Nom en anglais',
  localeNativeNameLabel: 'Nom dans la langue elle-même',
  localeRequestSubmit: 'Envoyer la demande',
  localeRequestedToast: "Demande envoyée, nous l'examinerons sous peu",
  localeRequestFailedToast: "Impossible d'envoyer la demande",
  namespaceIntro:
    'Choisissez une section à traduire. Votre progression est enregistrée par section.',
  namespaceProgress: '{done} textes sur {total}',
  sourceLabel: 'Anglais',
  yourTranslationLabel: 'Votre traduction',
  placeholderHint:
    'Conservez {placeholder} dans votre traduction : il est remplacé par une valeur.',
  placeholderMismatch: 'Il manque {placeholder} dans votre traduction.',
  statusPending: 'En attente de relecture',
  statusApproved: 'Approuvée',
  statusRejected: 'Modifications demandées',
  rejectedNote: 'Note du relecteur : {note}',
  saveButton: 'Envoyer {count} traduction(s)',
  savedToast: '{count} traduction(s) envoyée(s) pour relecture',
  saveFailedToast: "Échec de l'envoi",
  rowError: 'Non envoyée : {error}',
  backToNamespaces: 'Toutes les sections',
  stringLabel: 'Traduire : {key}',
}
