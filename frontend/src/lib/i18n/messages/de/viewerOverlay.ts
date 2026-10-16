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

export const viewerOverlay: LocaleNamespace<'viewerOverlay'> = {
  engagement: {
    pollHeading: 'Umfrage',
    predictionHeading: 'Vorhersage',
    twitchSourceBadge: 'Twitch',

    labelListEntry: '{noun} {index}',
    labelListRemove: 'Entfernen',
    labelListAdd: '{noun} hinzufügen',

    pollQuestionPlaceholder: 'Frage',
    pollQuestionLabel: 'Umfragefrage',
    pollOptionNoun: 'Option',
    pollAllowChange: 'Stimmabgaben ändern erlauben',
    pollAutoCloseAfter: 'Automatisch schließen nach',
    secondsSuffix: 's',
    pollStart: 'Umfrage starten',

    pollVotes: '{total} Stimmen',
    pollAutoCloses: ' · schließt automatisch {time}',
    pollNew: 'Neue Umfrage',
    pollClose: 'Umfrage schließen',
    pollMirroredNote:
      'Von Twitch gespiegelt – Zuschauer stimmen in der Twitch-Oberfläche oder im Chat ab',
    pollParticipateHint:
      'Zuschauer stimmen auf der {link} oder im Chat ab ({voteCommand} oder einfach {shortCommand})',
    participateLink: 'Teilnahmeseite',

    predictionTitlePlaceholder: 'Titel (z. B. Gewinnen wir diese Runde?)',
    predictionTitleLabel: 'Vorhersage-Titel',
    predictionOutcomeNoun: 'Ergebnis',
    predictionAutoLockAfter: 'Wetten automatisch sperren nach',
    predictionStart: 'Vorhersage starten',
    predictionParticipateHint:
      'Zuschauer setzen auf der {link} (dort sehen sie ihren Punktestand) – oder im Chat: {predictCommand}',

    predictionPointsWagered: '{total} Punkte gesetzt',
    predictionAutoLocks: ' · sperrt automatisch {time}',
    predictionOutcomeTally: '{points} P · {entrants} Teilnehmer',
    winningOutcome: 'Gewinnendes Ergebnis',
    winningOutcomeChoice: 'Gewinnendes Ergebnis: {label}',
    predictionLock: 'Wetten sperren',
    predictionNew: 'Neue Vorhersage',
    predictionLockedNote:
      'Wähle das gewinnende Ergebnis und zahle dann aus. Auszahlungen sind endgültig.',
    predictionMirroredNote: 'Von Twitch gespiegelt — läuft über Twitch-Kanalpunkte',

    predictionResolve: 'Auswerten',
    predictionPayOut: '"{label}" auszahlen',
    predictionPayOutConfirm: '"{label}" auszahlen — endgültig?',
    predictionResolveDisabledTitle: 'Wähle zuerst das gewinnende Ergebnis',

    predictionCancel: 'Abbrechen & erstatten',
    predictionCancelTitle: 'Abbrechen und alle Wetten erstatten',
    predictionCancelConfirm: 'Wirklich alle Wetten erstatten?',

    mirrorNote:
      'Spiegle native Twitch-Umfragen und -Vorhersagen auf deine Overlays (schreibgeschützt). Opt-in; greift nach der nächsten Kanal-Synchronisierung (Stream-Restart oder erneutes Hinzufügen der Quelle).',
    mirrorEnable: 'Twitch-Spiegelung aktivieren',

    pollIncompleteToast: 'Eine Umfrage braucht eine Frage und mindestens 2 Optionen',
    pollStartedToast: 'Umfrage gestartet',
    pollStartFailed: 'Umfrage konnte nicht gestartet werden',
    pollClosedToast: 'Umfrage geschlossen',
    pollCloseFailed: 'Umfrage konnte nicht geschlossen werden',
    predictionIncompleteToast: 'Eine Vorhersage braucht einen Titel und mindestens 2 Ergebnisse',
    predictionStartedToast: 'Vorhersage gestartet',
    predictionStartFailed: 'Vorhersage konnte nicht gestartet werden',
    predictionLockedToast: 'Vorhersage gesperrt \u2014 Wetten sind eingefroren',
    predictionLockFailed: 'Vorhersage konnte nicht gesperrt werden',
    pickWinnerToast: 'Wähle zuerst das gewinnende Ergebnis',
    predictionNoLongerLockedToast:
      'Die Vorhersage ist nicht mehr gesperrt \u2014 lade neu und versuche es erneut',
    predictionResolvedToast: 'Vorhersage ausgewertet \u2014 Gewinner ausgezahlt',
    predictionResolveFailed: 'Vorhersage konnte nicht ausgewertet werden',
    predictionCanceledToast: 'Vorhersage abgebrochen \u2014 alle Wetten erstattet',
    nothingToCancelToast: 'Nichts abzubrechen \u2014 die Vorhersage ist bereits {state}',
    predictionCancelFailed: 'Vorhersage konnte nicht abgebrochen werden',
    twitchConsentFailedToast:
      'Twitch-Einwilligung konnte nicht gestartet werden. Bitte erneut versuchen.',
  },

  participate: {
    loading: 'Wird geladen…',
    loginHeading: 'Mach mit',
    loginBlurb: 'Melde dich mit deinem Plattform-Konto an, um abzustimmen und zu wetten.',
    loginWith: 'Weiter mit {platform}',
    noWebLoginNote:
      'Du schaust auf TikTok oder Discord? Mach mit den Chat-Befehlen auf dem Bildschirm mit — eine Web-Anmeldung gibt es für diese Plattformen noch nicht.',

    heading: 'Teilnehmen',
    balanceLabel: 'Guthaben: {balance} {pointsName}',
    balance: '{balance} {pointsName}',
    defaultPointsName: 'Punkte',

    settledBanner:
      'Deine Vorhersage auf „{outcome}“ ist ausgegangen — du hast {amount} {pointsName} gesetzt. Prüfe dein Guthaben oben.',

    pollNativeNote: 'Diese Umfrage läuft auf Twitch — vote im Twitch-Chat oder in der Twitch-App.',
    pollVoteNativeTitle: 'Im Twitch-Chat abstimmen',
    working: 'Wird ausgeführt…',
    yourVote: '(deine Stimme)',
    pollOptionTally: '{pct}% ({votes})',

    predictionLocked: 'gesperrt',
    predictionNativeNote: 'Diese Vorhersage läuft auf Twitch-Kanalpunkten.',
    youHave: 'Du hast {balance} {pointsName}',
    maxWager: 'Max',
    noPointsYet:
      'Du hast noch keine {pointsName}. Verdiene sie, indem du diese Seite offen hältst und den Stream unterstützt (Subs, Bits, Spenden, Geschenke), und komme dann zurück zum Wetten.',
    wagerAmountLabel: 'Zu setzender Betrag in {pointsName}',
    wagerAmountPlaceholder: 'Zu setzender Betrag ({pointsName})',
    yourWager: ' · deine Wette: {amount}',
    outcomeTally: '{points} · {pct}%',
    alreadyWagered: 'Du hast deine Wette für diese Runde schon abgegeben.',
    nothingActive: 'Gerade läuft keine Umfrage und keine Vorhersage. Bleib dran!',

    wagerNativeTitle: 'Läuft auf Twitch-Kanalpunkten',
    wagerAlreadyTitle: 'Du hast in dieser Runde schon gewettet',
    wagerClosedTitle: 'Wetten sind geschlossen',

    lockedAnnouncement: 'Vorhersage gesperrt — Wetten sind geschlossen.',

    rejectNotFound: 'Diese Vorhersage ist nicht mehr verfügbar.',
    rejectNotActive: 'Wetten sind für diese Runde geschlossen.',
    rejectBadOutcome: 'Dieses Ergebnis ist ungültig.',
    rejectAlreadyWagered: 'Du hast in dieser Runde schon gewettet.',
    rejectInsufficient: 'Nicht genug {pointsName}. Du hast {balance}.',
    rejectNative: 'Diese Vorhersage läuft auf Twitch-Kanalpunkten.',
    insufficientLocal: 'Nicht genug {pointsName} — du hast {balance}.',

    loginFailed: 'Anmeldung konnte nicht gestartet werden. Bitte erneut versuchen.',
    voteFailed: 'Abstimmung fehlgeschlagen',
    wagerFailed: 'Wette fehlgeschlagen',
    wagerNeedsAmount: 'Gib einen positiven Betrag ein.',
  },

  pollWidget: {
    finalBadge: 'Endstand',
    finalResults: 'Endergebnis',
    winnerPill: 'Gewinner',
    tiePill: 'Gleichstand',
    optionTally: '{pct}% ({votes})',
    remaining: '{clock} verbleiben',
  },

  predictionWidget: {
    stateLocked: '🔒 GESPERRT',
    stateResolved: '🏆 AUSGEWERTET',
    stateOpen: 'OFFEN',
    winnerPill: 'Gewinner',
    outcomeTally: '{points} P · {pct}%',
    pool: '{points} P gesetzt · {players} Spieler',
  },

  credits: {
    topMembers: 'Top-Mitglieder',
    topCoinChests: 'Top-Münztruhen',
    topSharers: 'Top-Teiler',
    topLikers: 'Top-Liker',
    valueBitsOne: '{count} Bit',
    valueBitsMany: '{count} Bits',
    valueMonthsOne: '{count} Monat',
    valueMonthsMany: '{count} Monate',
    valueViewersOne: '{count} Zuschauer',
    valueViewersMany: '{count} Zuschauer',
    valuePointsOne: '{count} Punkt',
    valuePointsMany: '{count} Punkte',
    valuePointsNamed: '{count} {name}',
    valueGiftsOne: '{count} Geschenk',
    valueGiftsMany: '{count} Geschenke',
    valueDiamondsOne: '{count} Diamant',
    valueDiamondsMany: '{count} Diamanten',
    valueCoinsOne: '{count} Münze',
    valueCoinsMany: '{count} Münzen',
    valueLikesOne: '{count} Like',
    valueLikesMany: '{count} Likes',
    valueSharesOne: '{count} Share',
    valueSharesMany: '{count} Shares',
    loading: 'Abspann wird geladen...',
    loadFailed: 'Abspann konnte nicht geladen werden',
    errorHeading: 'Abspann konnte nicht geladen werden',
    errorHint: 'Stelle sicher, dass du eine aktive Streaming-Session hast',
    empty: 'Keine Abspann-Daten verfügbar',

    heading: '🎬 Stream-Abspann',
    subheading: 'Danke an alle, die den Stream unterstützt haben!',
    session: 'Session: {date}',
    duration: 'Dauer: {duration}',

    durationHourOne: '{hours} Stunde',
    durationHourMany: '{hours} Stunden',
    durationMinuteOne: '{minutes} Minute',
    durationMinuteMany: '{minutes} Minuten',
    durationHoursAndMinutes: '{hours} {minutes}',
    durationJustStarted: 'gerade gestartet',

    topSubscribers: 'Top-Abonnenten',
    topGifters: 'Top-Geschenkespender',
    topCheerers: 'Top-Cheerer',
    topChannelPoints: 'Top-Kanalpunkte',
    topRaiders: 'Top-Raider',
    topSuperChats: 'Top-Super-Chats',
    newFollowers: 'Neue Follower',

    nowPlaying: 'Gerade läuft',
    clipViews: '{views} Aufrufe',
    clipCounter: 'Clip {index}/{total}',

    thanks: 'Danke fürs Zuschauen! ❤️',
    seeYou: 'Bis zum nächsten Stream!',
  },

  activity: {
    heading: 'Aktivität & Events',
    empty: 'Noch keine Events.',
    modBadge: 'Mod',
    someUser: 'ein Nutzer',

    deleted: 'Nachricht gelöscht',
    deletedBy: 'Nachricht von {moderator} gelöscht',
    cleared: 'Chat geleert',
    clearedBy: 'Chat von {moderator} geleert',
    banned: '{user} gebannt',
    bannedBy: '{user} von {moderator} gebannt',
    timedOut: '{user} in Timeout geschickt',
    timedOutFor: '{user} für {seconds}s in Timeout geschickt',
    timedOutBy: '{user} von {moderator} in Timeout geschickt',
    timedOutForBy: '{user} für {seconds}s von {moderator} in Timeout geschickt',
    automodHeld: 'AutoMod hat eine Nachricht von {user} zurückgehalten',
    automodHeldCategory: 'AutoMod hat eine Nachricht von {user} zurückgehalten ({category})',
    automodResolved: 'AutoMod-Halten {resolution}',
    automodResolvedBy: 'AutoMod-Halten {resolution} durch {moderator}',
    automodHeldBadge: 'gehalten',
  },

  chatPanel: {
    heading: 'Chat',
    empty: 'Noch keine Chat-Nachrichten.',
    filteredBy: 'Zeige nur Nachrichten von {user}',
    showAll: 'Gesamten Chat anzeigen',
    filteredEmpty: 'Noch keine Nachrichten von {user}.',
    filteredCount: '{shown} von {total}',
    sharedBadge: 'geteilt',
  },

  observability: {
    eventTiktokSubscriptions: 'TikTok-Subs',
    eventTiktokSuperFans: 'TikTok Super Fans',
    eventTiktokQuestions: 'TikTok-Fragen',
    sources: 'Quellen ({count})',
    configuredEvents: 'Konfigurierte Events',
    emotes: 'Emotes',
    filters: 'Filter',

    noSources: 'Keine Quellen konfiguriert.',
    sourceLive: 'live',
    sourceIdle: 'inaktiv',

    eventsUnavailable:
      'Event-Konfiguration nicht verfügbar; Events erscheinen hier, sobald sie eintreffen.',
    sevenTvSet: '7TV-Set',
    sevenTvDefault: 'Standard pro Quelle',

    bannedWords: 'Gesperrte Wörter',
    bannedUsers: 'Gesperrte Nutzer',
    minLength: 'Mindestlänge',
    hideCommands: 'Befehle ausblenden',
    sayHiFilter: 'Say-hi-Filter',
    yes: 'ja',
    no: 'nein',
    filtersNote: 'Filter dienen nur zur Übersicht; diese Ansicht zeigt alle Nachrichten.',

    eventTwitchSubs: 'Twitch-Subs',
    eventTwitchResubs: 'Twitch-Resubs',
    eventTwitchGiftSubs: 'Twitch-Geschenk-Subs',
    eventTwitchBits: 'Twitch-Bits',
    eventTwitchRaids: 'Twitch-Raids',
    eventTwitchChannelPoints: 'Kanalpunkte',
    eventTwitchFollows: 'Twitch-Follower',
    eventTwitchWatchStreaks: 'Watch Streaks',
    eventYoutubeSuperChat: 'YouTube Super Chat',
    eventYoutubeSuperSticker: 'Super Sticker',
    eventYoutubeMembers: 'YouTube-Mitglieder',
    eventYoutubeMemberMilestones: 'Mitglieder-Meilensteine',
    eventYoutubeMemberGifts: 'Mitglieder-Geschenke',
    eventYoutubeSubscribers: 'YouTube-Abonnenten',
    eventKickSubs: 'Kick-Subs',
    eventKickGifts: 'Kick-Geschenke',
    eventTiktokLikes: 'TikTok-Likes',
    eventTiktokGifts: 'TikTok-Geschenke',
    eventTiktokFollows: 'TikTok-Follower',
    eventTiktokShares: 'TikTok-Shares',
    eventTiktokTreasureChests: 'TikTok-Münzkisten',
    eventTokenWarnings: 'Token-Warnungen',
  },

  viewSettings: {
    buttonLabel: 'Anzeigeeinstellungen',
    buttonText: 'Anzeige',
    heading: 'Ansichtseinstellungen',
    chatOrderHeading: 'Chat-Reihenfolge',
    activitySoundHeading: 'Aktivitäts-Sound',

    showAvatars: 'Avatare',
    showBadges: 'Badges',
    showPronouns: 'Pronomen',
    showTimestamps: 'Zeitstempel',
    showPlatformGlyph: 'Plattform-Symbol',
    showModeration: 'Moderations-Steuerung',

    newestFirst: 'Neueste Nachrichten zuerst',
    newestFirstNote:
      'Setzt die neueste Nachricht an den Anfang des Chat-Panels, damit du den Chat lesen kannst, ohne nach unten zu schauen. Betrifft nur diesen Browser.',

    activitySoundEnabled: 'Sound bei neuer Aktivität',
    soundPresetLabel: 'Sound',
    volume: 'Lautstärke',
    testSound: 'Sound testen',
    activitySoundNote:
      'Spielt nur hier in diesem Browser ab, damit du leicht zu überhörende Aktivitäten mitbekommst, z. B. Kanalpunkte-Einlösungen oder eine TikTok-Rose. Getrennt von den Benachrichtigungs-Sounds deiner Overlays im Stream.',
  },

  chatSend: {
    formLabel: 'Chat-Nachricht senden',
    targetGroupLabel: 'Senden an',
    sendToPlatform: 'An {platform} senden',
    enableSendingFor: 'Senden für {platform} aktivieren',
    allLabel: 'An alle Plattformen senden',
    allText: 'Alle',
    messageLabel: 'Chat-Nachricht',
    placeholderAll: 'An alle Plattformen senden…',
    placeholderOne: 'Nachricht senden…',
    send: 'Senden',

    enableSending: 'Senden aktivieren',
    reconnect: 'Neu verbinden',
    enablePlatform: '{platform} aktivieren',
    reconnectPlatform: '{platform} neu verbinden',

    sendFailed: 'Senden fehlgeschlagen. Bitte erneut versuchen.',
    missingScope: 'Senden ist noch nicht aktiviert.',
    missingScopeFor: 'Senden ist für {platform} noch nicht aktiviert.',
    reauthRequired: 'Deine {platform}-Anmeldung ist abgelaufen. Bitte verbinde neu.',
    reauthRequiredGeneric: 'Deine Plattform-Anmeldung ist abgelaufen. Bitte verbinde neu.',
    rateLimitedRetry: 'Rate-Limit erreicht — versuche es in {seconds}s erneut.',
    rateLimited: 'Rate-Limit erreicht — bitte etwas langsamer.',
    streamOffline: 'Dieser Kanal ist gerade nicht live.',

    sent: 'Gesendet \u2713',
    resultOk: '{platform} \u2713',
    resultFailed: '{platform} \u2717',
    resultFailedWhy: '{platform} \u2717 {why}',

    reasonReauthRequired: 'neu verbinden',
    reasonMissingScope: 'gesperrt',
    reasonStreamOffline: 'offline',
    reasonQuotaExhausted: 'Kontingent',
    reasonSendFailed: 'fehlgeschlagen',
  },

  moderationControls: {
    menuLabel: 'Nutzer moderieren',
    timeout: 'Timeout',
    ban: 'Nutzer bannen',
    unban: 'Nutzer entbannen',
    deleteMessage: 'Nachricht löschen',

    noModerationApi: '{platform} hat keine Moderations-API',
    unavailable: 'Moderation ist für diese Quelle nicht verfügbar',
    missingScope: 'Erteile Moderations-Berechtigungen, um Mod-Aktionen zu aktivieren',
    needsDiscordLink: 'Verknüpfe dein Discord-Konto, um hier zu moderieren',
    ownerChannelUnverified:
      'Das Discord-Konto dieses Streamers ist nicht verbunden, daher kann hier nichts moderiert werden',
    botMissingPermission:
      'Dem All-Chat-Bot wurde diese Discord-Berechtigung nicht gegeben — bitte den Streamer, ihn neu einzuladen',
  },

  layoutPicker: {
    groupLabel: 'Panel-Layout',
    chatLeft: 'Chat links, Events rechts',
    chatRight: 'Chat rechts, Events links',
    chatTop: 'Chat oben, Events darunter',
    eventsTop: 'Events oben, Chat darunter',
  },

  platformGlyph: {
    groupLabel: 'Plattformen: {platforms}',
  },

  monitor: {
    details: 'Details',
    engagement: 'Engagement',
    engagementTitle: 'Umfragen und Vorhersagen für dieses Overlay durchführen',
    rediscoverYouTube: 'YouTube neu entdecken',
    rediscoverYouTubeTitle:
      'Löst eine erneute Stream-Erkennung auf YouTube aus \u2014 nutze es, wenn der Chat nach einem Stream-Absturz oder -Restart stehen blieb',
    obsOverlay: 'OBS-Overlay',

    stillReconnecting:
      'Verbindung wird noch wiederhergestellt \u2014 das geht von selbst, und zwischenzeitlich gesendete Nachrichten werden abgespielt, sobald die Verbindung zurück ist. Nur das Schließen dieser Seite verliert sie.',
    replayTruncated:
      'Einige ältere Nachrichten könnten fehlen \u2014 die Unterbrechung dauerte länger als der Replay-Puffer, daher ließ sich der älteste Teil der Lücke nicht wiederherstellen.',

    noRole:
      'Du kannst diesen Monitor ansehen, aber hier nicht moderieren \u2014 Moderation ist deaktiviert.',

    featureGatedOwner: 'Chat-Moderation ist eine Premium-Funktion.',
    featureGatedUpgrade: 'Upgrade holen, um direkt aus deinem Overlay zu moderieren',
    featureGatedModerator:
      'Der Plan dieses Streamers enthält momentan keine Moderation, daher sind deine Aktionen nicht verfügbar, bis er ihn verlängert.',

    needsConsent: 'Verbinde dein eigenes {platform}-Konto, um zu moderieren.',
    needsConsentChannel: 'Verbinde dein eigenes {platform}-Konto, um {channel} zu moderieren.',
    connectPlatform: '{platform} verbinden',

    needsDiscordLink:
      'Verknüpfe dein Discord-Konto, um Discord hier zu moderieren \u2014 All-Chat prüft deine eigenen Server-Berechtigungen, bevor etwas passiert.',
    linkDiscord: 'Discord verknüpfen',

    missingScope:
      'Erteile Moderations-Berechtigungen, um Mod-Aktionen für {platform} zu aktivieren.',
    missingScopeChannel:
      'Erteile Moderations-Berechtigungen, um Mod-Aktionen für {platform} ({channel}) zu aktivieren.',
    missingScopeDiscord:
      'Lade den Bot mit Moderations-Berechtigungen neu ein, um Mod-Aktionen für {platform} zu aktivieren.',
    missingScopeDiscordChannel:
      'Lade den Bot mit Moderations-Berechtigungen neu ein, um Mod-Aktionen für {platform} ({channel}) zu aktivieren.',
    reinviteBot: 'Bot neu einladen',
    enableModeration: 'Moderation & Chat-Senden aktivieren',
    comingSoonFor: '(kommt bald für {platform})',

    modLogOptIn:
      'Zeige Twitch-Moderationsaktionen und AutoMod-Halte in diesem Aktivitäts-Feed. Twitch verlangt eine AutoMod-\u201eVerwalten\u201c-Berechtigung, um uns gehaltene Nachrichten überhaupt zu schicken \u2014 All-Chat liest sie nur; Freigabe-/Ablehnen-Buttons gibt es noch nicht.',
    enableModLog: 'Moderations- & AutoMod-Events anzeigen',

    reauthOwner:
      'Deine {platform}-Moderationsberechtigung ist abgelaufen oder wurde nie erteilt \u2014 autorisiere neu, um weiter aus deinem Overlay zu moderieren.',
    reauthModerator:
      'Deine {platform}-Moderationsberechtigung ist abgelaufen oder wurde nie erteilt \u2014 autorisiere neu, um hier weiter zu moderieren.',
    reconnectPlatform: '{platform} neu verbinden',
    reauthorizeModeration: 'Moderation & Chat-Senden neu autorisieren',

    consentStartFailed:
      'Moderations-Einrichtung konnte nicht gestartet werden. Bitte erneut versuchen.',
    twitchConsentStartFailed:
      'Twitch-Einwilligung konnte nicht gestartet werden. Bitte erneut versuchen.',
    modConnectUnavailable:
      '{platform} verbinden ist noch nicht verfügbar. Bitte den Streamer vorerst, dort zu moderieren.',
    discordLinkUnavailable:
      'Discord verknüpfen ist gerade nicht verfügbar. Bitte den Streamer vorerst, dort zu moderieren.',
    reloginStartFailed: 'Neu-Anmeldung konnte nicht gestartet werden. Bitte erneut versuchen.',

    rediscoverStarted: 'YouTube-Stream wird neu entdeckt…',
    rediscoverRateLimited: 'Bitte warte einen Moment, bevor du es erneut versuchst',
    rediscoverForbidden: 'Für dieses Overlay nicht berechtigt',
    rediscoverFailed: 'Neuentdeckung konnte nicht ausgelöst werden',

    reauthNeededToast: '{platform} verlangt, dass du die Moderation neu autorisierst',
    actionFailed: 'Moderationsaktion fehlgeschlagen',
    messageDeleted: 'Nachricht gelöscht',
    timedOut: '{name} in Timeout geschickt',
    banned: '{name} gebannt',
    unbanned: '{name} entbannt',
    unbanFailed: 'Entbannen fehlgeschlagen',
    unnamedTarget: 'Nutzer',

    connectRequired: 'Verbinde dein eigenes {platform}-Konto, um hier zu moderieren',
    ownerChannelUnverifiedModerator:
      'Das {platform}-Konto dieses Streamers ist nicht verbunden, daher kann hier nichts moderiert werden',
    ownerChannelUnverifiedOwner:
      'Dein {platform}-Konto ist für diesen Kanal nicht verbunden \u2014 verbinde es neu, um hier zu moderieren',
    delegationUnsupported:
      'Moderatoren können auf {platform} noch nicht agieren \u2014 bitte den Streamer, das zu übernehmen',
    targetNotActionable:
      '{platform} lässt nicht zu, dass jemand diese Person moderiert \u2014 sie ist Kanalbetreiber oder ein anderer Moderator',
    discordLinkRequired: 'Verknüpfe dein Discord-Konto, um hier zu moderieren',
    modNotInGuild:
      'Du bist nicht in diesem Discord-Server \u2014 bitte den Streamer, dich einzuladen',
    modLacksPermission:
      'Deine Discord-Rollen erlauben das nicht \u2014 bitte den Streamer um eine Rolle, die es erlaubt',
    modBelowTarget:
      'Die Rollen-Hierarchie von Discord blockiert das \u2014 deine höchste Rolle muss über ihrer liegen',
    botMissingPermission:
      'Dem All-Chat-Bot wurde diese Discord-Berechtigung nicht gegeben \u2014 bitte den Streamer, ihn neu einzuladen',
  },

  dock: {
    productName: 'All-Chat',
    signInExplanation:
      'Dieses Dock hat eine eigene Anmeldung, getrennt von deinem Browser. Melde dich einmal an, dann bleibst du angemeldet.',
    signInWith: 'Anmelden mit {platform}',
    signInFailed: 'Anmeldung konnte nicht gestartet werden',
    signInFailedBody: 'Anmeldung mit {platform} fehlgeschlagen. Bitte erneut versuchen.',

    chatTab: 'Chat',
    activityTab: 'Aktivität',
    splitTab: 'Beides',
    menuLabel: 'Monitor-Steuerung',
    noticesLabel: 'Monitor-Hinweise',
    noticesSummary: '{count} Hinweise',
  },

  eventNotice: {
    tokenExpired: 'OAuth-Token ist abgelaufen',
    tokenRefreshFailed: 'OAuth-Token konnte nicht aktualisiert werden',
    tokenExpiredFor: 'OAuth-Token ist für {username} abgelaufen',
    tokenRefreshFailedFor: 'OAuth-Token konnte für {username} nicht aktualisiert werden',
    tokenRemedy: '\u2192 Bitte verbinde dein Konto neu unter Einstellungen \u2192 Verbindungen',

    channelInaccessible: 'Kanal {channel} ist nicht zugreifbar',
    channelRemedy:
      '\u2192 Erteile dem Bot die Berechtigung \u201eKanal ansehen\u201c in den Server-Einstellungen deines Discord-Servers',

    listenerDeprecated: 'Die alte Twitch-Chat-Verbindung wird eingestellt.',
    listenerRemedy:
      '\u2192 Füge deine Twitch-Quelle neu hinzu, um auf die neue EventSub-Verbindung zu wechseln',
  },

  unsupportedKind: {
    heading: '{kind}-Overlay',
    body: 'Dieser Overlay-Typ wird noch nicht unterstützt.',
  },

  statusIndicator: {
    active: '{platform} - {channel} (Aktiv)',
    inactive: '{platform} - {channel} (Inaktiv)',
    connected: '{platform} - {channel} (Verbunden)',
    reconnecting: '{platform} - {channel} - Neuverbindung in {seconds}s',
    reconnectingWithError: '{platform} - {channel} - {error} (erneut in {seconds}s)',
    quotaExceeded: '{platform} - {channel} - Kontingent überschritten',
    error: '{platform} - {channel} - Fehler',
    discoveryPaused:
      '{platform} - {channel} - Erkennung pausiert (im Chat-Monitor erneut versuchen)',
    authRequired: '{platform} - {channel} - Anmeldung erforderlich',
    offline: '{platform} - {channel} - Offline',
    withErrorMessage: '{platform} - {channel} - {error}',
    countdownSeconds: '{seconds}s',
  },
}
