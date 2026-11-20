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

export const guides: LocaleNamespace<'guides'> = {
  shared: {
    eyebrow: 'Guide',
    ctaHeading: 'Richte es ein',
    ctaBody:
      'All-Chat ist kostenlos. Melde dich mit Twitch, YouTube oder Kick an, erstelle ein Overlay und füge dessen URL in OBS ein.',
    ctaSignIn: 'Anmelden',
    ctaDocs: 'Doku lesen',
    relatedHeading: 'Weitere Guides',
    limitsHeading: 'Grenzen',
    premiumLinkText: 'Premium',
    tiktokGuideLinkText: 'TikTok-Guide',
    linkObsOverlay: 'OBS-Chat-Overlay',
    linkObsDock: 'OBS-Chat-Dock',
    linkTiktok: 'TikTok-LIVE-Chat in OBS',
    linkMultistream: 'Multistream-Chat',
    linkAlerts: 'Stream-Alerts für OBS',
    linkChatBot: 'Multistream-Chatbot',
    linkCompare: 'Vergleich mit anderen Chat-Tools',
  },
  index: {
    metaTitle: 'Guides: OBS-Chat-Overlay, Chat-Dock, Stream-Alerts, Chatbot und Multistream-Chat',
    metaDescription:
      'Schritt-für-Schritt-Guides für All-Chat: zusammengeführten Chat in OBS einblenden, als Dock in OBS integrieren, TikTok-LIVE-Chat anzeigen, Multistream-Chat zusammenführen, Stream-Alerts und einen Chatbot ergänzen und Chat-Tools vergleichen.',
    breadcrumb: 'Guides',
    heading: 'Guides',
    intro:
      'So bekommst du den zusammengeführten Chat von Twitch, YouTube, Kick, TikTok und Discord in OBS, ergänzt Alerts und einen Chatbot – und wie sich All-Chat im Vergleich mit anderen Chat-Tools verhält.',
  },
  obsOverlay: {
    metaTitle: 'OBS-Chat-Overlay für Twitch, YouTube, Kick und TikTok',
    metaDescription:
      'Eine OBS-Browser-Quelle, die Twitch-, YouTube-, Kick-, TikTok- und Discord-Chat zusammen zeigt. URL, Größe, transparenter Hintergrund und die Quellen-Einstellungen, Schritt für Schritt.',
    breadcrumb: 'OBS-Chat-Overlay',
    heading: 'OBS-Chat-Overlay für Twitch, YouTube, Kick, TikTok und Discord',
    intro:
      'All-Chat führt den Chat aller Plattformen, auf denen du streamst, in einem Feed zusammen und stellt ihn als Webseite bereit. OBS zeigt diese Seite als Browser-Quelle an, also gibt es nichts zu installieren.',
    stepsHeading: 'Overlay zu OBS hinzufügen',
    stepSignIn: 'Melde dich auf {home} mit deinem Twitch-, YouTube- oder Kick-Konto an.',
    stepSignInLinkText: 'allch.at',
    stepCreate:
      'Erstelle im Dashboard ein Overlay und verbinde die Plattformen, auf denen du streamst. Jede Plattform wird zu einer Chatquelle dieses Overlays.',
    stepCopy: 'Kopiere die Overlay-URL. Sie hat die Form {url}.',
    stepAdd:
      'Klicke in OBS unter Quellen auf {plus}, wähle {browser} und füge die URL in das URL-Feld ein.',
    stepAddPlus: '+',
    stepAddBrowser: 'Browser',
    stepSize:
      'Stelle Breite und Höhe auf den Bereich ein, den der Chat füllen soll, zum Beispiel 400 × 800. Passe die Größe nicht an die volle Leinwand an.',
    stepShutdown:
      'Entscheide, ob du {shutdown} anhakst. Der folgende Abschnitt erklärt den Kompromiss.',
    stepShutdownEmphasis: 'Quelle bei Nichtanzeige herunterfahren',
    stepDone: 'Klicke auf OK. Der Chat erscheint, sobald sich das Overlay verbindet.',
    backgroundHeading: 'Transparenter Hintergrund',
    backgroundBody:
      'Die Overlay-Seite hat einen transparenten Hintergrund, also zeigen sich nur die Nachrichten über deiner Szene. Du brauchst dafür keinen Chroma-Key und kein zusätzliches CSS in OBS.',
    shutdownHeading: 'Quelle bei Nichtanzeige herunterfahren',
    shutdownBody:
      'All-Chat liest den Chat deiner Plattformen nur, solange ein Overlay verbunden ist. Mit angehaktem {shutdown} schließt OBS die Seite, wann immer die Quelle nicht in der aktiven Szene ist. Nach einer kurzen Schonfrist stellt All-Chat das Lesen deines Chats ein und beginnt von Neuem, wenn die Quelle sichtbar wird.',
    shutdownTradeoff:
      'Hake es an, wenn die Chat-Aufnahme stoppen soll, während das Overlay verborgen ist. Lasse es abgehakt, wenn du oft Szenen wechselst und den Feed bereits laufend haben willst, wenn du zurückkommst.',
    shutdown247:
      'Läuft dein OBS rund um die Uhr für einen IRL- oder 24/7-Stream? Lies zuerst den Abschnitt {section} der Doku. Er deckt eine passive Overlay-URL ab, die verhindert, dass die YouTube-Erkennung einfriert, während du offline bist.',
    shutdown247LinkText: '24/7 und IRL',
    themesHeading: 'Themes und benutzerdefiniertes CSS',
    themesBody:
      'Wähle eines der {themes} im Overlay-Editor. Schriftart, Abstände, Farben und Avatare lassen sich im Aussehen-Bereich ohne Code anpassen. Für volle Kontrolle schreibst du eigene Styles mit {css}.',
    themesLinkText: '16 mitgelieferte Themes',
    cssLinkText: 'benutzerdefiniertes CSS',
    platformBadges:
      'Jede Nachricht kann ein Badge für die Plattform zeigen, aus der sie stammt. Aktiviere es im Overlay-Editor oder gestalte jede Plattform selbst mit benutzerdefiniertem CSS.',
    emotesHeading: 'Emotes',
    emotesBody:
      '7TV-, BTTV- und FFZ-Emotes werden im Overlay neben den nativen Twitch- und YouTube-Emotes dargestellt. Dafür ist nichts einzurichten.',
    limitYoutube:
      'YouTube-Chat funktioniert eigenständig für öffentliche Livestreams. Für einen ungelisteten Stream füge dessen Link in die Quellen-Einstellungen ein. Member-only-Chat auf deinem eigenen Kanal braucht den Premium-Modus mit offizieller API. Geplante Streams werden aufgegriffen, sobald sie live geht.',
    limitTiktokReadOnly:
      'Du kannst TikTok-Chat aus All-Chat heraus weder beantworten noch moderieren. TikTok bietet dafür keine API.',
    limitRetention:
      'Chat-Nachrichten werden nach etwa einer Stunde aus All-Chat gelöscht. Das Overlay ist ein Live-Feed, kein Archiv.',
    limitTiktok:
      'TikTok ist die Plattform, die am ehesten für kurze Zeit ausfällt. Der {guide} erklärt, warum.',
  },
  obsDock: {
    metaTitle: 'OBS-Chat-Dock für Twitch, YouTube, Kick und TikTok',
    metaDescription:
      'Docke zusammengeführten Twitch-, YouTube-, Kick-, TikTok- und Discord-Chat in OBS über Ansicht > Docks > Benutzerdefinierte Browser-Docks an. Lesen, antworten, moderieren und Umfragen von einem Panel aus.',
    breadcrumb: 'OBS-Chat-Dock',
    heading: 'OBS-Chat-Dock für Twitch, YouTube, Kick, TikTok und Discord',
    intro:
      'Jedes All-Chat-Overlay bringt einen Chat-Monitor mit: dieselben Nachrichten, so angeordnet, dass du sie liest statt deine Zuschauer sie sehen. OBS kann den Monitor als Dock neben Szenen und Quellen halten, damit der Chat im Fenster bleibt, aus dem du streamst.',
    urlHeading: 'Die Dock-URL',
    urlBody:
      'Der Dock-Link für ein Overlay lautet {url}: der Monitor, angeordnet für ein schmales Panel. Klicke im Overlay-Editor auf Dock-URL kopieren, um ihn zu kopieren. Ohne ?dock=1 öffnet dieselbe Adresse den Monitor in voller Breite in einem Browser-Tab.',
    stepsHeading: 'Den Monitor als OBS-Dock hinzufügen',
    stepMenu: 'Öffne in OBS {menu}.',
    stepMenuPath: 'Ansicht > Docks > Benutzerdefinierte Browser-Docks',
    stepPaste:
      'Gib einen Namen für das Dock ein, füge die Dock-URL in das URL-Feld ein und klicke dann auf Anwenden und schließe den Dialog.',
    stepSignIn:
      'Melde dich im Dock an. OBS-Docks nutzen ihr eigenes Browser-Profil, daher überträgt sich eine Anmeldung in deinem normalen Browser nicht. Das machst du nur einmal.',
    stepPlace:
      'Ziehe das Panel dorthin, wo du es haben willst. Eine schmale Spalte neben Szenen und Quellen funktioniert gut.',
    streamlabsNote:
      'Auch Streamlabs Desktop hat benutzerdefinierte Browser-Docks, und die Dock-URL funktioniert dort auf dieselbe Weise.',
    featuresHeading: 'Was du aus dem Dock heraus kannst',
    featureRead: 'Lies jede Plattform in einer Liste, mit einem Aktivitäts-Feed daneben.',
    featureSend:
      'Sende Nachrichten als du selbst an Twitch, YouTube oder Kick. TikTok und Discord haben keine Sende-API, also bleiben sie hier schreibgeschützt.',
    featureModerate:
      'Löschen, Timeout, Bann und Entbannen auf der Plattform, aus der die Nachricht stammt: alle vier auf Twitch, Kick und Discord, Timeout und Bann auf YouTube. TikTok hat keine Moderations-API. Moderieren über All-Chat ist eine {premium}-Funktion.',
    featureEngage:
      'Starte und schließe Umfragen sowie starte, sperre und zahle Vorhersagen aus. Sie laufen über alle verbundenen Plattformen, und die Zuschauer steigen über den Chat ein.',
    featureMods:
      'Übergib den Monitor deinen Moderatoren. Sie handeln mit ihren eigenen Plattform-Konten und brauchen selbst kein Premium. Moderatoren hinzufügen braucht {premium} auf deinem Konto.',
    sameFeedHeading: 'Derselbe Feed wie im Overlay',
    sameFeedBody:
      'Dock und OBS-Overlay zeigen denselben Feed. Anzeigeeinstellungen im Monitor wie Avatare, Zeitstempel und Plattform-Icons ändern nur deine Ansicht, nie das, was die Zuschauer im Stream sehen.',
    limitPermissions:
      'Beim ersten Senden oder Moderieren bittet dich der Monitor, zusätzliche Berechtigungen mit {enable} zu erteilen. Bei Discord bedeutet das, den Bot erneut einzuladen.',
    limitPermissionsEmphasis: 'Moderation & Chat-Senden aktivieren',
    limitTiktok:
      'TikTok-Chat im Dock ist schreibgeschützt, und TikTok ist die Plattform, die am ehesten für kurze Zeit ausfällt. Der {guide} hat die Details.',
    moreDocs:
      'Die Abschnitte {monitor}, {moderation} und {engagement} der Doku gehen weiter ins Detail.',
    moreDocsMonitor: 'Chat-Monitor',
    moreDocsModeration: 'Moderation',
    moreDocsEngagement: 'Umfragen und Vorhersagen',
  },
  tiktok: {
    metaTitle: 'TikTok-LIVE-Chat-Overlay für OBS',
    metaDescription:
      'Zeige TikTok-LIVE-Chat in OBS neben Twitch-, YouTube-, Kick- und Discord-Chat, ausgelegt auf ein Verfügbarkeitsziel von 99,9 % ohne offizielle TikTok-API.',
    breadcrumb: 'TikTok-LIVE-Chat-Overlay',
    heading: 'TikTok-LIVE-Chat-Overlay für OBS',
    intro:
      'All-Chat zeigt TikTok-LIVE-Chat im selben OBS-Overlay wie deinen Twitch-, YouTube-, Kick- und Discord-Chat. TikTok ist die am schwierigsten anzubindende und die am wenigsten stabile der Plattformen, mit denen sich All-Chat verbindet, und trotzdem gilt für TikTok-Chat dasselbe Verfügbarkeitsziel von 99,9 % wie für jede andere Plattform. Diese Seite erklärt, wie das geht.',
    stepsHeading: 'TikTok zu einem Overlay hinzufügen',
    stepOpen: 'Öffne dein Overlay im All-Chat-Dashboard.',
    stepConnect:
      'Klicke auf {connect} und gib den TikTok-Benutzernamen des Creators ein. Es gibt keinen TikTok-Login-Schritt.',
    stepConnectEmphasis: 'TikTok verbinden',
    stepLive:
      'Geh auf TikTok live. Der Chat erscheint im Overlay, sobald All-Chat sieht, dass das Konto live ist.',
    stepObs: 'Falls das Overlay noch nicht in OBS liegt, folge dem {guide}.',
    stepObsLinkText: 'OBS-Chat-Overlay-Guide',
    apiHeading: 'Kein Tool hat eine offizielle TikTok-LIVE-Chat-API',
    apiBody:
      'TikTok hat keine öffentliche API für LIVE-Chat. Jedes Tool, das TikTok-LIVE-Chat zeigt, All-Chat eingeschlossen, liest ihn so, wie TikToks eigener Web-Player es tut.',
    approachHeading: 'Was All-Chat dazu tut',
    approachSigner:
      'All-Chat signiert TikTok-Verbindungen mit seinem eigenen {signer}; es ist kein Sign-Server eines Drittanbieters beteiligt. Die Entscheidung und ihre Geschichte stehen in {adr}.',
    approachSignerLinkText: 'Open-Source-Signing-Dienst',
    approachAdrLinkText: 'ADR-0052',
    approachSessions:
      'Am 09.09.2026 begann TikTok, Browser-Grade-Sessions für Live-Chat-Daten zu verlangen. All-Chat reagierte noch dieselbe Woche; die Updates sind in derselben ADR festgehalten.',
    approachFallback:
      'Räume auf {premium}-Konten bekommen einen zweiten Auslieferungspfad, wenn TikTok die primäre Verbindung verweigert.',
    approachStory: 'Die ganze Geschichte steht in {post}.',
    limitInterruptions:
      'TikTok bleibt die am wenigsten stabile Plattform, die All-Chat unterstützt. Wenn TikTok seinen Web-Player ändert, muss jedes Tool, das TikTok-LIVE-Chat liest, sich anpassen, All-Chat eingeschlossen, daher kann es zu kurzen Aussetzern kommen. Twitch-, YouTube-, Kick- und Discord-Chat im selben Overlay laufen währenddessen weiter.',
    limitReadOnly:
      'Du kannst TikTok-Chat aus All-Chat heraus weder beantworten noch moderieren. TikTok bietet dafür keine API.',
    verticalNote:
      'Vertikal zu TikTok und horizontal sonstwo streamst? Der {guide} behandelt ein zweites Overlay für die vertikale Leinwand.',
    verticalNoteLinkText: 'Multistream-Chat-Guide',
  },
  multistream: {
    metaTitle: 'Multistream-Chat: Twitch, YouTube, Kick und TikTok in einem Feed',
    metaDescription:
      'Führe den Chat aller Plattformen, zu denen du multistreamst, in einem OBS-Overlay und einem Moderations-Panel zusammen. Funktioniert mit SE.Live, Restream, Streamlabs Multistream oder mehreren RTMP-Ausgängen.',
    breadcrumb: 'Multistream-Chat',
    heading: 'Multistream-Chat: ein Feed für Twitch, YouTube, Kick, TikTok und Discord',
    intro:
      'Streamst du zu vier Plattformen gleichzeitig, kommt der Chat an vier Orten an. All-Chat führt ihn – samt Discord – zu einem Feed für dein OBS-Overlay und einem Panel, aus dem du liest und moderierst.',
    methodHeading: 'Funktioniert mit jedem Multistream-Setup',
    methodBody:
      'All-Chat verbindet sich direkt mit dem Chat jeder Plattform und rührt dein Video nie an. Wie das Video rausgeht, ist egal: SE.Live, Restream, Streamlabs Multistream, mehrere RTMP-Ausgänge aus OBS oder ein zweiter PC funktionieren alle gleich.',
    methodSources:
      'Der Chat hängt nicht davon ab, deinen Stream durch einen bestimmten Dienst zu schicken. Verbinde jede Plattform als Chatquelle an deinem Overlay und der Feed folgt.',
    platformsHeading: 'Plattformen auseinanderhalten',
    platformsBody:
      'Jede Nachricht kann ein Badge für die Plattform zeigen, aus der sie stammt, sodass du und deine Zuschauer auf einen Blick sehen, ob sie von Kick oder YouTube kam. Aktiviere Badges im Overlay-Editor. Mit {css} kannst du jeder Plattform eine eigene Farbe geben, weil jede Nachricht mit ihrer Plattform getaggt ist.',
    platformsCssLinkText: 'benutzerdefiniertes CSS',
    moderationHeading: 'Ein Ort zum Moderieren',
    moderationBody:
      'Der Chat-Monitor listet jede Plattform zusammen auf. Aus ihm kannst du auf Twitch, Kick und Discord löschen, Timeout geben, bannen und entbannen, und auf YouTube Timeout und Bann; jeweils angewandt auf der Plattform, aus der die Nachricht stammt. TikTok hat keine Moderations-API. Moderieren über All-Chat ist eine {premium}-Funktion, und du kannst den Monitor an deine Moderatoren übergeben.',
    moderationDock: 'Halte den Monitor in OBS mit dem {guide}.',
    moderationDockLinkText: 'OBS-Chat-Dock-Guide',
    engagementHeading: 'Umfragen und Vorhersagen über Plattformen hinweg',
    engagementBody:
      'All-Chat betreibt eigene Umfragen, Vorhersagen und Zuschauer-Punkte. Zuschauer auf jeder verbundenen Plattform steigen über den Chat ein, eine Umfrage ist also nicht auf dein Twitch-Publikum beschränkt.',
    overlaysHeading: 'Getrennte Overlays für getrennte Leinwände',
    overlaysBody:
      'Ein Konto kann mehrere Overlays haben, jedes mit eigenen Chatquellen und eigenem Theme. Ein übliches Setup ist ein Overlay mit jeder Plattform für die horizontale Leinwand und ein schmaleres zweites Overlay für eine vertikale Leinwand, zum Beispiel eines, das nur TikTok-Chat zeigt.',
    limitTiktok:
      'TikTok ist die Plattform, die am ehesten für kurze Zeit ausfällt. Der {guide} erklärt, warum.',
    limitYoutube:
      'YouTube-Chat funktioniert eigenständig für öffentliche Livestreams. Für einen ungelisteten Stream füge dessen Link in die Quellen-Einstellungen ein. Member-only-Chat auf deinem eigenen Kanal braucht den Premium-Modus mit offizieller API. Geplante Streams werden aufgegriffen, sobald sie live geht.',
    compareNote:
      'Wählst du zwischen Tools? Der {compare} listet auf, was Streamlabs, SE.Live, Social Stream Ninja, SleepyChat und Restream Chat bieten, mit Quellen.',
    compareNoteLinkText: 'Vergleichsseite',
  },
  compare: {
    metaTitle: 'Multistream-Chat-Tools im Vergleich',
    metaDescription:
      'All-Chat neben Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat, Restream Chat und Multistream: Plattformen, Installation, OBS-Overlay und Dock, Emotes, Preis, Open Source, Alerts und TTS, mit einer Quelle pro Zelle.',
    breadcrumb: 'Vergleich',
    heading: 'All-Chat im Vergleich mit anderen Multistream-Chat-Tools',
    intro:
      'Diese Tabelle stellt All-Chat neben sechs weitere Tools, die Chat von mehreren Plattformen zusammenführen. Wir bauen All-Chat, lies sie also mit diesem Wissen. Jede Aussage über ein anderes Tool verlinkt auf dessen eigene Seiten, und wo wir keine Aussage fanden, haben wir „nicht dokumentiert“ statt zu raten.',
    lastChecked: 'Zuletzt geprüft: {date}',
    caption:
      'Multistream-Chat-Tools nach Funktion, zuletzt geprüft {date}. Nummerierte Links zeigen auf die unten unter der Tabelle gelisteten Quellen.',
    colTool: 'Tool',
    colPlatforms: 'Twitch-, YouTube-, Kick-, TikTok-Chat',
    colInstall: 'Installation nötig',
    colOverlay: 'OBS-Overlay',
    colDock: 'OBS-Dock',
    colEmotes: '7TV, BTTV, FFZ',
    colPrice: 'Preis für zusammengeführten Chat',
    colOpenSource: 'Open Source',
    colSelfHost: 'Selbst hostbar',
    colAlerts: 'Alerts',
    colTts: 'TTS',
    notDocumented: 'Nicht dokumentiert',
    yes: 'Ja',
    allThree: 'Ja, alle drei',
    sourceRefLabel: 'Quelle {number}',
    toolAllChat: 'All-Chat',
    allChatPlatforms: 'Alle vier, plus Discord',
    allChatInstall: 'Nein. URL als Browser-Quelle einfügen',
    allChatDock: 'Ja, der Chat-Monitor',
    allChatPrice: 'Kostenlos. Premium-Extras über Patreon',
    allChatOpenSource: 'Ja, AGPL-3.0',
    allChatSelfHost: 'Ja, mit Docker Compose',
    allChatAlerts: 'Ja, ein Alert-Overlay, auf Wunsch mit eigenem HTML, CSS und JavaScript',
    allChatTts: 'Ja. Browser-Stimmen kostenlos, ElevenLabs mit Premium und eigenem Key',
    toolStreamlabs: 'Streamlabs Chat Box und Multichat',
    streamlabsPlatforms:
      'Twitch, YouTube, Kick. TikTok steht nicht in der Chat-Box-Plattformliste; ein Hilfe-Artikel sagt, der TikTok-Chat öffnet sich in einem separaten Browserfenster',
    streamlabsInstall:
      'Nein für die Chat Box, die eine Browser-Source-URL ist. Multichat kommt mit Streamlabs Multistream',
    streamlabsOverlay: 'Ja, die Chat Box',
    streamlabsDock: 'Multichat in Streamlabs Desktop. Ein OBS-Dock ist nicht dokumentiert',
    streamlabsPrice:
      'Chat Box kostenlos. Multistream ist eine Ultra-Funktion; das kostenlose Dual Output deckt ein horizontales und ein vertikales Ziel ab',
    streamlabsOpenSource: 'Streamlabs Desktop ist GPL-3.0. Widgets: nicht dokumentiert',
    streamlabsAlerts: 'Ja, Alert Box',
    streamlabsTts: 'Ja, Tips und Bits werden in der Alert Box vorgelesen',
    toolSeLive: 'StreamElements SE.Live',
    seLivePlatforms:
      'Multi-Chat zeigt jede verbundene Plattform, und Twitch, YouTube und Kick lassen sich verbinden. Der TikTok-Chat hat ein eigenes Dock außerhalb von Multi-Chat',
    seLiveInstall: 'Ja, ein OBS-Plugin, nur Windows',
    seLiveOverlay:
      'Ein zusammengeführtes Chat-Overlay ist nicht dokumentiert. Im Februar 2025 sagte das Team, das Chat-Widget zeige nur die primäre Plattform',
    seLiveDock: 'Ja, Chat-Dock und Multi-Chat-Fenster',
    seLivePrice: 'Kostenloses Plugin',
    seLiveAlerts: 'Ja, Overlay-Alerts',
    seLiveTts: 'Ja, über Stream Store-Einlösungen',
    toolSsn: 'Social Stream Ninja',
    ssnPlatforms: 'Alle vier, plus über 100 weitere Websites',
    ssnInstall:
      'Ja, eine Browser-Erweiterung oder Desktop-App. Eine Lite-Web-App hat weniger Funktionen',
    ssnDock: 'Ja, die Chat-Dock-Seite',
    ssnPrice: 'Kostenlos. Einige optionale Integrationen brauchen kostenpflichtige Konten',
    ssnOpenSource: 'Ja, GPL-3.0',
    ssnAlerts: 'Ja, Event-Alert-Overlays',
    ssnTts: 'Ja. System-Stimmen, plus optional kostenpflichtige Anbieter',
    toolSleepyChat: 'SleepyChat',
    sleepyChatPlatforms: 'Alle vier',
    sleepyChatInstall: 'Nein. Kopiere einen Link aus dem Web-Dashboard',
    sleepyChatPrice: 'Kostenlose Stufe mit Nutzungslimits. Premium hebt sie an',
    sleepyChatOpenSource:
      'Nicht dokumentiert. Das öffentliche GitHub-Repository ist ein Issue-Tracker',
    sleepyChatAlerts: 'Ja, eigene animierte Alerts',
    sleepyChatTts: 'Ja, eine Warteschlange über alle vier Plattformen',
    toolRestream: 'Restream Chat',
    restreamPlatforms: 'Twitch, YouTube, Kick. TikTok-Chat wird nicht gelesen',
    restreamInstall: 'Nein. Läuft im Browser; eine Desktop-App ist optional',
    restreamOverlay: 'Ja, eine Embed-URL als Browser-Quelle',
    restreamPrice:
      'Der Gratisplan enthält Chat. Nachrichten kommen nur an, während du über Restream streamst',
    restreamTts: 'Sprachausgabe-Alerts in der Desktop-Chat-App',
    toolMultistream: 'Multistream (multistream.gg)',
    multistreamPlatforms: 'Alle vier, plus Instagram',
    multistreamInstall: 'Ja, eine Desktop-App für Windows oder Mac. Linux wird nicht unterstützt',
    multistreamOverlay:
      'Ja, ein Browser-Quellen-Link zur lokalen App. Er funktioniert nur, solange die App auf demselben PC wie OBS geöffnet ist',
    multistreamDock:
      'Private Widgets lassen sich als OBS-Docks einbinden. Ein Chat-Dock ist nicht dokumentiert',
    multistreamPrice:
      'Kostenlos in der Desktop-App. Chat auf der Website gehört zu Pro, 10 $ im Monat',
    multistreamAlerts: 'Ja, Follows, Abos, Geschenke und Raids',
    multistreamTts: 'Ja, eingebaute Stimmen, solange die App läuft',
    sourcesHeading: 'Quellen',
    sourceAllChatReadme: 'All-Chat-README auf GitHub',
    sourceStreamlabsChatBox: 'Streamlabs: Das Chat-Box-Widget',
    sourceStreamlabsMultistream: 'Streamlabs: Multistream-Seite und Multichat-FAQ',
    sourceStreamlabsWidgets: 'Streamlabs: Widget-Seite und FAQ',
    sourceStreamlabsTiktok: 'Streamlabs-Hilfe: So gehst du vom Desktop aus auf TikTok live',
    sourceStreamlabsDesktopRepo: 'Streamlabs-Desktop-Quellcode auf GitHub',
    sourceStreamlabsTts: 'Streamlabs: Sprachausgabe für Tips (und Bits) hinzufügen',
    sourceSeLiveGuide: 'StreamElements-Hilfe: SE.Live, der komplette Guide',
    sourceSeMultichatOverlay:
      'StreamElements-Community: Multichat-Overlay-Anfrage und Antwort des Personals',
    sourceSeAlerts: 'StreamElements-Hilfe: Twitch-Alerts einrichten mit StreamElements-Overlays',
    sourceSeTts: 'StreamElements-Hilfe: Stream Store, der komplette Guide',
    sourceSsnReadme: 'Social Stream Ninja-README auf GitHub',
    sourceSsnFeatures: 'Social Stream Ninja: Funktionen',
    sourceSsnParameters: 'Social Stream Ninja: Overlay-Parameter',
    sourceSleepyChatHome: 'SleepyChat-Startseite',
    sourceSleepyChatObs: 'SleepyChat: OBS-Browser-Dock- und Overlay-Guide',
    sourceSleepyChatPricing: 'SleepyChat: Preisseite',
    sourceSleepyChatRepo: 'SleepyChat-Repository auf GitHub',
    sourceRestreamRead: 'Restream-Hilfe: Eingehende Chat-Nachrichten lesen',
    sourceRestreamChat: 'Restream: Chat-Produktseite',
    sourceRestreamEncoders: 'Restream-Hilfe: Restream Chat mit Encodern verwenden',
    sourceRestreamEmbed: 'Restream-Hilfe: Chat in deine Streaming-Software einbetten',
    sourceRestreamFree: 'Restream-Hilfe: Kann ich Restream kostenlos nutzen?',
    sourceMultistreamHome: 'Multistream-Startseite',
    sourceMultistreamDownload: 'Multistream: App herunterladen',
    sourceMultistreamOverlay: 'Multistream-Tutorial: Ein Overlay zu OBS hinzufügen',
    sourceMultistreamChats: 'Multistream-Guide: Alle Stream-Chats in einem Fenster',
    sourceMultistreamPricing: 'Multistream: Preise',
    sourceMultistreamTts: 'Multistream-Tutorial: Chat vorlesen lassen',
    betterHeading: 'Wann ein anderes Tool die bessere Wahl ist',
    betterIntro: 'Kein Tool auf dieser Liste ist in allem am besten. Nimm ein anderes, wenn:',
    betterAllInOne:
      'Du willst, dass das Tool, das deinen Chat zusammenführt, auch dein Video an jede Plattform sendet. Multistream und Restream machen beides, Streamlabs Multistream mit Ultra. All-Chat überträgt kein Video; es legt Chat, Alerts und einen Chatbot über das, womit du streamst.',
    betterTts:
      'Du willst Sprachausgabe an Tips und Alerts gekoppelt. Streamlabs liest Tips und Bits in seiner Alert Box vor, und SleepyChat hält TTS von allen vier Plattformen in einer Warteschlange neben seinen Alerts.',
    betterPlatforms:
      'Du streamst auf Plattformen außerhalb dieser vier. Social Stream Ninja deckt über 100 Websites ab, darunter Facebook, Instagram, Rumble und Zoom.',
    betterRestream:
      'Du multistreamst schon über Restream und willst aus einem Fenster heraus antworten und Nachrichten über Plattformen hinweg weiterleiten. Restream Chat ist in diesen Workflow eingebaut.',
    fitHeading: 'Wo All-Chat passt',
    fitBody:
      'All-Chat braucht keine Installation, deckt TikTok neben Twitch, YouTube, Kick und Discord ab, rendert 7TV-, BTTV- und FFZ-Emotes und ist unter AGPL-3.0 quelloffen. Alert-Overlays, Chat-Befehle, Stichwörter und Timer sind eingebaut: Chat, Alerts und Chatbot kommen aus einem Browser-Tab, ohne dass neben OBS eine App laufen muss. Zusammengeführter Chat, Alerts und Befehle sind kostenlos; Moderation aus dem Chat-Monitor und einige Extras sind Premium.',
    corrections: 'Produkte ändern sich. Falls eine Zelle veraltet ist, {issue}, und wir fixen sie.',
    correctionsLinkText: 'ein Issue auf GitHub',
  },
  // /stream-alerts. The event lists mirror shared/alerts eventDefs: name a
  // platform's event here only once that file has it.
  alerts: {
    metaTitle: 'Stream-Alerts für OBS: Twitch, YouTube und TikTok in einer Alert-Box',
    metaDescription:
      'Kostenlose Alert-Box für OBS, die Twitch, YouTube und TikTok zugleich abdeckt: Abos, Bits, Raids, Follows, Super Chats, Mitgliedschaften und TikTok-Geschenke, mit eigenem GIF, Sound und Text oder eigenem HTML, CSS und JavaScript.',
    breadcrumb: 'Stream-Alerts',
    heading: 'Stream-Alerts für Twitch, YouTube und TikTok in einer Alert-Box',
    intro:
      'Ein All-Chat-Alert-Overlay ist eine Alert-Box, die du als Browser-Quelle in OBS einfügst. Es zeigt Events von jeder verbundenen Plattform, sodass ein Multistream eine Alert-Quelle braucht statt einer pro Plattform. Es ist kostenlos und läuft im Browser, ohne Installation.',
    eventsHeading: 'Was es zeigt',
    eventsTwitch:
      'Twitch: Abos, Resubs, Geschenk-Abos, Bits, Raids, Follows, Kanalpunkte-Einlösungen und Charity-Spenden.',
    eventsYoutube:
      'YouTube: Super Chats, Super Sticker, neue Mitglieder, Mitglieder-Meilensteine, verschenkte Mitgliedschaften und neue Abonnenten.',
    eventsTiktok: 'TikTok LIVE: Geschenke, Shares und Abos.',
    eventsBody:
      'Alerts erscheinen nacheinander mit ihrem Text, einem Bild oder GIF und einem Sound, danach ist die Quelle wieder leer.',
    stepsHeading: 'Eine Alert-Box zu OBS hinzufügen',
    stepCreate: 'Melde dich auf allch.at an und erstelle ein Overlay der Art {kind}.',
    stepCreateKind: 'Alerts',
    stepSources:
      'Füge für jede Plattform, auf der du streamst, eine Chat-Quelle hinzu. Ein Alert-Overlay nutzt dieselben Quellen wie ein Chat-Overlay.',
    stepStyle:
      'Lege im Abschnitt {section} des Overlay-Editors unter {all} das Aussehen aller Alerts fest: ein Bild oder GIF, einen Sound, eine Dauer und eine Lautstärke.',
    stepStyleSection: 'Alerts',
    stepStyleAll: 'Alle Ereignisse (Standardstil)',
    stepObs: 'Kopiere die Overlay-URL und füge sie in OBS als Browser-Quelle ein.',
    stepTest: 'Klicke auf {test}. Der Alert läuft in der Editor-Vorschau und in OBS.',
    stepTestButton: 'Test-Alert senden',
    customHeading: 'Mach jeden Alert zu deinem',
    customBody:
      'Wähle einen Event-Typ, um ihm einen eigenen Text oder ein eigenes Bild und einen eigenen Sound zu geben. Der Text kennt diese Platzhalter: {placeholders}. Leere Felder nutzen den Standardstil. Suche GIFs im Editor oder füge einen Link ein.',
    codeBody:
      'Für volle Kontrolle schreibst du eigenes HTML, CSS und JavaScript für einen Alert. Es läuft in einer isolierten Sandbox im Overlay.',
    commandsBody:
      'Ein {commands} kann seinen eigenen Alert im selben Overlay zeigen: ein Bild, einen Sound und einen Text, ausgelöst aus jedem Chat.',
    commandsLinkText: 'Chat-Befehl',
    limitKick:
      'Kick-Events wie Abos und Follows werden nicht als Alerts gezeigt. Ein Chat-Befehl im Kick-Chat kann trotzdem seinen eigenen Alert auslösen.',
    limitUploads:
      'Eigene Bilder, GIFs und Sounds hochzuladen ist eine {premium}-Funktion. GIFs suchen und einen Link einfügen sind kostenlos.',
    limitTiktok:
      'TikTok fällt am ehesten kurzzeitig aus, und seine Alerts dann mit. Der {guide} hat die Details.',
  },
  // /multistream-chat-bot (ADR-0079, ADR-0080, ADR-0081).
  chatBot: {
    metaTitle: 'Multistream-Chatbot: Befehle und Timer für Twitch, YouTube und Kick',
    metaDescription:
      'Kostenloser Chatbot für Multistreamer: Lege !Befehle, Stichwörter und Timer einmal an, und sie antworten im Twitch-, Kick- und YouTube-Chat. Variablen, Web-Abfragen, JavaScript, Twitch-Clips und Alerts im Stream.',
    breadcrumb: 'Chatbot',
    heading: 'Ein Chatbot für Twitch, YouTube und Kick: Befehle, Stichwörter und Timer',
    intro:
      'All-Chat hat einen Chatbot eingebaut. Du legst einen Befehl einmal an, und er antwortet in jedem Chat, den deine Overlays führen, sodass ein Multistream nicht einen Bot pro Plattform braucht. Er läuft auf den Servern von All-Chat: nichts zu installieren und nichts, was laufen muss.',
    kindsHeading: 'Befehle, Stichwörter und Timer',
    kindCommand:
      'Ein Befehl löst aus, wenn eine Nachricht mit seinem Auslöser beginnt, zum Beispiel !discord.',
    kindKeyword:
      'Ein Stichwort löst aus, wenn sein Wort oder Satz irgendwo in einer Nachricht vorkommt, zum Beispiel clip that.',
    kindTimer:
      'Ein Timer postet eine Nachricht alle X Minuten, nach jeweils X Chatnachrichten oder alle X Minuten, sobald genug Nachrichten kamen.',
    stepsHeading: 'Erstelle deinen ersten Befehl',
    stepOpen: 'Melde dich auf allch.at an und öffne {commands} in der oberen Leiste.',
    stepOpenNav: 'Befehle',
    stepNew: 'Klicke auf {create} und gib einen Auslöser wie !discord ein.',
    stepNewButton: 'Neuer Befehl',
    stepReply: 'Schreibe die Antwort. Sie wird in den Chat gepostet, aus dem die Nachricht kam.',
    stepSave: 'Speichere. Der Befehl ist in jedem Chat aktiv, den deine Overlays führen.',
    repliesHeading: 'Was eine Antwort kann',
    repliesBody:
      'Antworten sind Vorlagen. Variablen setzen den Chatter, den Text nach dem Auslöser, einen Zähler, eine Zufallszahl, eine Auswahl aus einer Liste oder die Uhrzeit einer Zeitzone ein. Eine urlfetch-Variable setzt ein, was eine Webadresse antwortet, und ein js-Block führt JavaScript in einer Sandbox aus.',
    actionsBody:
      'Ein Befehl kann außerdem deinen laufenden Twitch-Stream clippen oder seinen eigenen Alert mit Bild, Sound und Text auf deinem {alerts} zeigen.',
    actionsAlertsLinkText: 'Alert-Overlay',
    accessBody:
      'Jeder Befehl hat eine Abklingzeit und lässt sich auf Abonnenten, VIPs, Moderatoren oder dich selbst beschränken.',
    senderHeading: 'Wer die Antworten sendet',
    senderBody:
      'Standardmäßig antwortet der All-Chat Bot, auf Twitch, Kick und YouTube, ohne eigenes Konto. Antworten können stattdessen von deinem eigenen Konto kommen. Ein eigenes Bot-Konto zu verbinden, damit Antworten dessen Namen tragen, ist eine {premium}-Funktion.',
    limitYoutube:
      'Antworten und Timer sind im Twitch- und Kick-Chat kostenlos. Im YouTube-Chat sind sie eine {premium}-Funktion, weil jede einzelne YouTube-API-Kontingent verbraucht.',
    limitReadOnly:
      'TikTok- und Discord-Chats können einen Befehl auslösen, zum Beispiel einen Clip oder einen Alert, aber es kann dort nicht geantwortet werden.',
  },
  // /compare/<tool>: All-Chat next to one tool. The table cells are the
  // /compare cells; only this prose is per tool, and it may state about the
  // other tool only what its /compare cells already source.
  vs: {
    colFeature: 'Merkmal',
    caption:
      'All-Chat und {tool} nach Merkmal, zuletzt geprüft am {date}. Nummerierte Links führen zu den Quellen unten.',
    pickThemHeading: 'Wann {tool} die bessere Wahl ist',
    pickUsHeading: 'Wann All-Chat die bessere Wahl ist',
    fullComparison: 'Alle Tools nebeneinander stehen auf der {compare}.',
    fullComparisonLinkText: 'Vergleichsseite',
    listHeading: 'Vergleiche eins zu eins',
    multistreamName: 'Multistream',
    multistreamMetaTitle:
      'All-Chat vs. Multistream (multistream.gg): Chat-Overlay, Alerts und Chatbot',
    multistreamMetaDescription:
      'All-Chat und Multistream im direkten Vergleich: Beide führen Twitch-, YouTube-, Kick- und TikTok-Chat zusammen und bringen Alerts und einen Chatbot mit. Das eine läuft im Browser, das andere ist eine Desktop-App für Windows und Mac. Mit einer Quelle pro Zelle.',
    multistreamHeading: 'All-Chat vs. Multistream',
    multistreamIntro:
      'Beide Tools bringen Twitch-, YouTube-, Kick- und TikTok-Chat in ein OBS-Overlay und ergänzen Alerts, einen Chatbot und Sprachausgabe. Der Unterschied liegt darin, wo sie laufen. Multistream ist eine Desktop-App, die auch dein Video an die Plattformen sendet. All-Chat ist ein gehosteter Open-Source-Dienst, den du als URL in OBS einfügst. Wir bauen All-Chat; jede Multistream-Zelle verlinkt auf dessen eigene Seiten.',
    multistreamPickThem:
      'Du willst, dass dieselbe App auch deinen Stream an jede Plattform sendet. Multistream streamt kostenlos von deinem eigenen Rechner und mit Pro aus der Cloud, und Chat, Alerts und Chatbot stecken in derselben Windows- oder Mac-App.',
    multistreamPickUs:
      'Du willst keine weitere App neben OBS laufen lassen. Ein All-Chat-Overlay ist eine URL, die auf jedem Rechner funktioniert, auch unter Linux, während der Overlay-Link von Multistream nur funktioniert, solange die App auf demselben PC wie OBS geöffnet ist. All-Chat liest außerdem Discord, rendert 7TV-, BTTV- und FFZ-Emotes, nimmt dein eigenes CSS und ist quelloffen und selbst hostbar.',
    ssnName: 'Social Stream Ninja',
    ssnMetaTitle: 'All-Chat vs. Social Stream Ninja: Chat-Overlay gehostet oder per Erweiterung',
    ssnMetaDescription:
      'All-Chat und Social Stream Ninja im direkten Vergleich: Beide sind kostenlos und quelloffen und führen Twitch-, YouTube-, Kick- und TikTok-Chat zusammen. Das eine ist gehostet, das andere läuft in einer Browser-Erweiterung oder Desktop-App.',
    ssnHeading: 'All-Chat vs. Social Stream Ninja',
    ssnIntro:
      'Beide sind kostenlose Open-Source-Tools, die Chat von mehreren Plattformen in ein OBS-Overlay und ein Dock zusammenführen. Social Stream Ninja erfasst den Chat mit einer Browser-Erweiterung oder Desktop-App auf deinem Rechner. All-Chat liest die Plattformen auf seinen Servern und gibt dir eine URL. Wir bauen All-Chat; jede Social-Stream-Ninja-Zelle verlinkt auf dessen eigene Seiten.',
    ssnPickThem:
      'Du streamst auf Plattformen jenseits von Twitch, YouTube, Kick, TikTok und Discord, oder alles soll auf deinem eigenen Rechner laufen. Social Stream Ninja deckt mehr als 100 Websites ab.',
    ssnPickUs:
      'Du willst nichts installieren und nichts offen halten müssen. All-Chat braucht keine Erweiterung und keine App: Die Overlay-URL funktioniert in OBS auf jedem Rechner. Moderation, Umfragen, Alerts, Chat-Befehle und Timer laufen über dasselbe Konto.',
    streamlabsName: 'Streamlabs',
    streamlabsMetaTitle: 'All-Chat vs. Streamlabs Chat Box: Multistream-Chat mit TikTok',
    streamlabsMetaDescription:
      'All-Chat neben der Streamlabs Chat Box und Multichat: Plattformen, TikTok-Chat, OBS-Overlay und Dock, Emotes, Preis, Alerts und TTS, mit einer Quelle pro Zelle.',
    streamlabsHeading: 'All-Chat vs. Streamlabs Chat Box und Multichat',
    streamlabsIntro:
      'Streamlabs hat zwei Chat-Tools: das Chat-Box-Widget, eine Browser-Quelle für Twitch, YouTube und Kick, und Multichat, das zu Streamlabs Multistream gehört. All-Chat ist ein gehostetes Open-Source-Chat-Overlay, das auch TikTok und Discord enthält. Wir bauen All-Chat; jede Streamlabs-Zelle verlinkt auf dessen eigene Seiten.',
    streamlabsPickThem:
      'Du streamst bereits mit Streamlabs Desktop und willst Chat, Alert Box und Tips in einem Konto, oder Tips und Bits sollen per Sprachausgabe vorgelesen werden.',
    streamlabsPickUs:
      'Du streamst auch auf TikTok. TikTok steht nicht in der Plattformliste der Chat Box, während All-Chat den TikTok-LIVE-Chat in dasselbe Overlay wie Twitch, YouTube, Kick und Discord bringt. Zusammengeführter Chat von jeder Plattform ist bei All-Chat kostenlos, während Streamlabs Multistream zu Ultra gehört, und All-Chat ist quelloffen und selbst hostbar.',
    seLiveName: 'StreamElements SE.Live',
    seLiveMetaTitle: 'All-Chat vs. StreamElements SE.Live: Zusammengeführtes Chat-Overlay für OBS',
    seLiveMetaDescription:
      'All-Chat und StreamElements SE.Live im direkten Vergleich: zusammengeführtes Chat-Overlay, Multi-Chat-Dock, TikTok, Installation, Preis, Alerts und TTS, mit einer Quelle pro Zelle.',
    seLiveHeading: 'All-Chat vs. StreamElements SE.Live',
    seLiveIntro:
      'SE.Live ist das StreamElements-Plugin für OBS unter Windows, mit einem Multi-Chat-Fenster für den Streamer. All-Chat ist ein gehosteter Open-Source-Dienst, der den zusammengeführten Chat als Browser-Quelle in den Stream bringt. Wir bauen All-Chat; jede StreamElements-Zelle verlinkt auf dessen eigene Seiten.',
    seLivePickThem:
      'Du nutzt bereits StreamElements-Overlays, Alerts und einen Stream Store unter Windows und willst vor allem selbst jeden Chat in einem OBS-Dock lesen.',
    seLivePickUs:
      'Deine Zuschauer sollen den zusammengeführten Chat im Stream sehen. In der StreamElements-Dokumentation haben wir kein zusammengeführtes Chat-Overlay gefunden, und im Februar 2025 sagte das Team, das Chat-Widget zeige nur die primäre Plattform. All-Chat hält außerdem TikTok im zusammengeführten Chat statt in einem eigenen Dock und braucht weder ein Plugin noch ein bestimmtes Betriebssystem.',
    sleepyChatName: 'SleepyChat',
    sleepyChatMetaTitle: 'All-Chat vs. SleepyChat: Multistream-Chat-Overlays im Vergleich',
    sleepyChatMetaDescription:
      'All-Chat und SleepyChat im direkten Vergleich: Beide führen Twitch-, YouTube-, Kick- und TikTok-Chat über ein Web-Dashboard ohne Installation zusammen. Emotes, Preis, Open Source, Alerts und TTS, mit einer Quelle pro Zelle.',
    sleepyChatHeading: 'All-Chat vs. SleepyChat',
    sleepyChatIntro:
      'Die beiden ähneln sich in unserem Vergleich am meisten: Beide sind gehostet, brauchen keine Installation und geben dir ein OBS-Overlay und ein Dock für Twitch-, YouTube-, Kick- und TikTok-Chat. Wir bauen All-Chat; jede SleepyChat-Zelle verlinkt auf dessen eigene Seiten.',
    sleepyChatPickThem:
      'Sprachausgabe soll im Mittelpunkt deines Streams stehen. SleepyChat hält TTS von allen vier Plattformen in einer Warteschlange neben seinen eigenen animierten Alerts.',
    sleepyChatPickUs:
      'Du willst Open Source. All-Chat steht unter AGPL-3.0 und lässt sich mit Docker Compose selbst hosten. Es liest außerdem Discord und rendert 7TV-, BTTV- und FFZ-Emotes, und der zusammengeführte Chat ist kostenlos, während die kostenlose Stufe von SleepyChat Nutzungslimits hat.',
    restreamName: 'Restream Chat',
    restreamMetaTitle: 'All-Chat vs. Restream Chat: Multistream-Chat mit TikTok',
    restreamMetaDescription:
      'All-Chat und Restream Chat im direkten Vergleich: Plattformen, TikTok-Chat, OBS-Overlay und Dock, Preis, Alerts und TTS, mit einer Quelle pro Zelle.',
    restreamHeading: 'All-Chat vs. Restream Chat',
    restreamIntro:
      'Restream Chat ist das Chat-Tool von Restream, dem Dienst, der einen Stream an mehrere Plattformen sendet. All-Chat überträgt kein Video; es führt den Chat zusammen, egal wie dein Stream die Plattformen erreicht. Wir bauen All-Chat; jede Restream-Zelle verlinkt auf dessen eigene Seiten.',
    restreamPickThem:
      'Du streamst bereits über Restream auf mehrere Plattformen und willst aus einem Fenster antworten und Nachrichten plattformübergreifend weiterleiten.',
    restreamPickUs:
      'Du streamst auf TikTok, oder dein Stream läuft nicht über Restream. Restream Chat liest keinen TikTok-Chat, und Nachrichten kommen nur an, solange du über Restream streamst. All-Chat liest den TikTok-LIVE-Chat und funktioniert mit jeder Art, live zu gehen, und bringt 7TV-, BTTV- und FFZ-Emotes, Alerts und einen Chatbot mit.',
  },
}
