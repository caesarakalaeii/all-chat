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

export const onboarding: LocaleNamespace<'onboarding'> = {
  steps: {
    createOverlay: 'Erstelle dein Overlay',
    connectSource: 'Verbinde eine Chatquelle',
    chooseTheme: 'Wähle ein Theme',
    copyObs: 'Füge es in OBS hinzu',
  },
  checklist: {
    title: 'Einrichtungsanleitung',
    allDone: 'Alle Schritte erledigt!',
    progress: '{done} von {total} Schritten erledigt',
    expand: 'Einrichtungsanleitung aufklappen',
    minimize: 'Einrichtungsanleitung einklappen',
    dismiss: 'Einrichtungsanleitung ausblenden',
    stepDone: ' (erledigt)',
    create: 'Erstellen',
    showMe: 'Zeig mir',
    openEditor: 'Editor öffnen',
    copyLink: 'Link kopieren',
    copied: 'Kopiert!',
  },
  obs: {
    heading: 'In OBS:',
    step1: 'Klicke in OBS auf {plus} unter Quellen und wähle {browser}.',
    step1Plus: '+',
    step1Browser: 'Browser',
    step2: 'Füge den kopierten Overlay-Link in das URL-Feld ein.',
    step3:
      'Passe die Quelle an den Bereich an, den der Chat füllen soll (eine schmale, hohe Box wie 450×800 eignet sich gut, nicht die gesamte Leinwand), und ziehe sie dann an ihren Platz. Der Chat erscheint, sobald sich das Overlay verbindet.',
  },
  obsDock: {
    step1: 'Öffne in OBS {menu}.',
    step1Menu: 'Ansicht → Docks → Benutzerdefinierte Browser-Docks',
    step2:
      'Gib dem Dock einen Namen, füge den kopierten Dock-Link in das URL-Feld ein, klicke dann auf Anwenden und schließe den Dialog.',
    step3:
      'Das Dock hat eine eigene Anmeldung — melde dich einmal innerhalb des Panels an und bleib angemeldet. Ziehe das Panel dorthin, wo du es haben willst; eine schmale Spalte neben Szenen und Quellen eignet sich gut.',
  },
  extras: {
    heading: 'Optional: Mehr rausholen',
    ttsTitle: 'Text-to-Speech',
    ttsBody:
      'Lies den Chat laut vor. Browser-Stimmen sind kostenlos; ElevenLabs-Stimmen sind Premium.',
    moderationTitle: 'Moderiere aus deinem Overlay',
    moderationBody:
      'Löschen, Timeout, Bann und Entbannen über die Monitor-Ansicht-Schaltfläche oben im Editor. Voller Funktionsumfang auf Twitch, Kick und Discord; Timeout und Ban auf YouTube. (Premium)',
    moderatorsTitle: 'Lass deine Mods helfen',
    moderatorsBody:
      'Gib die Monitor-Ansicht unter Moderatoren an deine bestehenden Moderatoren weiter. Sie handeln mit ihren eigenen Plattform-Konten und brauchen selbst nie Premium. (Premium)',
    sharedChatTitle: 'Geteilter Chat',
    sharedChatBody:
      'Fasse mehrere Kanäle über die Schaltfläche Overlay teilen zu einer Unterhaltung zusammen. (Premium)',
    streamSelectionTitle: 'YouTube-Stream-Auswahl',
    streamSelectionBody:
      'Lege genau fest, welcher Stream ein Overlay empfängt, pro YouTube-Quelle unter Quellen. (Premium)',
    youtubeOfficialApiTitle: 'Nicht gelistete und members-only YouTube-Streams',
    youtubeOfficialApiBody:
      'Aktiviere die offizielle YouTube-API bei einer YouTube-Quelle unter Quellen, und dein eigener Kanal wird über deine YouTube-Verbindung ausgelesen: nicht gelistete und members-only Streams werden automatisch gefunden. (Premium)',
    bubbleColorsTitle: 'Unterschiedlich farbige Sprechblasen',
    bubbleColorsBody:
      'Gib jeder Plattform ihre eigene Blasenfarbe oder Spiele eine Palette über den Chat hinweg durch, unter Blasenfarben in Darstellung. Kostenlos.',
    flairsTitle: 'Zuschauer-Flairs',
    flairsBody:
      'Premium-Kosmetik für Chatter, zum Beispiel animierte Namensverläufe, unter Flairs in der Navigation.',
    streamDeckTitle: 'Stream-Deck-Tasten',
    streamDeckBody:
      'Steuere Umfragen, Vorhersagen und vorbereitete Nachrichten über eine physische Taste. Kopple ein Stream Deck oder StreamController unter Gekoppelte Geräte – ohne etwas zu kopieren oder einzufügen. Eine Umfrage oder Vorhersage zu starten bleibt Premium.',
    languageTitle: 'All-Chat auf Deutsch nutzen',
    languageBody:
      'Wechsle die Sprache der Oberfläche unter Einstellungen. Deutsch ist die erste Sprache neben Englisch, und die Auswahl wird pro Browser gemerkt.',
    translateTitle: 'Hilf beim Übersetzen von All-Chat',
    translateBody:
      'Betatester können die Oberfläche unter Übersetzen in der Navigation in weitere Sprachen übertragen. Alles wird vor der Veröffentlichung geprüft.',
    seePremium: 'Sieh alles, was Premium enthält',
    done: 'Fertig',
    skip: 'Überspringen',
  },
  finale: {
    title: 'Du bist live! 🎉',
    body: 'Fragen, Feedback oder Theme-Wünsche? Unsere Community hilft gerne weiter.',
    joinDiscord: 'Tritt dem Discord bei',
    finish: 'Fertigstellen',
  },
  dismissDialog: {
    title: 'Einrichtungsanleitung ausblenden?',
    body: 'Du kannst sie jederzeit unter Einstellungen → Einrichtungsanleitung neu starten.',
    keep: 'Behalten',
    hide: 'Anleitung ausblenden',
  },
  createDialog: {
    title: 'Erstelle dein Overlay',
    body: 'Gib ihm einen Namen — deine Chatquelle verbindest du gleich danach.',
    nameLabel: 'Overlay-Name',
    cancel: 'Abbrechen',
    submit: 'Overlay erstellen',
    submitting: 'Wird erstellt…',
    failedToast: 'Das Overlay konnte nicht erstellt werden',
  },
}
