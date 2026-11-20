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
    columnField: 'Campo',
    columnType: 'Tipo',
    columnDescription: 'Descripción',
  },
  prose: {
    codeSample: 'Ejemplo de código',
  },
  api: {
    eyebrow: 'API para desarrolladores',
    heading: 'Referencia de la API para desarrolladores',
    intro:
      'Lee el flujo de chat unificado a través de un único WebSocket para crear bots, herramientas de moderación, contadores de votos, analíticas, alertas o lo que quieras.',
    guidePrompt: '¿Solo estás configurando tu overlay? {guide}',
    guideLinkText: 'Ver la guía para streamers →',
    tocHeading: 'En esta página',
    tocConnectATool: 'Conectar una herramienta',
    tocMessageFormat: 'Formato de los mensajes',
    tocChatMessages: 'Mensajes de chat',
    tocEvents: 'Eventos',
    tocStatusMessages: 'Mensajes de estado y control',
    tocReconnecting: 'Reconexión y heartbeats',
    tocTryItNow: 'Pruébalo ahora (sin configuración)',
    connectHeading: 'Conectar una herramienta',
    connectBody:
      'All-Chat reúne el chat en directo de {twitch}, {youtube}, {kick}, {tiktok} y {discord} en un único flujo normalizado: cada mensaje, además de eventos de plataforma como subs, bits, raids, super chats y regalos, en un solo formato y con los emotes de 7TV/BTTV/FFZ ya resueltos. El mismo flujo que alimenta el overlay del navegador es el que lee tu herramienta.',
    connectAnonymous:
      'Abre un WebSocket al endpoint del overlay. La lectura del flujo es {anonymous}: no hace falta token ni cuenta (es el mismo "modo OBS" que usa el overlay del navegador):',
    connectAnonymousEmphasis: 'anónima',
    connectOverlayId:
      'Tu {field} es el UUID que aparece en la URL de fuente de navegador de tu overlay. ¿Quieres probar antes sin cuenta? Usa el {testOverlay}.',
    connectTestOverlayLinkText: 'overlay público de pruebas',
    queryParamsHeading: 'Parámetros de consulta',
    exampleJavascriptHeading: 'Ejemplo mínimo (JavaScript)',
    examplePythonHeading: 'Ejemplo mínimo (Python)',
    formatHeading: 'Formato de los mensajes',
    formatBody: 'Cada trama es JSON con el mismo sobre:',
    formatTypeList: '{field} es uno de estos valores:',
    chatHeading: 'Mensajes de chat',
    chatBody: 'Para {chat} y {update}, {data} es el objeto de mensaje unificado:',
    userHeading: 'user',
    messageHeading: 'message, emotes y attachments',
    messageBody: '{field} es {shape}. Cada {emote}:',
    attachmentsBody:
      '{field} solo está presente cuando un mensaje lleva contenido multimedia (por ahora, imágenes, GIF y vídeos subidos a Discord y vistas previas de enlaces de Tenor/Giphy). Cada {attachment}:',
    badgeBody: 'Cada {badge} es {shape}, p. ej. {example}.',
    exampleHeading: 'Ejemplo',
    eventsHeading: 'Eventos',
    eventsBody:
      'Los eventos de plataforma (subs, bits, raids, donaciones, …) llegan como tramas {chat} cuyo {data} incluye un objeto {event}. El chat normal no tiene el campo {event}.',
    eventTypesHeading: 'Tipos de evento por plataforma',
    eventTypesLabel: '{platform}:',
    exampleSubscriptionHeading: 'Ejemplo (suscripción de Twitch)',
    statusHeading: 'Mensajes de estado y control',
    statusBody:
      'Al conectar, el servidor envía una trama {connected} y después una trama {status} por cada fuente configurada, para que tu interfaz pueda mostrar indicadores de inmediato. {status} solo se entrega por el WebSocket del overlay: la ruta anónima de espectador nunca la recibe. Datos de {status}:',
    reconnectHeading: 'Reconexión y heartbeats',
    reconnectPing:
      'El servidor envía {ping} periódicamente. Responde con {pong}. (La mayoría de las bibliotecas de WebSocket también responden automáticamente a las tramas ping de bajo nivel.)',
    reconnectBackfill:
      'Si el socket se cae, reconecta con espera progresiva (backoff). Para recuperar los mensajes perdidos durante una desconexión breve, reconecta con {since} ajustado a justo antes de perder la conexión: el servidor reenvía los mensajes del búfer posteriores a esa marca de tiempo.',
    reconnectDedup:
      'Usa los ID como clave de deduplicación al combinar el búfer de reenvío con los mensajes en directo.',
    tryItHeading: 'Pruébalo ahora (sin configuración)',
    tryItBody:
      'Hay un overlay público de pruebas que emite tráfico falso realista (chat aleatorio, votos de encuesta con los mensajes literales {one}, {two}, {three}, {four} y eventos de plataforma) para que puedas crear y validar una integración sin cuenta ni canal real. Solo tienes que conectarte; el tráfico fluye mientras haya un cliente conectado y se detiene cuando se desconecta el último:',
    tryItOutro:
      'Pon esa URL en cualquiera de los ejemplos de arriba y verás llegar mensajes y eventos al instante.',
    footerCopyright: '© {year} All-Chat',
    footerGuideLink: 'Guía para streamers',
    footerPrivacyLink: 'Política de privacidad',
    footerTermsLink: 'Términos del servicio',
  },
  apiFields: {
    queryParamsToken:
      'Token del propietario. Solo hace falta para el acceso con permisos de propietario; omítelo para un consumo de solo lectura.',
    queryParamsSince:
      'Milisegundos desde la época Unix. Al conectar, el servidor reenvía los mensajes del búfer posteriores a esta marca de tiempo para que un cliente que reconecta pueda rellenar el hueco. Omítelo para empezar en directo.',
    envelopeType: 'Tipo de mensaje (ver más abajo).',
    envelopeData: 'Carga útil; su forma depende del tipo. Se omite en ping/pong.',
    envelopeTimestamp: 'Marca de tiempo RFC 3339 del momento en que el gateway envió la trama.',
    messageTypesChatMessage:
      'Un mensaje de chat o un evento de plataforma. data es el objeto de mensaje unificado.',
    messageTypesMessageUpdate:
      'Una actualización de un mensaje enviado antes: un agregado de likes de TikTok (se empareja por event.aggregation_id) o un mensaje de chat cuyo contenido multimedia llegó tarde, p. ej. la vista previa de un enlace GIF de Discord (se empareja por id; ignórala si ese mensaje ya no se muestra). Misma forma de data que chat_message.',
    messageTypesConnected: 'Se envía una vez al conectar: { overlay_id, message }.',
    messageTypesPlatformStatus: 'Estado de conexión de una plataforma de origen.',
    messageTypesPing: 'Heartbeat del servidor. Responde con { "type": "pong" }.',
    messageTypesError: 'Aviso de error: { code, message }.',
    chatMessageId: 'ID único del mensaje.',
    chatMessageOverlayId: 'Overlay al que se entregó este mensaje.',
    chatMessagePlatform: '"twitch" | "youtube" | "kick" | "tiktok" | "discord".',
    chatMessageChannelId: 'Identificador del canal en la plataforma.',
    chatMessageChannelName: 'Nombre legible del canal.',
    chatMessageUser: 'Datos del autor (ver más abajo).',
    chatMessageMessage: '{ text, emotes[], attachments[]? } (ver más abajo).',
    chatMessageTimestamp: 'Hora del mensaje en RFC 3339 (UTC).',
    chatMessageMetadata: 'Extras de formato libre, específicos de cada plataforma.',
    chatMessageEvent:
      'Solo está presente cuando el mensaje es un evento de plataforma (ver Eventos). Ausente en el chat normal.',
    userId: 'ID del usuario en la plataforma.',
    userUsername: 'Login/handle.',
    userDisplayName: 'Nombre visible.',
    userColor: 'Color del nombre, en hexadecimal (p. ej. "#FF0000").',
    userBadges: 'Insignias del autor (ver más abajo).',
    userAvatarUrl: 'URL de la imagen de perfil, si está disponible.',
    userPronouns: 'p. ej. "she/her", si se conocen.',
    userNameGradient: 'Descriptor de degradado opcional (cadena JSON).',
    userSourceBadgesSourceUserId:
      'Identidad del canal de origen en los mensajes de chat compartido.',
    userAvatarFrameUrlAvatarFlairUrl: 'Marco/flair cosmético, si está definido.',
    emoteCode: 'Token de texto del emote, p. ej. "Kappa".',
    emoteProvider: '"twitch" | "7tv" | "bttv" | "ffz" | "discord" | plataforma.',
    emoteUrl: 'URL de la imagen en la CDN.',
    emotePositions:
      'Array de pares de índices [inicio, fin] dentro de text donde aparece el emote.',
    attachmentType: '"image" o "video". Los GIF son imágenes animadas.',
    attachmentUrl: 'URL del contenido multimedia.',
    attachmentContentType: 'Tipo MIME, p. ej. "image/gif", "video/mp4".',
    attachmentWidthHeight: 'Dimensiones intrínsecas en píxeles, si se conocen.',
    attachmentThumbUrl: 'Fotograma de póster para vídeos, si está disponible.',
    attachmentSpoiler: 'True cuando quien lo envió marcó el contenido como spoiler.',
    attachmentFilename: 'Nombre de archivo original (se usa para el texto alternativo).',
    eventType: 'Tipo de evento (ver la lista más abajo).',
    eventTier: 'Relevancia relativa: "high" | "medium" | "low".',
    eventValue:
      '{ amount: number, currency: string, display_text: string }, p. ej. amount 250, currency "bits", display_text "250 bits".',
    eventDuration: 'Tiempo sugerido en pantalla, en segundos.',
    eventIsUpdate:
      'true cuando actualiza un evento anterior (p. ej. los agregados de likes de TikTok, entregados como message_update).',
    eventAggregationId: 'Agrupa las actualizaciones sucesivas del mismo agregado.',
    eventMetadata: 'Campos en bruto específicos del evento.',
    platformStatusPlatform: 'Plataforma de origen.',
    platformStatusChannelId: 'Canal de origen.',
    platformStatusChannelName: 'Nombre legible del canal.',
    platformStatusStatus: '"connected" | "reconnecting" | "offline" | "quota_exceeded".',
    platformStatusNextRetryAt:
      'Hora RFC 3339 del próximo intento de reconexión, cuando corresponda.',
    platformStatusErrorMessage: 'Detalle legible cuando el servicio está degradado.',
  },
  guide: {
    eyebrow: 'All-Chat Docs',
    heading: 'Guía para streamers',
    intro:
      'Configúralo en OBS, lanza encuestas y predicciones, modera desde el monitor de chat y hazlo tuyo.',
    apiPrompt: '¿Estás creando un bot, una caja de alertas o una herramienta de analíticas? {api}',
    apiLinkText: 'Ver la API para desarrolladores →',
    tocHeading: 'En esta página',
    tocWhatIsAllChat: 'Qué es All-Chat',
    tocGettingStarted: 'Pon tu overlay en directo',
    tocIrl: 'Streams 24/7 e IRL',
    tocTiktok: 'TikTok: cómo funciona y sus límites',
    tocMonitor: 'El monitor de chat',
    tocModeration: 'Modera tu chat',
    tocEngagement: 'Encuestas, predicciones y puntos',
    tocEventsCredits: 'Eventos y créditos finales',
    tocAlerts: 'Overlays de alertas',
    tocCommands: 'Comandos de chat y temporizadores',
    tocSharing: 'Compartir un overlay',
    tocThemes: 'Elige un tema',
    tocCustomize: 'Hazlo tuyo',
    tocCustomCss: 'Ve más allá con CSS',
    tocFonts: 'Fuentes personalizadas',
    tocPremium: 'Premium',
    whatIsHeading: 'Qué es All-Chat',
    whatIsBody:
      'All-Chat reúne tu chat en directo de {twitch}, {youtube}, {kick}, {tiktok} y {discord} en un único overlay que añades a OBS. Todos los mensajes llegan a un solo feed, junto con eventos como subs, bits, raids, super chats y regalos, y los emotes de 7TV, BTTV y FFZ aparecen automáticamente. Sin bots que invitar ni widgets de chat que configurar.',
    startHeading: 'Pon tu overlay en directo',
    startSignIn: 'Inicia sesión en {home} con tu cuenta de Twitch, YouTube o Kick.',
    startSignInLinkText: 'allch.at',
    startCreate:
      'En el panel, crea un {overlay} y conecta las plataformas en las que haces stream. Cada plataforma conectada se convierte en una {source} de ese overlay: combina tantas como quieras.',
    startCreateOverlayEmphasis: 'overlay',
    startCreateSourceEmphasis: 'fuente de chat',
    startObs:
      'Copia la URL de fuente de navegador del overlay y añádela a OBS como {browserSource}. El chat aparece en cuanto el overlay se conecta.',
    startObsEmphasis: 'Fuente de navegador',
    startDemandDriven:
      'Las fuentes funcionan bajo demanda: All-Chat empieza a escuchar cuando el overlay está abierto y se detiene cuando no hay nada conectado, así que nunca pagas por listeners inactivos.',
    startMultipleOverlays:
      'Una misma cuenta puede tener varios overlays, cada uno con su propia combinación de fuentes de chat, su propio tema y su propia URL para OBS. Por ejemplo, un overlay para Twitch y TikTok en un stream de juegos y otro solo de YouTube para un pódcast.',
    irlHeading: 'Streams 24/7 e IRL',
    irlIntro:
      'Tener una instancia de OBS funcionando las 24 horas, algo habitual entre streamers de IRL que quieren protección contra desconexiones, requiere un paso adicional para que el chat de YouTube se comporte bien. Añade {passiveParam} a la URL de fuente de navegador de tu overlay:',
    irlExplainer:
      'Un overlay {passive} muestra el chat exactamente igual que uno normal, pero no le pide a All-Chat que empiece a capturar por su cuenta. Esto importa porque la detección de YouTube se rinde tras una hora sin encontrar un directo (así nunca bombardea a YouTube por un canal desconectado). Un overlay 24/7 normal agotaría ese tiempo de espera mientras estás desconectado y estaría en pausa cuando empezaras el directo; un overlay pasivo nunca pone en marcha ese reloj, así que nada se queda atascado.',
    irlExplainerEmphasis: 'pasivo',
    irlWhenLiveHeading: 'Cuando empieces el directo',
    irlStepPassiveUrl: 'Deja la fuente de navegador de tu OBS 24/7 con la URL pasiva.',
    irlStepOpenMonitor: 'Abre el {monitor} de tu overlay (la página de monitor / vista).',
    irlStepOpenMonitorEmphasis: 'monitor de chat',
    irlStepRediscover:
      'Si el chat todavía no fluye, pulsa {rediscover} en el monitor: la captura empieza en cuestión de un minuto.',
    irlStepRediscoverEmphasis: 'Redescubrir',
    irlStepKeepOpen:
      'Mantén el monitor de chat abierto mientras haces stream; así la captura sigue activa durante la sesión y se detiene unos minutos después de cerrarlo.',
    irlRefreshNote:
      'Una simple actualización de la fuente de navegador {negation} reinicia un canal de YouTube en pausa: usa el botón {rediscover} del monitor. Mientras un canal está en pausa, su punto de plataforma muestra un estado {paused} (a la espera de que lo actives), no un error en rojo.',
    irlRefreshNoteNegationEmphasis: 'no',
    irlRefreshNotePausedEmphasis: '«en pausa» de color índigo',
    tiktokHeading: 'TikTok: cómo funciona y sus límites',
    tiktokNoApi:
      'TikTok no tiene una API pública para el chat de TikTok LIVE. Todas las herramientas que muestran el chat de TikTok LIVE, All-Chat incluido, lo leen igual que lo hace el reproductor web del propio TikTok. Añade una fuente de TikTok por nombre de usuario y el chat aparecerá mientras estés en LIVE.',
    tiktokSigner:
      'Esa conexión tiene que ir firmada. All-Chat la firma con su propio servicio de firma de código abierto; no interviene ningún servidor de firma de terceros. Cuando TikTok pasó a comprobaciones de sesión propias de un navegador el 2026-09-09, All-Chat se adaptó al cambio esa misma semana. El diseño y sus actualizaciones están documentados en {adr}.',
    tiktokAdrLinkText: 'ADR-0052 en GitHub',
    tiktokFallback:
      'Las salas premium cuentan con una segunda vía de entrega, que se usa cuando TikTok rechaza la conexión principal.',
    tiktokLimit:
      '{limit} TikTok es la plataforma con más probabilidades de sufrir interrupciones breves cuando TikTok cambia su reproductor web. TikTok tampoco tiene API de envío ni de moderación, así que el monitor no puede responder ni moderar allí.',
    tiktokLimitEmphasis: 'El límite:',
    monitorHeading: 'El monitor de chat',
    monitorIntro:
      'Abre la {monitorView} desde el editor del overlay para tener una sala de control en directo, independiente del overlay de OBS. Muestra todos los mensajes en un panel con un feed de actividad al lado, y es donde envías mensajes y moderas.',
    monitorIntroEmphasis: 'Vista de monitor',
    monitorSend:
      '{send} con tu propia cuenta a Twitch, YouTube o Kick directamente desde el monitor (TikTok y Discord no tienen API de envío).',
    monitorSendEmphasis: 'Envía mensajes',
    monitorRediscover:
      '{rediscover} fuerza una nueva búsqueda de tu directo: úsalo si el chat de YouTube se detiene tras un fallo o un reinicio, o para iniciar la captura de un {passiveOverlay} cuando empieces el directo.',
    monitorRediscoverEmphasis: 'Redescubrir YouTube',
    monitorPassiveOverlayEmphasis: 'overlay pasivo 24/7',
    monitorDisplay:
      'Los ajustes de {display} controlan lo que ves (avatares, insignias, pronombres, marcas de tiempo, iconos de plataforma, controles de moderación). Son tus preferencias personales del monitor y no cambian el overlay de OBS.',
    monitorDisplayEmphasis: 'Visualización',
    monitorDockHeading: 'Acopla el monitor en OBS',
    monitorDockIntro:
      'El monitor también funciona como panel de navegador personalizado de OBS, así el chat queda junto a tu mezclador en lugar de en una segunda ventana. En el editor del overlay, haz clic en {copyDock}. Copia el enlace del monitor con el modo panel activado:',
    monitorDockSteps:
      'En OBS, abre {menu}, ponle un nombre al panel, pega el enlace y haz clic en Aplicar. El panel tiene su propio perfil de navegador, así que te pedirá iniciar sesión una vez dentro del panel. Streamlabs Desktop acepta el mismo enlace como panel de navegador personalizado.',
    monitorDockMenu: 'Vista > Paneles > Paneles de navegador personalizados',
    moderationHeading: 'Modera tu chat',
    moderationBody:
      'Con los {controls} activados en el monitor, pasa el cursor sobre un mensaje para usar {del}, o abre un usuario del chat para usar {timeout}, {ban} o {unban}. La acción se aplica en la plataforma de origen: Twitch, Kick y Discord permiten eliminar, expulsar temporalmente, banear y desbanear; YouTube permite expulsar temporalmente y banear; TikTok no tiene API de moderación.',
    moderationControlsEmphasis: 'Controles de moderación',
    moderationDeleteEmphasis: 'Eliminar',
    moderationTimeoutEmphasis: 'Expulsión temporal',
    moderationBanEmphasis: 'Banear',
    moderationUnbanEmphasis: 'Desbanear',
    moderationEnable:
      'La primera vez, usa {enable} para conceder los permisos adicionales (en Discord tienes que volver a invitar al bot). Moderar desde tu overlay es una función {premium}.',
    moderationEnableEmphasis: 'Activar moderación y envío al chat',
    engagementHeading: 'Encuestas, predicciones y puntos',
    engagementIntro:
      'All-Chat tiene sus propias {polls}, {predictions} y {points} que funcionan en todas las plataformas conectadas, no solo en Twitch.',
    engagementPollsEmphasis: 'encuestas',
    engagementPredictionsEmphasis: 'predicciones',
    engagementPointsEmphasis: 'puntos de espectador',
    engagementSetup:
      'Actívalos en la sección {section} del editor: {enablePoints}, define tu {pointsName} y elige cómo se ganan los puntos (por bit, por sub, por sub regalada, etc.).',
    engagementSectionEmphasis: 'Interacción',
    engagementEnablePointsEmphasis: 'Activar puntos de espectador',
    engagementPointsNameEmphasis: 'Nombre de los puntos',
    engagementRun:
      'Lanza rondas en directo desde el panel {section} del monitor: {startPoll} / {closePoll}, y {startPrediction} / {lockWagers} / pagar o {cancelRefund}.',
    engagementStartPollEmphasis: 'Iniciar encuesta',
    engagementClosePollEmphasis: 'Cerrar encuesta',
    engagementStartPredictionEmphasis: 'Iniciar predicción',
    engagementLockWagersEmphasis: 'Bloquear apuestas',
    engagementCancelRefundEmphasis: 'Cancelar y reembolsar',
    engagementJoin:
      'Los espectadores participan desde el chat (p. ej. {voteCommand}, {predictCommand}) o en la {participationPage}.',
    engagementParticipationPageEmphasis: 'Página de participación de espectadores',
    engagementWidgets:
      'Añade el {pollWidget} y el {predictionWidget} como fuentes de navegador independientes para que los resultados se vean en el stream; copia sus URL desde la sección Interacción.',
    engagementPollWidgetEmphasis: 'Widget de encuestas para OBS',
    engagementPredictionWidgetEmphasis: 'Widget de predicciones para OBS',
    eventsHeading: 'Eventos y créditos finales',
    eventsBody:
      'Tu overlay también muestra {events}: subs, resubs, subs regaladas, bits/cheers, raids, Super Chats, membresías y follows. Usa {eventSettings} en el editor para elegir exactamente qué eventos aparecen, por plataforma.',
    eventsEmphasis: 'eventos',
    eventSettingsEmphasis: 'Ajustes de eventos',
    creditsBody:
      'Para el final del stream, abre {credits} y configura unos {creditRoll}: un agradecimiento en scroll a tus principales suscriptores, quienes regalaron subs, quienes enviaron cheers, raiders y nuevos seguidores. Es una fuente de navegador independiente ({copyUrl}).',
    creditsEmphasis: 'Créditos',
    creditRollEmphasis: 'Créditos finales',
    creditsCopyUrlEmphasis: 'Copiar URL de OBS de los créditos',
    alertsHeading: 'Overlays de alertas',
    alertsBody:
      'Crea un overlay de tipo {kind} para tener una caja de alertas como fuente de navegador de OBS propia. Las suscripciones, bits, raids y follows de Twitch, los Super Chats y membresías de YouTube y los regalos de TikTok aparecen con tu texto, tu GIF y tu sonido, o con tu propio HTML, CSS y JavaScript. Los overlays de alertas son gratis; subir tus propios archivos es premium. La {guide} explica la configuración paso a paso.',
    alertsKindEmphasis: 'Alertas',
    alertsGuideLinkText: 'guía de alertas de stream',
    commandsHeading: 'Comandos de chat y temporizadores',
    commandsBody:
      'En {nav}, en la barra superior, defines una sola vez tus !comandos, activadores por palabra clave y temporizadores, y responden en todos los chats que llevan tus overlays. Las respuestas las envía el All-Chat Bot o tu propia cuenta. Son gratis en los chats de Twitch y Kick y premium en el chat de YouTube. La {guide} tiene los detalles.',
    commandsNavEmphasis: 'Comandos',
    commandsGuideLinkText: 'guía del bot de chat',
    sharingHeading: 'Compartir un overlay',
    sharingBody:
      '{share} permite que otro streamer lleve el chat de tu overlay al suyo, algo muy útil para colaboraciones y raids. Envía una solicitud a su nombre de usuario de Twitch; cuando la acepte, tu overlay aparecerá en su editor en {sharedOverlays} para añadirlo como fuente, y cualquiera de los dos puede revocarlo más adelante. Compartir es una función {premium}.',
    sharingShareEmphasis: 'Compartir overlay',
    sharingSharedOverlaysEmphasis: 'Overlays compartidos',
    themesHeading: 'Elige un tema',
    themesIntro:
      'All-Chat incluye {count}, desde Modern Dark y Minimal hasta Trading Card, Comic Speech, Sticky Notes, Vaporwave, Cyberpunk y más. Aplicar uno es cuestión de un clic y {noCss}.',
    themesCountEmphasis: '16 temas integrados',
    themesNoCssEmphasis: 'no hace falta nada de CSS',
    themesStepOpen: 'Abre tu overlay en el panel para editarlo.',
    themesStepApply:
      'En la sección {theme}, explora los temas y aplica uno. La vista previa se actualiza al instante.',
    themesStepApplyEmphasis: 'Tema',
    themesStepSave:
      'Guarda. Tu fuente de navegador de OBS mostrará el nuevo aspecto la próxima vez que se actualice.',
    themesPreview:
      'También puedes ver todos los temas en directo en la {home} antes de iniciar sesión.',
    themesPreviewLinkText: 'página de inicio',
    customizeHeading: 'Hazlo tuyo',
    customizeIntro:
      'Un tema es un punto de partida: casi siempre puedes conseguir exactamente el aspecto que quieres{noCode}. En el panel {appearance} del editor del overlay puedes ajustar, con deslizadores, interruptores y selectores de color:',
    customizeNoCodeEmphasis: ' sin escribir nada de código',
    customizeAppearanceEmphasis: 'Apariencia',
    customizeFont:
      'Familia tipográfica, tamaño del texto, grosor, altura de línea y espaciado entre letras',
    customizeSpacing: 'Espaciado, relleno y esquinas de las burbujas de mensaje',
    customizeAvatar: 'Tamaño de avatares e insignias',
    customizeVisibility:
      'Qué mostrar u ocultar: avatares, insignias, marcas de tiempo, nombres de usuario',
    customizeColors: 'Colores, incluido un color de nombre por plataforma',
    customizeEvents: 'Cómo aparecen eventos como subs, raids y super chats',
    customizeOutro:
      'Empieza por el tema más parecido a lo que buscas y luego ajusta estas opciones hasta que lo sientas tuyo. Si necesitas algo que el panel no cubre, pasa al CSS de más abajo.',
    cssHeading: 'Ve más allá con CSS',
    cssIntro:
      'Para tener control total, activa {customCss} en la sección {expert} del editor del overlay y escribe tus propios estilos. Algunas cosas que conviene saber:',
    cssCustomCssEmphasis: 'CSS personalizado',
    cssExpertEmphasis: 'Experto',
    cssScope: 'Tu CSS solo afecta a tu overlay, a nada más.',
    cssOrder:
      'Se carga {after} del tema, así que puedes partir de un tema integrado y sobrescribir solo las partes que quieras.',
    cssOrderEmphasis: 'después',
    cssPreview:
      'Hay una vista previa en directo justo al lado del editor, así que ves los cambios mientras escribes.',
    cssVarsHeading: 'Cambios rápidos: variables de estilo',
    cssVarsIntro:
      'La palanca más sencilla no requiere conocer nombres de clases. Define cualquiera de estas variables en {root} y el overlay las aplicará:',
    cssVarsForced:
      'Hay un grupo de variables especial: el grosor de la fuente, las esquinas de la burbuja, la altura de línea, el espaciado entre letras, el color del mensaje y la familia tipográfica se leen de estas variables desde los propios estilos del overlay, así que un valor que definas aquí también prevalece sobre el deslizador del panel de apariencia.',
    cssVarsColumnVariable: 'Variable',
    cssVarsColumnDefault: 'Valor por defecto',
    cssVarsColumnEffect: 'Qué cambia',
    cssHooksHeading: 'Control más fino: apunta a las partes del chat',
    cssHooksIntro:
      'Para todo lo que las variables no cubren, aplica estilos directamente a los elementos del overlay. Estos nombres de clase y atributos data son estables y puedes usarlos con seguridad:',
    cssHooksColumnSelector: 'Selector',
    cssHooksColumnKind: 'Tipo',
    cssHooksColumnTargets: 'A qué apunta',
    cssFeedAnchor:
      '{feedAnchor} (ajustes de Mensajes) decide en qué {edge} se apoya el feed: ánclalo abajo y los mensajes nuevos empujan a los anteriores hacia arriba, con el espacio vacío acumulándose en la parte superior. {invertOrder} es un ajuste distinto para un eje distinto: solo cambia en qué {endOfList} está lo más reciente. Las cuatro combinaciones funcionan.',
    cssFeedAnchorEmphasis: 'Anclaje del chat',
    cssFeedAnchorEdgeEmphasis: 'borde',
    cssInvertOrderEmphasis: 'Invertir el orden de los mensajes',
    cssEndOfListEmphasis: 'extremo de la lista',
    cssExampleCaption: 'Ejemplo: dale a cada plataforma su propia franja de color:',
    cssCallout:
      '¿Quieres ejemplos ya hechos? Cada tema integrado es simplemente un archivo CSS que puedes leer y tomar prestado en {github}. ¿Ya tienes un tema de otra herramienta o quieres ayuda para escribir uno? Pregunta en nuestro {discord} y te ayudaremos encantados.',
    cssCalloutGithubLinkText: 'GitHub',
    fontsHeading: 'Fuentes personalizadas',
    fontsIntro:
      'Puedes cargar una fuente de Google Fonts con un {importRule} normal y usarla en cualquier parte de tu CSS. Para proteger la privacidad de tus espectadores, las fuentes se sirven a través de All-Chat y no directamente desde Google, así que solo están disponibles estas familias:',
    fontsFamilies:
      'Barlow, Barlow Condensed, Bebas Neue, Exo 2, Inter, Monoton, Montserrat, Nunito, Open Sans, Orbitron, Oswald, Poppins, Press Start 2P, Rajdhani, Roboto, Share Tech Mono, Source Code Pro, Space Grotesk, VT323.',
    fontsOutro:
      'Si pides una familia que no está en la lista, simplemente no se cargará: elige otra o deja la predeterminada.',
    premiumHeading: 'Premium',
    premiumIntro:
      'All-Chat es gratuito y de código abierto. Una suscripción Premium, a través de Patreon y conectada en {settings}, desbloquea:',
    premiumSettingsEmphasis: 'Ajustes → Premium',
    premiumModeration: '{label}: elimina, expulsa temporalmente y banea desde el monitor de chat.',
    premiumModerationEmphasis: 'Modera desde tu overlay',
    premiumTts: '{label}: voces TTS premium (el TTS básico del navegador es gratuito).',
    premiumTtsEmphasis: 'Texto a voz con ElevenLabs',
    premiumYoutube:
      '{label}: elige qué stream seguir en canales con varios directos (la opción gratuita por defecto sigue el primero que encuentra).',
    premiumYoutubeEmphasis: 'Selección de directo de YouTube',
    premiumSharedChat: '{label}: comparte tu overlay con otros streamers.',
    premiumSharedChatEmphasis: 'Chat compartido',
    premiumFlairs: '{label}: colores de nombre con degradado animado.',
    premiumFlairsEmphasis: 'Flairs de espectador',
    footerCopyright: '© {year} All-Chat',
    footerApiLink: 'API para desarrolladores',
    footerPrivacyLink: 'Política de privacidad',
    footerTermsLink: 'Términos del servicio',
  },
  guideCssVars: {
    chatFontSize: 'Tamaño del texto de los mensajes.',
    chatFontWeight: 'Grosor del texto de los mensajes.',
    chatFontFamily: 'Fuente del texto de los mensajes.',
    chatUsernameFontWeight: 'Grosor del nombre de usuario.',
    chatMessageColor: 'Color del texto de los mensajes.',
    chatMessageGap: 'Espacio vertical entre mensajes.',
    chatAvatarGap: 'Espacio entre el avatar y el texto del mensaje.',
    chatBubbleBorderRadius: 'Redondez de la burbuja del mensaje.',
    chatBubblePadding: 'Relleno interior de cada mensaje.',
    chatAvatarSize: 'Ancho y alto del avatar.',
    chatUsernameFontSize: 'Tamaño del nombre de usuario.',
    chatEmoteScale: 'Multiplicador del tamaño de los emotes.',
    chatShowAvatars: 'Ponlo en none para ocultar los avatares.',
    chatShowBadges: 'Ponlo en none para ocultar las insignias.',
    chatShowTimestamps: 'Ponlo en none para ocultar las marcas de tiempo.',
  },
  guideCssHooks: {
    overlayLiveBody: 'El contenedor de toda la lista de mensajes.',
    chatMessage: 'Una burbuja de mensaje de chat.',
    chatUsername: 'El nombre del autor.',
    platformBadge: 'La etiqueta/icono de plataforma junto al nombre.',
    eventMessage: 'Una alerta de sub / raid / super chat / regalo.',
    dataPlatformTwitch: 'Mensajes de una plataforma (twitch, youtube, kick, tiktok, discord).',
    dataUsername: 'Mensajes de un usuario concreto.',
    dataFeedAnchorTopBottom:
      'El contenedor del overlay, que lleva el ajuste Anclaje del chat. Léelo para adaptarte; no sobrescribas el flex-direction del contenedor ni el margin-top de la lista, que son los que mueven el feed.',
    dataFeedOrderNewestLastNewestFirst:
      'El contenedor del overlay, que lleva el ajuste Invertir el orden de los mensajes. También invierte --msg-enter-dir y --msg-enter-origin, para que las animaciones de entrada lleguen desde el extremo donde aparece el mensaje más reciente.',
  },
  themeContrast: {
    heading: 'Banco de pruebas de contraste de temas',
    intro:
      'Solo para desarrollo. Renderiza todos los temas incluidos para la comprobación WCAG del texto de los mensajes. {count} temas.',
  },
}
