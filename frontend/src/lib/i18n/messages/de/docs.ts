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

export const docs: LocaleNamespace<'docs'> = {
  fieldTable: {
    columnField: 'Feld',
    columnType: 'Typ',
    columnDescription: 'Beschreibung',
  },
  prose: {
    codeSample: 'Codebeispiel',
  },
  api: {
    eyebrow: 'Entwickler-API',
    heading: 'Entwickler-API-Referenz',
    intro:
      'Lese den vereinheitlichten Chat-Stream über eine einzige WebSocket, um Bots, Moderations-Tools, Vote-Zähler, Analytics, Alerts und alles Weitere zu bauen.',
    guidePrompt: 'Du richtest gerade dein Overlay ein? {guide}',
    guideLinkText: 'Zum Streamer-Guide →',
    tocHeading: 'Auf dieser Seite',
    tocConnectATool: 'Ein Tool verbinden',
    tocMessageFormat: 'Nachrichtenformat',
    tocChatMessages: 'Chat-Nachrichten',
    tocEvents: 'Events',
    tocStatusMessages: 'Status- und Steuerungsnachrichten',
    tocReconnecting: 'Wiederverbinden und Heartbeats',
    tocTryItNow: 'Jetzt ausprobieren (ohne Setup)',
    connectHeading: 'Ein Tool verbinden',
    connectBody:
      'All-Chat bündelt den Live-Chat von {twitch}, {youtube}, {kick}, {tiktok} und {discord} in einen einzigen normalisierten Stream — jede Nachricht, dazu Plattform-Events wie Subs, Bits, Raids, Super Chats und Geschenke, in einem Format, mit aufgelösten 7TV/BTTV/FFZ-Emotes. Derselbe Stream, der das Browser-Overlay antreibt, ist der, den dein Tool liest.',
    connectAnonymous:
      'Öffne eine WebSocket zum Overlay-Endpunkt. Das Lesen des Streams ist {anonymous} — kein Token und kein Konto nötig (das ist derselbe „OBS-Modus“, den das Browser-Overlay nutzt):',
    connectAnonymousEmphasis: 'anonym',
    connectOverlayId:
      'Deine {field} ist die UUID in der Browser-Quelle-URL deines Overlays. Willst du es erst ohne Konto ausprobieren? Nutze das {testOverlay}.',
    connectTestOverlayLinkText: 'öffentliche Test-Overlay',
    queryParamsHeading: 'Query-Parameter',
    exampleJavascriptHeading: 'Minimalbeispiel (JavaScript)',
    examplePythonHeading: 'Minimalbeispiel (Python)',
    formatHeading: 'Nachrichtenformat',
    formatBody: 'Jeder Frame ist JSON mit demselben Umschlag:',
    formatTypeList: '{field} ist einer von:',
    chatHeading: 'Chat-Nachrichten',
    chatBody: 'Für {chat} und {update} ist {data} das vereinheitlichte Nachrichtenobjekt:',
    userHeading: 'user',
    messageHeading: 'message, emotes und Attachments',
    messageBody: '{field} ist {shape}. Jedes {emote}:',
    attachmentsBody:
      '{field} ist nur vorhanden, wenn eine Nachricht Medien enthält (aktuell Discord-Bild-/GIF-/Video-Uploads und Tenor-/Giphy-Linkvorschauen). Jedes {attachment}:',
    badgeBody: 'Jeder {badge} ist {shape} — z. B. {example}.',
    exampleHeading: 'Beispiel',
    eventsHeading: 'Events',
    eventsBody:
      'Plattform-Events (Subs, Bits, Raids, Spenden, …) kommen als {chat}-Frames, deren {data} ein {event}-Objekt enthält. Normale Chats haben kein {event}-Feld.',
    eventTypesHeading: 'Event-Typen nach Plattform',
    eventTypesLabel: '{platform}:',
    exampleSubscriptionHeading: 'Beispiel (Twitch-Abo)',
    statusHeading: 'Status- und Steuerungsnachrichten',
    statusBody:
      'Beim Verbinden schickt der Server zuerst einen {connected}-Frame, dann einen {status}-Frame pro konfigurierter Quelle, damit deine UI sofort Indikatoren zeigen kann. {status} wird nur auf dem Overlay-WebSocket zugestellt — der anonyme Zuschauer-Pfad erhält ihn nie. {status}-Daten:',
    reconnectHeading: 'Wiederverbinden und Heartbeats',
    reconnectPing:
      'Der Server sendet periodisch {ping}. Antworte mit {pong}. (Die meisten WebSocket-Bibliotheken beantworten auch Low-Level-Ping-Frames automatisch.)',
    reconnectBackfill:
      'Wenn die Verbindung abbricht, verbinde dich mit Backoff neu. Um Nachrichten nachzuliefern, die während einer kurzen Unterbrechung verpasst wurden, verbinde dich mit {since} auf den Zeitpunkt direkt vor dem Verbindungsabbruch — der Server spielt darauf neue gepufferte Nachrichten ab.',
    reconnectDedup:
      'Behandle IDs als Dedup-Schlüssel, wenn du den Replay-Puffer mit Live-Nachrichten zusammenführst.',
    tryItHeading: 'Jetzt ausprobieren (ohne Setup)',
    tryItBody:
      'Es gibt ein öffentliches Test-Overlay, das realistischen Fake-Traffic streamt — zufälligen Chat, Umfrage-Stimmen (die wörtlichen Nachrichten {one}, {two}, {three}, {four}) und Plattform-Events —, damit du eine Integration ohne Konto oder echten Kanal bauen und prüfen kannst. Verbinde dich einfach; der Traffic läuft, solange ein Client verbunden ist, und stoppt, wenn sich der letzte trennt:',
    tryItOutro:
      'Füge diese URL in eines der Beispiele oben ein und du wirst sofort Nachrichten und Events hereinstromen sehen.',
    footerCopyright: '© {year} All-Chat',
    footerGuideLink: 'Streamer-Guide',
    footerPrivacyLink: 'Datenschutzerklärung',
    footerTermsLink: 'Nutzungsbedingungen',
  },
  apiFields: {
    queryParamsToken:
      'Owner-Token. Nur für owner-scoped Zugriff nötig; für reinen Lesezugriff weglassen.',
    queryParamsSince:
      'Millisekunden seit der Unix-Epoche. Beim Verbinden spielt der Server gepufferte Nachrichten ab, die neuer sind als dieser Zeitstempel, damit sich ein Client, der sich neu verbindet, die Lücke nachholen kann. Weglassen, um live zu starten.',
    envelopeType: 'Nachrichtentyp (siehe unten).',
    envelopeData: 'Payload; Form hängt vom Typ ab. Ohne ping/pong ausgelassen.',
    envelopeTimestamp: 'RFC-3339-Zeitstempel, wann das Gateway den Frame gesendet hat.',
    messageTypesChatMessage:
      'Eine Chat-Nachricht oder ein Plattform-Event. data ist das vereinheitlichte Nachrichtenobjekt.',
    messageTypesMessageUpdate:
      'Ein Update zu einer zuvor gesendeten Nachricht (z. B. TikTok-Like-Aggregation). Dieselbe Datenform wie chat_message.',
    messageTypesConnected: 'Beim Verbinden einmal gesendet: { overlay_id, message }.',
    messageTypesPlatformStatus: 'Verbindungsstatus einer Quell-Plattform.',
    messageTypesPing: 'Herzschlag vom Server. Antworte mit { "type": "pong" }.',
    messageTypesError: 'Fehlerhinweis: { code, message }.',
    chatMessageId: 'Eindeutige Nachrichten-ID.',
    chatMessageOverlayId: 'Overlay, an das diese Nachricht zugestellt wurde.',
    chatMessagePlatform: '"twitch" | "youtube" | "kick" | "tiktok" | "discord".',
    chatMessageChannelId: 'Plattform-Kanalkennung.',
    chatMessageChannelName: 'Menschlich lesbarer Kanalname.',
    chatMessageUser: 'Informationen zum Verfasser (siehe unten).',
    chatMessageMessage: '{ text, emotes[], attachments[]? } (siehe unten).',
    chatMessageTimestamp: 'RFC-3339-Nachrichtenzeit (UTC).',
    chatMessageMetadata: 'Freie, plattformspezifische Zusatzfelder.',
    chatMessageEvent:
      'Nur vorhanden, wenn die Nachricht ein Plattform-Event ist (siehe Events). Bei normalem Chat abwesend.',
    userId: 'Plattform-Benutzer-ID.',
    userUsername: 'Login/Handle.',
    userDisplayName: 'Anzeigename.',
    userColor: 'Namensfarbe, hex (z. B. "#FF0000").',
    userBadges: 'Badges des Verfassers (siehe unten).',
    userAvatarUrl: 'Profilbild-URL, wenn verfügbar.',
    userPronouns: 'z. B. "she/her", wenn bekannt.',
    userNameGradient: 'Optionaler Gradient-Deskriptor (JSON-String).',
    userSourceBadgesSourceUserId: 'Ursprungskanal-Identität für Shared-Chat-Nachrichten.',
    userAvatarFrameUrlAvatarFlairUrl: 'Dekorativer Rahmen/Flair, wenn gesetzt.',
    emoteCode: 'Emote-Text-Token, z. B. "Kappa".',
    emoteProvider: '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | Plattform.',
    emoteUrl: 'CDN-Bild-URL.',
    emotePositions: 'Array von [start, end]-Indexpaaren in text, an denen das Emote vorkommt.',
    attachmentType: '"image" oder "video". GIFs sind animierte Bilder.',
    attachmentUrl: 'Medien-URL.',
    attachmentContentType: 'MIME-Type, z. B. "image/gif", "video/mp4".',
    attachmentWidthHeight: 'Echte Pixelabmessungen, wenn bekannt.',
    attachmentThumbUrl: 'Vorschaubild für Videos, wenn verfügbar.',
    attachmentSpoiler: 'true, wenn der Absender die Medien als Spoiler markiert hat.',
    attachmentFilename: 'Originaldateiname (für Alternativtext genutzt).',
    eventType: 'Event-Typ (Liste unten).',
    eventTier: 'Relative Prominenz: "high" | "medium" | "low".',
    eventValue:
      '{ amount: number, currency: string, display_text: string } — z. B. amount 250, currency "bits", display_text "250 bits".',
    eventDuration: 'Empfohlene Anzeigedauer auf dem Bildschirm, in Sekunden.',
    eventIsUpdate:
      'true, wenn dies ein früheres Event aktualisiert (z. B. TikTok-Like-Aggregation, zugestellt als message_update).',
    eventAggregationId: 'Gruppiert aufeinanderfolgende Updates derselben Aggregation.',
    eventMetadata: 'Eventspezifische Rohfelder.',
    platformStatusPlatform: 'Quell-Plattform.',
    platformStatusChannelId: 'Quellkanal.',
    platformStatusChannelName: 'Menschlich lesbarer Kanalname.',
    platformStatusStatus: '"connected" | "reconnecting" | "offline" | "quota_exceeded".',
    platformStatusNextRetryAt:
      'RFC-3339-Zeit des nächsten Wiederverbindungsversuchs, falls zutreffend.',
    platformStatusErrorMessage: 'Menschlich lesbare Details im Störungsfall.',
  },
  guide: {
    eyebrow: 'All-Chat Docs',
    heading: 'Streamer-Guide',
    intro:
      'Richte alles in OBS ein, starte Umfragen, Vorhersagen und Moderation über den Chat-Monitor, und mache das Overlay zu deinem eigenen.',
    apiPrompt: 'Du baust einen Bot, eine Alert-Box oder ein Analytics-Tool? {api}',
    apiLinkText: 'Zur Entwickler-API →',
    tocHeading: 'Auf dieser Seite',
    tocWhatIsAllChat: 'Was ist All-Chat',
    tocGettingStarted: 'Overlay live schalten',
    tocIrl: '24/7- und IRL-Streams',
    tocTiktok: 'TikTok: Funktionsweise und Grenzen',
    tocMonitor: 'Der Chat-Monitor',
    tocModeration: 'Deinen Chat moderieren',
    tocEngagement: 'Umfragen, Vorhersagen & Punkte',
    tocEventsCredits: 'Events & Abspann',
    tocSharing: 'Overlay teilen',
    tocThemes: 'Ein Theme wählen',
    tocCustomize: 'Dein eigenes Ding machen',
    tocCustomCss: 'Mehr rausholen mit CSS',
    tocFonts: 'Eigene Fonts',
    tocPremium: 'Premium',
    whatIsHeading: 'Was ist All-Chat',
    whatIsBody:
      'All-Chat holt deinen Live-Chat von {twitch}, {youtube}, {kick}, {tiktok} und {discord} in ein einziges Overlay, das du in OBS einfügst. Jede Nachricht landet in einem Feed — dazu Events wie Subs, Bits, Raids, Super Chats und Geschenke — und 7TV-, BTTV- und FFZ-Emotes erscheinen automatisch. Keine Bots einzuladen, kein Chat-Widget zu verdrahten.',
    startHeading: 'Overlay live schalten',
    startSignIn: 'Melde dich auf {home} mit deinem Twitch-, YouTube- oder Kick-Konto an.',
    startSignInLinkText: 'allch.at',
    startCreate:
      'Erstelle im Dashboard ein {overlay} und verbinde die Plattformen, auf denen du streamst. Jede verbundene Plattform wird eine {source} auf diesem Overlay — mische so viele, wie du willst.',
    startCreateOverlayEmphasis: 'Overlay',
    startCreateSourceEmphasis: 'Chatquelle',
    startObs:
      'Kopiere die Browser-Quelle-URL des Overlays und füge sie als {browserSource} in OBS hinzu. Der Chat erscheint, sobald sich das Overlay verbindet.',
    startObsEmphasis: 'Browser-Quelle',
    startDemandDriven:
      'Quellen sind nachfragegesteuert: All-Chat hört zu, solange das Overlay offen ist, und fährt herunter, wenn nichts verbunden ist — du zahlst nie für inaktive Listener.',
    startMultipleOverlays:
      'Ein Konto kann mehrere Overlays betreiben, jedes mit eigener Mischung aus Chatquellen, eigenem Theme und eigener OBS-URL. Zum Beispiel: ein Overlay mit Twitch und TikTok für einen Gaming-Stream und ein nur-YouTube-Overlay für einen Podcast.',
    irlHeading: '24/7- und IRL-Streams',
    irlIntro:
      'Wer eine OBS-Instanz rund um die Uhr laufen lässt — ein übliches Setup für IRL-Streamer, die Absicherung gegen Verbindungsabbrüche wollen — braucht einen zusätzlichen Schritt, damit YouTube-Chat sauber funktioniert. Hänge {passiveParam} an die Browser-Quelle-URL deines Overlays an:',
    irlExplainer:
      'Ein {passive} Overlay rendert den Chat genau wie ein normales, bittet All-Chat aber nicht, von sich aus mit dem Aufzeichnen zu beginnen. Das ist wichtig, weil YouTube-Erkennung nach einer Stunde vergeblicher Suche aufgibt (so wird YouTube nie dauerhaft für einen Offline-Kanal belastet). Ein normales 24/7-Overlay würde beim Offline-Sein diesen Timeout auslösen und bis zum Going Live geparkt bleiben — ein passives Overlay startet diese Uhr nie, also bleibt nichts stecken.',
    irlExplainerEmphasis: 'passive',
    irlWhenLiveHeading: 'Wenn du live gehst',
    irlStepPassiveUrl: 'Lasse deine 24/7-OBS-Browser-Quelle auf der passiven URL.',
    irlStepOpenMonitor: 'Öffne den {monitor} deines Overlays (die Monitor-/Ansichtsseite).',
    irlStepOpenMonitorEmphasis: 'Chat-Monitor',
    irlStepRediscover:
      'Läuft der Chat nicht bereits, drücke {rediscover} auf dem Monitor — Aufzeichnung startet innerhalb von etwa einer Minute.',
    irlStepRediscoverEmphasis: 'Rediscover',
    irlStepKeepOpen:
      'Lasse den Chat-Monitor während deines Streams offen; das hält die Aufzeichnung für die Session am Laufen, und sie läuft wenige Minuten nach dem Schließen aus.',
    irlRefreshNote:
      'Ein bloßer Neuladevorgang der Browser-Quelle startet einen geparkten YouTube-Kanal {negation} — nutze die {rediscover}-Schaltfläche auf dem Monitor. Solange ein Kanal geparkt ist, zeigt sein Plattform-Punkt einen {paused}-Zustand (wartet darauf, dass du ihn auslöst), nicht einen roten Fehler.',
    irlRefreshNoteNegationEmphasis: 'nicht',
    irlRefreshNotePausedEmphasis: 'indigofarbener „pausiert“-Zustand',
    tiktokHeading: 'TikTok: Funktionsweise und Grenzen',
    tiktokNoApi:
      'Für LIVE-Chat gibt es keine öffentliche TikTok-API. Jedes Tool, das TikTok-LIVE-Chat zeigt, auch All-Chat, liest ihn so, wie TikToks eigener Webplayer es tut. Füge eine TikTok-Quelle per Benutzername hinzu, und der Chat erscheint, während du LIVE bist.',
    tiktokSigner:
      'Diese Verbindung muss signiert werden. All-Chat signiert sie mit seinem eigenen Open-Source-Signing-Dienst; kein fremder Sign-Server ist beteiligt. Als TikTok am 2026-09-09 auf browsergrade Sitzungsprüfungen umstellte, hat All-Chat die Änderung noch in derselben Woche nachgezogen. Design und Updates sind in {adr} dokumentiert.',
    tiktokAdrLinkText: 'ADR-0052 auf GitHub',
    tiktokFallback:
      'Premium-Räume haben einen zweiten Auslieferungspfad, genutzt wenn TikTok die Hauptverbindung ablehnt.',
    tiktokLimit:
      '{limit} TikTok ist die Plattform, bei der kurze Unterbrechungen am wahrscheinlichsten sind, wenn TikTok seinen Webplayer ändert. TikTok hat zudem keine Sende- oder Moderations-API, daher kann der Monitor dort nicht antworten oder moderieren.',
    tiktokLimitEmphasis: 'Die Grenze:',
    monitorHeading: 'Der Chat-Monitor',
    monitorIntro:
      'Öffne {monitorView} aus dem Overlay-Editor für einen Live-Kontrollraum, unabhängig vom OBS-Overlay. Er zeigt jede Nachricht in einem Panel mit einem Aktivitätsfeed daneben, und hier sendest du Nachrichten und moderierst.',
    monitorIntroEmphasis: 'Monitor View',
    monitorSend:
      '{send} als du selbst an Twitch, YouTube oder Kick direkt aus dem Monitor (TikTok und Discord haben keine Sende-API).',
    monitorSendEmphasis: 'Nachrichten senden',
    monitorRediscover:
      '{rediscover} erzwingt eine frische Suche nach deinem Live-Stream — nutze es, wenn YouTube-Chat nach einem Absturz oder Neustart stockt, oder um die Aufzeichnung für ein {passiveOverlay} zu starten, wenn du live gehst.',
    monitorRediscoverEmphasis: 'YouTube neu suchen',
    monitorPassiveOverlayEmphasis: 'passives 24/7-Overlay',
    monitorDisplay:
      '{display}-Einstellungen steuern, was du siehst (Avatare, Badges, Pronomen, Zeitstempel, Plattform-Icons, Moderationssteuerung). Das sind deine persönlichen Monitor-Einstellungen und ändern nichts am OBS-Overlay.',
    monitorDisplayEmphasis: 'Anzeige',
    monitorDockHeading: 'Den Monitor in OBS andocken',
    monitorDockIntro:
      'Der Monitor funktioniert auch als OBS-Custom-Browser-Dock, so dass der Chat neben deinem Audiopult sitzt statt in einem zweiten Fenster. Klicke im Overlay-Editor auf {copyDock}. Er kopiert den Monitor-Link im Dock-Modus:',
    monitorDockSteps:
      'Öffne in OBS {menu}, benenne das Dock, füge den Link ein und klicke auf Anwenden. Das Dock hält sein eigenes Browser-Profil, daher wirst du einmal im Panel zur Anmeldung aufgefordert. Streamlabs Desktop akzeptiert denselben Link als Custom-Browser-Dock. Um Chat und Aktivität gleichzeitig zu sehen, wähle im Dock den Tab „Beides“ oder füge zwei Docks hinzu, eines mit ?dock=chat und eines mit ?dock=activity am Ende, die jeweils nur diesen Bereich zeigen.',
    monitorDockMenu: 'Ansicht > Docks > Benutzerdefinierte Browser-Docks',
    moderationHeading: 'Deinen Chat moderieren',
    moderationBody:
      'Mit {controls} im Monitor aktiviert schwebst du über eine Nachricht, um sie zu {del}, oder öffnest einen Chatter, um ihn zu {timeout}, {ban} oder {unban} — ausgeführt auf der Quell-Plattform: Twitch, Kick und Discord führen Löschen, Timeout, Bannen und Entbannen aus; YouTube aus Timeout und Bann; TikTok hat keine Moderations-API.',
    moderationControlsEmphasis: 'Moderationssteuerung',
    moderationDeleteEmphasis: 'Löschen',
    moderationTimeoutEmphasis: 'Timeout',
    moderationBanEmphasis: 'Bannen',
    moderationUnbanEmphasis: 'Entbannen',
    moderationEnable:
      'Beim ersten Mal nutze {enable}, um die Zusatzberechtigungen zu erteilen (bei Discord lädst du den Bot neu ein). Moderation über dein Overlay ist eine {premium}-Funktion.',
    moderationEnableEmphasis: 'Moderation & Chat-Senden aktivieren',
    engagementHeading: 'Umfragen, Vorhersagen & Punkte',
    engagementIntro:
      'All-Chat hat eigene {polls}, {predictions} und {points}, die über jede verbundene Plattform hinweg funktionieren, nicht nur Twitch.',
    engagementPollsEmphasis: 'Umfragen',
    engagementPredictionsEmphasis: 'Vorhersagen',
    engagementPointsEmphasis: 'Zuschauer-Punkte',
    engagementSetup:
      'Aktiviere sie im {section}-Bereich des Editors: {enablePoints}, setze deinen {pointsName} und wähle, wie Punkte verdient werden (pro Bit, pro Sub, pro geschenktem Sub usw.).',
    engagementSectionEmphasis: 'Engagement',
    engagementEnablePointsEmphasis: 'Zuschauer-Punkte aktivieren',
    engagementPointsNameEmphasis: 'Points-Name',
    engagementRun:
      'Starte Runden live aus dem {section}-Panel des Monitors: {startPoll} / {closePoll} sowie {startPrediction} / {lockWagers} / Auszahlung oder {cancelRefund}.',
    engagementStartPollEmphasis: 'Umfrage starten',
    engagementClosePollEmphasis: 'Umfrage beenden',
    engagementStartPredictionEmphasis: 'Vorhersage starten',
    engagementLockWagersEmphasis: 'Einsätze sperren',
    engagementCancelRefundEmphasis: 'Abbrechen & erstatten',
    engagementJoin:
      'Zuschauer steigen über den Chat ein (z. B. {voteCommand}, {predictCommand}) oder auf der {participationPage}.',
    engagementParticipationPageEmphasis: 'Zuschauer-Beteiligungsseite',
    engagementWidgets:
      'Füge {pollWidget} und {predictionWidget} als eigene Browser-Quellen hinzu, damit die Ergebnisse auf dem Stream erscheinen — kopiere ihre URLs aus dem Engagement-Bereich.',
    engagementPollWidgetEmphasis: 'OBS-Umfrage-Widget',
    engagementPredictionWidgetEmphasis: 'OBS-Vorhersage-Widget',
    eventsHeading: 'Events & Abspann',
    eventsBody:
      'Dein Overlay zeigt auch {events} — Subs, Resubs, Geschenk-Subs, Bits/Cheers, Raids, Super Chats, Mitgliedschaften und Follows. Nutze {eventSettings} im Editor, um genau zu wählen, welche Events erscheinen, pro Plattform.',
    eventsEmphasis: 'Events',
    eventSettingsEmphasis: 'Event-Einstellungen',
    creditsBody:
      'Zum Ende eines Streams öffne {credits}, um einen {creditRoll} einzurichten — einen scrollenden Dank an deine Top-Abonnenten, Geschenkgeber, Cheerer, Raider und neuen Follower. Er ist eine eigene Browser-Quelle ({copyUrl}).',
    creditsEmphasis: 'Credits',
    creditRollEmphasis: 'Abspann',
    creditsCopyUrlEmphasis: 'Credits-OBS-URL kopieren',
    sharingHeading: 'Overlay teilen',
    sharingBody:
      '{share} lässt einen anderen Streamer den Chat deines Overlays in sein eigenes ziehen — praktisch für Collabs und Raids. Sende eine Anfrage an ihren Twitch-Benutzernamen; sobald sie sie annehmen, erscheint dein Overlay in ihrem Editor unter {sharedOverlays}, um es als Quelle hinzuzufügen, und jeder von euch kann es später zurückziehen. Teilen ist eine {premium}-Funktion.',
    sharingShareEmphasis: 'Overlay teilen',
    sharingSharedOverlaysEmphasis: 'Geteilte Overlays',
    themesHeading: 'Ein Theme wählen',
    themesIntro:
      'All-Chat kommt mit {count} — von Modern Dark und Minimal bis Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk und mehr. Ein Theme anzuwenden erfordert einen Klick und braucht {noCss}.',
    themesCountEmphasis: '16 integrierte Themes',
    themesNoCssEmphasis: 'überhaupt kein CSS',
    themesStepOpen: 'Öffne dein Overlay im Dashboard, um es zu bearbeiten.',
    themesStepApply:
      'Im {theme}-Bereich stöbere durch die Themes und wende eines an. Die Vorschau aktualisiert sich sofort.',
    themesStepApplyEmphasis: 'Theme',
    themesStepSave:
      'Speichern. Deine OBS-Browser-Quelle übernimmt den neuen Look beim nächsten Neuladen.',
    themesPreview: 'Du kannst jedes Theme live auf der {home} ansehen, bevor du dich anmeldest.',
    themesPreviewLinkText: 'Startseite',
    customizeHeading: 'Dein eigenes Ding machen',
    customizeIntro:
      'Ein Theme ist ein Ausgangspunkt — meistens erreichst du genau den Look, den du willst{noCode}. Im {appearance}-Panel des Overlay-Editors kannst du mit Schiebereglern, Schaltern und Farbwählern einstellen:',
    customizeNoCodeEmphasis: ' ohne Code zu schreiben',
    customizeAppearanceEmphasis: 'Erscheinungsbild',
    customizeFont: 'Schriftfamilie, Textgröße, Stärke, Zeilenhöhe und Zeichenabstand',
    customizeSpacing: 'Abstand, Innenabstand und Ecken der Nachrichtenblasen',
    customizeAvatar: 'Avatar- und Badge-Größe',
    customizeVisibility:
      'Was gezeigt oder versteckt wird — Avatare, Badges, Zeitstempel, Benutzernamen',
    customizeColors: 'Farben, inklusive einer Namensfarbe pro Plattform',
    customizeEvents: 'Wie Events wie Subs, Raids und Super Chats erscheinen',
    customizeOutro:
      'Starte vom Theme, das deinem Ziel am nächsten kommt, und justiere diese Einstellungen, bis es sich nach deinem anfühlt. Brauchst du etwas, das das Panel nicht abdeckt, steige auf CSS unten um.',
    cssHeading: 'Mehr rausholen mit CSS',
    cssIntro:
      'Für volle Kontrolle aktiviere {customCss} im {expert}-Bereich des Overlay-Editors und schreibe eigene Styles. Ein paar Dinge, die du wissen solltest:',
    cssCustomCssEmphasis: 'Eigenes CSS',
    cssExpertEmphasis: 'Experte',
    cssScope: 'Dein CSS betrifft nur dein Overlay — nichts anderes.',
    cssOrder:
      'Es lädt {after} dem Theme, so dass du von einem integrierten Theme startest und nur die Teile überschreibst, die du willst.',
    cssOrderEmphasis: 'nach',
    cssPreview:
      'Direkt neben dem Editor gibt es eine Live-Vorschau, damit du Änderungen beim Tippen siehst.',
    cssVarsHeading: 'Quick Wins: Style-Variablen',
    cssVarsIntro:
      'Der einfachste Hebel braucht kein Wissen über Klassennamen. Setze eine dieser Variablen auf {root} und das Overlay übernimmt sie:',
    cssVarsForced:
      'E Variablesgruppe ist besonders: Font-Weight, Blasen-Ecken, Zeilenhöhe, Zeichenabstand, Nachrichtenfarbe und Schriftfamilie werden von der eigenen Stylinglogik des Overlays aus diesen Variablen gelesen, so dass ein hier gesetzter Wert auch den Schieberegler des Erscheinungsbild-Panels schlägt.',
    cssVarsColumnVariable: 'Variable',
    cssVarsColumnDefault: 'Standard',
    cssVarsColumnEffect: 'Was sie ändert',
    cssHooksHeading: 'Feinere Kontrolle: die Chat-Teile ansteuern',
    cssHooksIntro:
      'Für alles, was die Variablen nicht abdecken, style die Elemente des Overlays direkt. Diese Klassennamen und Datenattribute sind stabil und sicher anzusprechen:',
    cssHooksColumnSelector: 'Selektor',
    cssHooksColumnKind: 'Art',
    cssHooksColumnTargets: 'Ziel',
    cssFeedAnchor:
      '{feedAnchor} (Nachrichten-Einstellungen) entscheidet, an welcher {edge} der Feed liegt — verankere ihn unten und neue Nachrichten schieben die älteren nach oben, wobei der leere Platz sich oben sammelt. {invertOrder} ist eine separate Einstellung für eine separate Achse: Sie ändert nur, welches {endOfList} am neuesten ist. Alle vier Kombinationen funktionieren.',
    cssFeedAnchorEmphasis: 'Feed-Anker',
    cssFeedAnchorEdgeEmphasis: 'Kante',
    cssInvertOrderEmphasis: 'Nachrichtenreihenfolge umkehren',
    cssEndOfListEmphasis: 'Ende der Liste',
    cssExampleCaption: 'Beispiel — gib jeder Plattform ihren eigenen Akzent-Streifen:',
    cssCallout:
      'Willst du fertige Beispiele? Jedes integrierte Theme ist nur eine CSS-Datei, die du auf {github} lesen und ausleihen kannst. Hast du schon ein Theme von einem anderen Tool, oder brauchst du Hilfe beim Schreiben? Frag in unserem {discord}, wir helfen gerne.',
    cssCalloutGithubLinkText: 'GitHub',
    fontsHeading: 'Eigene Fonts',
    fontsIntro:
      'Du kannst einen Google-Font mit einer normalen {importRule} nachladen und überall im CSS nutzen. Zum Schutz deiner Zuschauer werden Fonts über All-Chat ausgeliefert statt direkt von Google, so dass nur diese Familien verfügbar sind:',
    fontsFamilies:
      'Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito, Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech Mono, Source Code Pro, Space Grotesk, VT323.',
    fontsOutro:
      'Eine Familie, die nicht auf der Liste steht, lädt einfach nicht — wähle eine andere oder lasse den Standard.',
    premiumHeading: 'Premium',
    premiumIntro:
      'All-Chat ist frei und Open Source. Ein Premium-Abo — via Patreon, verbunden unter {settings} — schaltet frei:',
    premiumSettingsEmphasis: 'Einstellungen → Premium',
    premiumModeration: '{label} — im Chat-Monitor löschen, Timeout erteilen und bannen.',
    premiumModerationEmphasis: 'Von deinem Overlay aus moderieren',
    premiumTts: '{label} — Premium-TTS-Stimmen (grundlegende Browser-TTS ist gratis).',
    premiumTtsEmphasis: 'ElevenLabs-Text-zu-Sprache',
    premiumYoutube:
      '{label} — wähle, welchem Stream auf Mehrfach-Kanälen du folgst (die gratis Standardwahl folgt dem zuerst gefundenen).',
    premiumYoutubeEmphasis: 'YouTube-Streamauswahl',
    premiumSharedChat: '{label} — teile dein Overlay mit anderen Streamern.',
    premiumSharedChatEmphasis: 'Geteilter Chat',
    premiumFlairs: '{label} — animierte Gradient-Namensfarben.',
    premiumFlairsEmphasis: 'Zuschauer-Flairs',
    footerCopyright: '© {year} All-Chat',
    footerApiLink: 'Entwickler-API',
    footerPrivacyLink: 'Datenschutzerklärung',
    footerTermsLink: 'Nutzungsbedingungen',
  },
  guideCssVars: {
    chatFontSize: 'Textgröße der Nachricht.',
    chatFontWeight: 'Schriftstärke des Nachrichtentexts.',
    chatFontFamily: 'Schrift des Nachrichtentexts.',
    chatUsernameFontWeight: 'Schriftstärke des Benutzernamens.',
    chatMessageColor: 'Farbe des Nachrichtentexts.',
    chatMessageGap: 'Vertikaler Abstand zwischen Nachrichten.',
    chatAvatarGap: 'Abstand zwischen Avatar und Nachrichtentext.',
    chatBubbleBorderRadius: 'Rundheit der Nachrichtenblase.',
    chatBubblePadding: 'Innenabstand innerhalb jeder Nachricht.',
    chatAvatarSize: 'Avatar-Breite und -Höhe.',
    chatUsernameFontSize: 'Größe des Benutzernamens.',
    chatEmoteScale: 'Emote-Größenfaktor.',
    chatShowAvatars: 'Auf none setzen, um Avatare zu verstecken.',
    chatShowBadges: 'Auf none setzen, um Badges zu verstecken.',
    chatShowTimestamps: 'Auf none setzen, um Zeitstempel zu verstecken.',
  },
  guideCssHooks: {
    overlayLiveBody: 'Der Container der gesamten Nachrichtenliste.',
    chatMessage: 'Eine Chat-Nachrichtenblase.',
    chatUsername: 'Der Verfassername.',
    platformBadge: 'Das Plattform-Tag/Icon neben dem Namen.',
    eventMessage: 'Ein Sub-/Raid-/Super-Chat-/Geschenk-Alert.',
    dataPlatformTwitch: 'Nachrichten von einer Plattform (twitch, youtube, kick, tiktok, discord).',
    dataUsername: 'Nachrichten von einem bestimmten Benutzer.',
    dataFeedAnchorTopBottom:
      'Der Overlay-Wrapper, mit der Feed-Anchor-Einstellung. Nutze es zum Anpassen; überschreibe nicht die flex-direction des Wrappers oder das margin-top der Liste, die den Feed bewegen.',
    dataFeedOrderNewestLastNewestFirst:
      'Der Overlay-Wrapper, mit der Nachrichtenreihenfolge-umkehren-Einstellung. Er dreht auch --msg-enter-dir und --msg-enter-origin, so dass Eintrittsanimationen von der Seite kommen, auf der die neueste Nachricht landet.',
  },
  themeContrast: {
    heading: 'Theme-Kontrast-Harness',
    intro:
      'Nur für die Entwicklung. Rendert jedes mitgelieferte Theme für die WCAG-Prüfung des Nachrichtentexts. {count} Themes.',
  },
}
