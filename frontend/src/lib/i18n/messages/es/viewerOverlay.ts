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
    pollHeading: 'Encuesta',
    predictionHeading: 'Predicción',
    twitchSourceBadge: 'Twitch',

    labelListEntry: '{noun} {index}',
    labelListRemove: 'Quitar',
    labelListAdd: 'Añadir {noun}',

    pollQuestionPlaceholder: 'Pregunta',
    pollQuestionLabel: 'Pregunta de la encuesta',
    pollOptionNoun: 'Opción',
    pollAllowChange: 'Permitir cambiar el voto',
    pollAutoCloseAfter: 'Cerrar automáticamente tras',
    secondsSuffix: 's',
    pollStart: 'Iniciar encuesta',

    pollVotes: '{total} votos',
    pollAutoCloses: ' · cierre automático {time}',
    pollNew: 'Nueva encuesta',
    pollClose: 'Cerrar encuesta',
    pollMirroredNote:
      'Reflejada desde Twitch: los espectadores votan en la interfaz o el chat de Twitch',
    pollParticipateHint:
      'Los espectadores votan en la {link} o desde el chat ({voteCommand} o simplemente {shortCommand})',
    participateLink: 'página de participación',

    predictionTitlePlaceholder: 'Título (p. ej. ¿Ganaremos esta ronda?)',
    predictionTitleLabel: 'Título de la predicción',
    predictionOutcomeNoun: 'Resultado',
    predictionAutoLockAfter: 'Bloquear apuestas automáticamente tras',
    predictionStart: 'Iniciar predicción',
    predictionParticipateHint:
      'Los espectadores apuestan en la {link} (donde ven su saldo) o desde el chat: {predictCommand}',

    predictionPointsWagered: '{total} puntos apostados',
    predictionAutoLocks: ' · bloqueo automático {time}',
    predictionOutcomeTally: '{points} pts · {entrants} participantes',
    winningOutcome: 'Resultado ganador',
    winningOutcomeChoice: 'Resultado ganador: {label}',
    predictionLock: 'Bloquear apuestas',
    predictionNew: 'Nueva predicción',
    predictionLockedNote: 'Elige el resultado ganador y luego paga. Los pagos son definitivos.',
    predictionMirroredNote: 'Reflejada desde Twitch: funciona con puntos de canal de Twitch',

    predictionResolve: 'Resolver',
    predictionPayOut: 'Pagar "{label}"',
    predictionPayOutConfirm: '¿Pagar "{label}"? Es definitivo',
    predictionResolveDisabledTitle: 'Selecciona primero el resultado ganador',

    predictionCancel: 'Cancelar y reembolsar',
    predictionCancelTitle: 'Cancelar y reembolsar todas las apuestas',
    predictionCancelConfirm: '¿Seguro que quieres reembolsar todas las apuestas?',

    mirrorNote:
      'Refleja las encuestas y predicciones nativas de Twitch en tus overlays (solo lectura). Es opcional; se aplica tras la siguiente sincronización del canal (al reiniciar el stream o volver a añadir la fuente).',
    mirrorEnable: 'Activar el reflejo de Twitch',

    pollIncompleteToast: 'Una encuesta necesita una pregunta y al menos 2 opciones',
    pollStartedToast: 'Encuesta iniciada',
    pollStartFailed: 'No se pudo iniciar la encuesta',
    pollClosedToast: 'Encuesta cerrada',
    pollCloseFailed: 'No se pudo cerrar la encuesta',
    predictionIncompleteToast: 'Una predicción necesita un título y al menos 2 resultados',
    predictionStartedToast: 'Predicción iniciada',
    predictionStartFailed: 'No se pudo iniciar la predicción',
    predictionLockedToast: 'Predicción bloqueada: las apuestas quedan congeladas',
    predictionLockFailed: 'No se pudo bloquear la predicción',
    pickWinnerToast: 'Elige primero el resultado ganador',
    predictionNoLongerLockedToast:
      'La predicción ya no está bloqueada: actualiza e inténtalo de nuevo',
    predictionResolvedToast: 'Predicción resuelta: se pagó a los ganadores',
    predictionResolveFailed: 'No se pudo resolver la predicción',
    predictionCanceledToast: 'Predicción cancelada: se reembolsaron todas las apuestas',
    nothingToCancelToast: 'Nada que cancelar: la predicción ya está en estado {state}',
    predictionCancelFailed: 'No se pudo cancelar la predicción',
    twitchConsentFailedToast: 'No se pudo iniciar la autorización de Twitch. Inténtalo de nuevo.',
  },

  participate: {
    loading: 'Cargando…',
    loginHeading: 'Únete a la diversión',
    loginBlurb: 'Inicia sesión con tu cuenta de la plataforma para votar y apostar.',
    loginWith: 'Continuar con {platform}',
    noWebLoginNote:
      '¿Estás viendo desde TikTok o Discord? Participa con los comandos de chat que aparecen en pantalla: el inicio de sesión web todavía no está disponible para esas plataformas.',

    heading: 'Participar',
    balanceLabel: 'Saldo: {balance} {pointsName}',
    balance: '{balance} {pointsName}',
    defaultPointsName: 'Puntos',

    settledBanner:
      'Tu predicción por «{outcome}» se ha resuelto: apostaste {amount} {pointsName}. Consulta tu saldo arriba.',

    pollNativeNote:
      'Esta encuesta se realiza en Twitch: vota en el chat de Twitch o en la app de Twitch.',
    pollVoteNativeTitle: 'Vota en el chat de Twitch',
    working: 'Procesando…',
    yourVote: '(tu voto)',
    pollOptionTally: '{pct}% ({votes})',

    predictionLocked: 'bloqueada',
    predictionNativeNote: 'Esta predicción funciona con puntos de canal de Twitch.',
    youHave: 'Tienes {balance} {pointsName}',
    maxWager: 'Máx.',
    noPointsYet:
      'Aún no tienes {pointsName}. Consíguelos manteniendo esta página abierta y apoyando el stream (subs, bits, donaciones, regalos), y luego vuelve para apostar.',
    wagerAmountLabel: 'Cantidad a apostar en {pointsName}',
    wagerAmountPlaceholder: 'Cantidad a apostar ({pointsName})',
    yourWager: ' · tu apuesta: {amount}',
    outcomeTally: '{points} · {pct}%',
    alreadyWagered: 'Ya has fijado tu apuesta para esta ronda.',
    nothingActive: 'Ahora mismo no hay ninguna encuesta ni predicción activa. ¡Un momento!',

    wagerNativeTitle: 'Funciona con puntos de canal de Twitch',
    wagerAlreadyTitle: 'Ya apostaste en esta ronda',
    wagerClosedTitle: 'Las apuestas están cerradas',

    lockedAnnouncement: 'Predicción bloqueada: las apuestas están cerradas.',

    rejectNotFound: 'Esta predicción ya no está disponible.',
    rejectNotActive: 'Las apuestas están cerradas para esta ronda.',
    rejectBadOutcome: 'Ese resultado no es válido.',
    rejectAlreadyWagered: 'Ya hiciste una apuesta en esta ronda.',
    rejectInsufficient: 'No tienes suficientes {pointsName}. Tienes {balance}.',
    rejectNative: 'Esta predicción funciona con puntos de canal de Twitch.',
    insufficientLocal: 'No tienes suficientes {pointsName}: tienes {balance}.',

    loginFailed: 'No se pudo iniciar sesión. Inténtalo de nuevo.',
    voteFailed: 'No se pudo votar',
    wagerFailed: 'No se pudo apostar',
    wagerNeedsAmount: 'Introduce una cantidad positiva para apostar.',
  },

  pollWidget: {
    finalBadge: 'Final',
    finalResults: 'Resultados finales',
    winnerPill: 'Ganadora',
    tiePill: 'Empate',
    optionTally: '{pct}% ({votes})',
    remaining: 'Quedan {clock}',
  },

  predictionWidget: {
    stateLocked: '🔒 BLOQUEADA',
    stateResolved: '🏆 RESUELTA',
    stateOpen: 'ABIERTA',
    winnerPill: 'Ganador',
    outcomeTally: '{points} pts · {pct}%',
    pool: '{points} pts apostados · {players} jugadores',
  },

  credits: {
    loading: 'Cargando créditos...',
    loadFailed: 'No se pudieron cargar los créditos finales',
    errorHeading: 'No se pueden cargar los créditos finales',
    errorHint: 'Asegúrate de tener una sesión de stream activa',
    empty: 'No hay datos para los créditos finales',

    heading: '🎬 Créditos del stream',
    subheading: '¡Gracias a todos los que apoyaron el stream!',
    session: 'Sesión: {date}',
    duration: 'Duración: {duration}',

    durationHourOne: '{hours} hora',
    durationHourMany: '{hours} horas',
    durationMinuteOne: '{minutes} minuto',
    durationMinuteMany: '{minutes} minutos',
    durationHoursAndMinutes: '{hours} {minutes}',
    durationJustStarted: 'recién empezado',

    topSubscribers: 'Top suscriptores',
    topGifters: 'Top regaladores',
    topCheerers: 'Top cheers',
    topChannelPoints: 'Top puntos de canal',
    topRaiders: 'Top raiders',
    topSuperChats: 'Top Super Chats',
    newFollowers: 'Nuevos seguidores',
    topMembers: 'Top miembros',
    topCoinChests: 'Top cofres de monedas',
    topSharers: 'Top compartidos',
    topLikers: 'Top likes',

    valueBitsOne: '{count} bit',
    valueBitsMany: '{count} bits',
    valueMonthsOne: '{count} mes',
    valueMonthsMany: '{count} meses',
    valueViewersOne: '{count} espectador',
    valueViewersMany: '{count} espectadores',
    valuePointsOne: '{count} punto',
    valuePointsMany: '{count} puntos',
    valuePointsNamed: '{count} {name}',
    valueGiftsOne: '{count} regalo',
    valueGiftsMany: '{count} regalos',
    valueDiamondsOne: '{count} diamante',
    valueDiamondsMany: '{count} diamantes',
    valueCoinsOne: '{count} moneda',
    valueCoinsMany: '{count} monedas',
    valueLikesOne: '{count} like',
    valueLikesMany: '{count} likes',
    valueSharesOne: '{count} vez compartido',
    valueSharesMany: '{count} veces compartido',

    nowPlaying: 'Reproduciendo',
    clipViews: '{views} visualizaciones',
    clipCounter: 'Clip {index}/{total}',

    thanks: '¡Gracias por vernos! ❤️',
    seeYou: '¡Nos vemos en el próximo stream!',
  },

  activity: {
    heading: 'Actividad y eventos',
    empty: 'Aún no hay eventos.',
    modBadge: 'mod',
    someUser: 'un usuario',

    deleted: 'Mensaje eliminado',
    deletedBy: 'Mensaje eliminado por {moderator}',
    cleared: 'Chat borrado',
    clearedBy: 'Chat borrado por {moderator}',
    banned: 'Se baneó a {user}',
    bannedBy: '{moderator} baneó a {user}',
    timedOut: 'Se expulsó temporalmente a {user}',
    timedOutFor: 'Se expulsó a {user} durante {seconds}s',
    timedOutBy: '{moderator} expulsó temporalmente a {user}',
    timedOutForBy: '{moderator} expulsó a {user} durante {seconds}s',
    automodHeld: 'AutoMod retuvo un mensaje de {user}',
    automodHeldCategory: 'AutoMod retuvo un mensaje de {user} ({category})',
    automodResolved: 'Retención de AutoMod: {resolution}',
    automodResolvedBy: 'Retención de AutoMod: {resolution} por {moderator}',
    automodHeldBadge: 'retenido',
  },

  chatPanel: {
    heading: 'Chat',
    empty: 'Aún no hay mensajes de chat.',
    filteredBy: 'Mostrando solo mensajes de {user}',
    showAll: 'Mostrar todo el chat',
    filteredEmpty: 'Aún no hay mensajes de {user}.',
    filteredCount: '{shown} de {total}',
    sharedBadge: 'compartido',
  },

  observability: {
    sources: 'Fuentes ({count})',
    configuredEvents: 'Eventos configurados',
    emotes: 'Emotes',
    filters: 'Filtros',

    noSources: 'No hay fuentes configuradas.',
    sourceLive: 'en directo',
    sourceIdle: 'inactiva',

    eventsUnavailable:
      'La configuración de eventos no está disponible; los eventos aparecerán aquí a medida que lleguen.',
    sevenTvSet: 'Set de 7TV',
    sevenTvDefault: 'predeterminado por fuente',

    bannedWords: 'Palabras prohibidas',
    bannedUsers: 'Usuarios bloqueados',
    minLength: 'Longitud mínima',
    hideCommands: 'Ocultar comandos',
    sayHiFilter: 'Filtro de saludos',
    yes: 'sí',
    no: 'no',
    filtersNote: 'Los filtros se muestran como referencia; esta vista muestra todos los mensajes.',

    twitchOptIns: 'Funciones opcionales de Twitch',
    optInAdBreaks: 'Avisos de pausas publicitarias',
    optInModLog: 'Eventos de moderación y AutoMod',
    optInEnable: 'Activar',

    eventTwitchSubs: 'Subs de Twitch',
    eventTwitchResubs: 'Resubs de Twitch',
    eventTwitchGiftSubs: 'Subs regaladas de Twitch',
    eventTwitchBits: 'Bits de Twitch',
    eventTwitchRaids: 'Raids de Twitch',
    eventTwitchChannelPoints: 'Puntos de canal',
    eventTwitchFollows: 'Follows de Twitch',
    eventTwitchWatchStreaks: 'Watch Streaks',
    eventYoutubeSuperChat: 'YouTube Super Chat',
    eventYoutubeSuperSticker: 'Super Sticker',
    eventYoutubeMembers: 'Miembros de YouTube',
    eventYoutubeMemberMilestones: 'Hitos de miembros',
    eventYoutubeMemberGifts: 'Regalos de membresía',
    eventYoutubeSubscribers: 'Suscriptores de YouTube',
    eventKickSubs: 'Subs de Kick',
    eventKickGifts: 'Regalos de Kick',
    eventKickHosts: 'Hosts de Kick',
    eventKickFollows: 'Follows de Kick',
    eventTiktokLikes: 'Likes de TikTok',
    eventTiktokGifts: 'Regalos de TikTok',
    eventTiktokFollows: 'Follows de TikTok',
    eventTiktokShares: 'Compartidos de TikTok',
    eventTiktokTreasureChests: 'Cofres de monedas de TikTok',
    eventTiktokSubscriptions: 'Subs de TikTok',
    eventTiktokSuperFans: 'TikTok Super Fans',
    eventTiktokQuestions: 'Preguntas de TikTok',
    eventTokenWarnings: 'Avisos de token',
  },

  viewSettings: {
    buttonLabel: 'Ajustes de visualización',
    buttonText: 'Visualización',
    heading: 'Ajustes de vista',
    chatOrderHeading: 'Orden del chat',
    activitySoundHeading: 'Sonido de actividad',

    showAvatars: 'Avatares',
    showBadges: 'Insignias',
    showPronouns: 'Pronombres',
    showTimestamps: 'Marcas de tiempo',
    showPlatformGlyph: 'Icono de plataforma',
    showModeration: 'Controles de moderación',

    newestFirst: 'Mensajes más recientes primero',
    newestFirstNote:
      'Coloca el mensaje más reciente en la parte superior del panel de Chat, para que puedas leer el chat sin bajar la vista. Solo afecta a este navegador.',

    activitySoundEnabled: 'Sonido con actividad nueva',
    soundPresetLabel: 'Sonido',
    volume: 'Volumen',
    testSound: 'Probar sonido',
    activitySoundNote:
      'Suena solo aquí, en este navegador, para que no se te escape actividad fácil de pasar por alto, como canjes de puntos de canal o una Rosa de TikTok. Es independiente de los sonidos de notificación en stream de tu overlay.',
  },

  chatSend: {
    formLabel: 'Enviar un mensaje de chat',
    targetGroupLabel: 'Enviar a',
    sendToPlatform: 'Enviar a {platform}',
    enableSendingFor: 'Activar el envío para {platform}',
    allLabel: 'Enviar a todas las plataformas',
    allText: 'Todas',
    messageLabel: 'Mensaje de chat',
    placeholderAll: 'Mensaje a todas las plataformas…',
    placeholderOne: 'Envía un mensaje…',
    send: 'Enviar',

    enableSending: 'Activar el envío',
    reconnect: 'Reconectar',
    enablePlatform: 'Activar {platform}',
    reconnectPlatform: 'Reconectar {platform}',

    sendFailed: 'No se pudo enviar. Inténtalo de nuevo.',
    missingScope: 'El envío aún no está activado.',
    missingScopeFor: 'El envío aún no está activado para {platform}.',
    reauthRequired: 'Tu sesión de {platform} ha caducado. Vuelve a conectar.',
    reauthRequiredGeneric: 'Tu sesión de la plataforma ha caducado. Vuelve a conectar.',
    rateLimitedRetry: 'Límite de frecuencia alcanzado: inténtalo de nuevo en {seconds}s.',
    rateLimited: 'Límite de frecuencia alcanzado: ve más despacio.',
    streamOffline: 'Ese canal no está en directo ahora mismo.',

    sent: 'Enviado ✓',
    resultOk: '{platform} ✓',
    resultFailed: '{platform} ✗',
    resultFailedWhy: '{platform} ✗ {why}',

    reasonReauthRequired: 'reconectar',
    reasonMissingScope: 'bloqueado',
    reasonStreamOffline: 'sin directo',
    reasonQuotaExhausted: 'cuota',
    reasonSendFailed: 'falló',
    reasonMessageDropped: 'bloqueado por los ajustes del chat',
  },

  moderationControls: {
    menuLabel: 'Moderar usuario',
    timeout: 'Expulsar temporalmente',
    ban: 'Banear usuario',
    unban: 'Desbanear usuario',
    deleteMessage: 'Eliminar mensaje',

    noModerationApi: '{platform} no tiene API de moderación',
    unavailable: 'La moderación no está disponible para esta fuente',
    missingScope: 'Concede permisos de moderación para activar las acciones de mod',
    needsDiscordLink: 'Vincula tu cuenta de Discord para moderar aquí',
    ownerChannelUnverified:
      'La cuenta de Discord de este streamer no está conectada, así que aquí no se puede moderar nada',
    botMissingPermission:
      'El bot de All-Chat no recibió este permiso de Discord: pide al streamer que lo vuelva a invitar',
  },

  layoutPicker: {
    groupLabel: 'Disposición de los paneles',
    chatLeft: 'Chat a la izquierda, eventos a la derecha',
    chatRight: 'Chat a la derecha, eventos a la izquierda',
    chatTop: 'Chat arriba, eventos abajo',
    eventsTop: 'Eventos arriba, chat abajo',
  },

  platformGlyph: {
    groupLabel: 'Plataformas: {platforms}',
  },

  monitor: {
    details: 'Detalles',
    engagement: 'Interacción',
    engagementTitle: 'Lanza encuestas y predicciones para este overlay',
    rediscoverYouTube: 'Redescubrir YouTube',
    rediscoverYouTubeTitle:
      'Fuerza a YouTube a volver a detectar el directo: úsalo si el chat se detuvo tras un fallo o un reinicio del stream',
    obsOverlay: 'Overlay de OBS',

    stillReconnecting:
      'Sigue reconectando: se recupera solo, y los mensajes enviados mientras tanto se reenvían cuando vuelve la conexión. Si cierras esta página, se pierden.',
    replayTruncated:
      'Puede que falten algunos mensajes anteriores: la desconexión duró más que el búfer de reenvío, así que la parte más antigua del hueco no se pudo recuperar.',

    noRole: 'Puedes ver este monitor, pero no moderas aquí: la moderación está desactivada.',

    featureGatedOwner: 'La moderación del chat es una función premium.',
    featureGatedUpgrade: 'Hazte premium para moderar desde tu overlay',
    featureGatedModerator:
      'El plan de este streamer no incluye la moderación ahora mismo, así que tus acciones no estarán disponibles hasta que lo renueve.',

    needsConsent: 'Conecta tu propia cuenta de {platform} para moderar.',
    needsConsentChannel: 'Conecta tu propia cuenta de {platform} para moderar {channel}.',
    connectPlatform: 'Conectar {platform}',

    needsDiscordLink:
      'Vincula tu cuenta de Discord para moderar Discord aquí: All-Chat comprueba tus propios permisos en el servidor antes de actuar.',
    linkDiscord: 'Vincular Discord',

    missingScope: 'Concede permisos de moderación para activar las acciones de mod en {platform}.',
    missingScopeChannel:
      'Concede permisos de moderación para activar las acciones de mod en {platform} ({channel}).',
    missingScopeDiscord:
      'Vuelve a invitar al bot con permisos de moderación para activar las acciones de mod en {platform}.',
    missingScopeDiscordChannel:
      'Vuelve a invitar al bot con permisos de moderación para activar las acciones de mod en {platform} ({channel}).',
    reinviteBot: 'Volver a invitar al bot',
    enableModeration: 'Activar moderación y envío al chat',
    comingSoonFor: '(próximamente para {platform})',

    modLogOptIn:
      'Muestra las acciones de moderación de Twitch y las retenciones de AutoMod en este feed de actividad. Twitch exige un permiso de «gestión» de AutoMod para enviarnos siquiera los mensajes retenidos: All-Chat solo los lee; todavía no hay botones para aprobar o rechazar.',
    enableModLog: 'Mostrar eventos de moderación y AutoMod',

    adBreaksOptIn:
      'Recibe un aviso en este feed de actividad cuando empiece una pausa publicitaria de Twitch, programada o iniciada a mano, para que puedas pausar lo que estés jugando.',
    enableAdBreaks: 'Mostrar pausas publicitarias',

    reauthOwner:
      'Tu permiso de moderación de {platform} caducó o nunca se concedió: vuelve a autorizar para seguir moderando desde tu overlay.',
    reauthModerator:
      'Tu permiso de moderación de {platform} caducó o nunca se concedió: vuelve a autorizar para seguir moderando aquí.',
    reconnectPlatform: 'Reconectar {platform}',
    reauthorizeModeration: 'Volver a autorizar moderación y envío al chat',

    consentStartFailed: 'No se pudo iniciar la configuración de la moderación. Inténtalo de nuevo.',
    twitchConsentStartFailed: 'No se pudo iniciar la autorización de Twitch. Inténtalo de nuevo.',
    modConnectUnavailable:
      'Conectar {platform} todavía no está disponible. Por ahora, pide al streamer que modere allí.',
    discordLinkUnavailable:
      'Vincular Discord no está disponible en este momento. Por ahora, pide al streamer que modere allí.',
    reloginStartFailed: 'No se pudo volver a iniciar sesión. Inténtalo de nuevo.',

    rediscoverStarted: 'Redescubriendo el stream de YouTube…',
    rediscoverRateLimited: 'Espera un momento antes de volver a intentarlo',
    rediscoverForbidden: 'No tienes autorización para este overlay',
    rediscoverFailed: 'No se pudo iniciar la nueva detección',

    reauthNeededToast: '{platform} necesita que vuelvas a autorizar la moderación',
    actionFailed: 'La acción de moderación falló',
    messageDeleted: 'Mensaje eliminado',
    timedOut: 'Se expulsó temporalmente a {name}',
    banned: 'Se baneó a {name}',
    unbanned: 'Se desbaneó a {name}',
    unbanFailed: 'No se pudo desbanear',
    unnamedTarget: 'un usuario',

    connectRequired: 'Conecta tu propia cuenta de {platform} para moderar aquí',
    ownerChannelUnverifiedModerator:
      'La cuenta de {platform} de este streamer no está conectada, así que aquí no se puede moderar nada',
    ownerChannelUnverifiedOwner:
      'Tu cuenta de {platform} no está conectada para este canal: vuelve a conectarla para moderar aquí',
    delegationUnsupported:
      'Los moderadores todavía no pueden actuar en {platform}: pide al streamer que se encargue de esto',
    targetNotActionable:
      '{platform} no permite que nadie modere a esta persona: es quien posee el canal u otro moderador',
    discordLinkRequired: 'Vincula tu cuenta de Discord para moderar aquí',
    modNotInGuild: 'No estás en este servidor de Discord: pide al streamer que te invite',
    modLacksPermission:
      'Tus roles de Discord no permiten esto: pide al streamer un rol que sí lo permita',
    modBelowTarget:
      'La jerarquía de roles de Discord lo impide: tu rol más alto tiene que estar por encima del suyo',
    botMissingPermission:
      'El bot de All-Chat no recibió este permiso de Discord: pide al streamer que lo vuelva a invitar',
  },

  dock: {
    productName: 'All-Chat',
    signInExplanation:
      'Este panel tiene su propio inicio de sesión, independiente del de tu navegador. Inicia sesión una vez y se mantendrá abierta.',
    signInWith: 'Iniciar sesión con {platform}',
    signInFailed: 'No se pudo iniciar sesión',
    signInFailedBody: 'No se pudo iniciar sesión con {platform}. Inténtalo de nuevo.',

    chatTab: 'Chat',
    activityTab: 'Actividad',
    menuLabel: 'Controles del monitor',
    noticesLabel: 'Avisos del monitor',
    noticesSummary: '{count} avisos',
  },

  eventNotice: {
    tokenExpired: 'El token de OAuth ha caducado',
    tokenRefreshFailed: 'No se pudo renovar el token de OAuth',
    tokenExpiredFor: 'El token de OAuth de {username} ha caducado',
    tokenRefreshFailedFor: 'No se pudo renovar el token de OAuth de {username}',
    tokenRemedy: '→ Vuelve a conectar tu cuenta en Ajustes → Conexiones',

    channelInaccessible: 'No se puede acceder al canal {channel}',
    channelRemedy:
      '→ Concede al bot el permiso "Ver canal" en los ajustes de tu servidor de Discord',

    listenerDeprecated: 'La antigua conexión de chat de Twitch se va a retirar.',
    listenerRemedy: '→ Vuelve a añadir tu fuente de Twitch para pasar a la nueva conexión EventSub',
  },
  customAlertFrameTitle: 'Alerta personalizada',
  goalTitles: {
    followers: 'Meta de seguidores',
    subscribers: 'Meta de suscriptores',
    bits: 'Meta de Bits',
    super_chat: 'Meta de Super Chat',
  },
  goal: {
    progress: '{current} / {target}',
  },
  listTitles: {
    bits: 'Top cheers',
    gifts: 'Top regaladores',
    super_chat: 'Top Super Chats',
  },
  list: {
    empty: 'Aún no hay apoyos',
  },
  statusIndicator: {
    active: '{platform} - {channel} (Activo)',
    inactive: '{platform} - {channel} (Inactivo)',
    connected: '{platform} - {channel} (Conectado)',
    reconnecting: '{platform} - {channel} - Reconectando en {seconds}s',
    reconnectingWithError: '{platform} - {channel} - {error} (reintento en {seconds}s)',
    quotaExceeded: '{platform} - {channel} - Cuota superada',
    error: '{platform} - {channel} - Error',
    discoveryPaused:
      '{platform} - {channel} - Detección en pausa (usa el monitor de chat para reintentar)',
    authRequired: '{platform} - {channel} - Requiere autorización',
    offline: '{platform} - {channel} - Sin conexión',
    withErrorMessage: '{platform} - {channel} - {error}',
    countdownSeconds: '{seconds}s',
  },
}
