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

// Claim nothing the product does not do today; see en/guides.ts.
export const guides: LocaleNamespace<'guides'> = {
  shared: {
    eyebrow: 'Guía',
    ctaHeading: 'Configúralo',
    ctaBody:
      'All-Chat es gratis. Inicia sesión con Twitch, YouTube o Kick, crea un overlay y pega su URL en OBS.',
    ctaSignIn: 'Iniciar sesión',
    ctaDocs: 'Leer la documentación',
    relatedHeading: 'Más guías',
    limitsHeading: 'Límites',
    premiumLinkText: 'Premium',
    tiktokGuideLinkText: 'Guía de TikTok',
    linkObsOverlay: 'Overlay de chat para OBS',
    linkObsDock: 'Dock de chat para OBS',
    linkTiktok: 'Chat de TikTok LIVE en OBS',
    linkMultistream: 'Chat multistream',
    linkCompare: 'Comparativa con otras herramientas de chat',
  },
  index: {
    metaTitle:
      'Guías: overlay de chat para OBS, dock de chat, chat de TikTok LIVE y chat multistream',
    metaDescription:
      'Guías paso a paso de All-Chat: lleva el chat unificado a OBS, acóplalo dentro de OBS, muestra el chat de TikTok LIVE, une el chat multistream y compara herramientas de chat.',
    breadcrumb: 'Guías',
    heading: 'Guías',
    intro:
      'Cómo llevar a OBS el chat unificado de Twitch, YouTube, Kick, TikTok y Discord, y cómo se compara All-Chat con otras herramientas de chat.',
  },
  obsOverlay: {
    metaTitle: 'Overlay de chat para OBS con Twitch, YouTube, Kick y TikTok',
    metaDescription:
      'Una sola fuente de navegador de OBS que muestra juntos los chats de Twitch, YouTube, Kick, TikTok y Discord. La URL, el tamaño, el fondo transparente y los ajustes de la fuente, paso a paso.',
    breadcrumb: 'Overlay de chat para OBS',
    heading: 'Overlay de chat para OBS con Twitch, YouTube, Kick, TikTok y Discord',
    intro:
      'All-Chat une el chat de todas las plataformas en las que haces stream en un solo feed y lo sirve como una página web. OBS muestra esa página como fuente de navegador, así que no hay nada que instalar.',
    stepsHeading: 'Añade el overlay a OBS',
    stepSignIn: 'Inicia sesión en {home} con tu cuenta de Twitch, YouTube o Kick.',
    stepSignInLinkText: 'allch.at',
    stepCreate:
      'En el panel, crea un overlay y conecta las plataformas en las que haces stream. Cada plataforma pasa a ser una fuente de chat de ese overlay.',
    stepCopy: 'Copia la URL del overlay. Tiene la forma {url}.',
    stepAdd:
      'En OBS, haz clic en {plus} debajo de Fuentes, elige {browser} y pega la URL en el campo URL.',
    stepAddPlus: '+',
    stepAddBrowser: 'Navegador',
    stepSize:
      'Ajusta el ancho y el alto al área que debe ocupar el chat, por ejemplo 400 × 800. No le des el tamaño de todo tu lienzo.',
    stepShutdown: 'Decide si marcas {shutdown}. La sección de abajo explica qué implica.',
    stepShutdownEmphasis: 'Apagar la fuente cuando no sea visible',
    stepDone: 'Haz clic en Aceptar. El chat aparece en cuanto el overlay se conecta.',
    backgroundHeading: 'Fondo transparente',
    backgroundBody:
      'La página del overlay tiene el fondo transparente, así que sobre tu escena solo se ven los mensajes. No necesitas un chroma key ni CSS adicional en OBS para esto.',
    shutdownHeading: 'Apagar la fuente cuando no sea visible',
    shutdownBody:
      'All-Chat lee el chat de tus plataformas solo mientras hay un overlay conectado. Con {shutdown} marcado, OBS cierra la página siempre que la fuente no esté en la escena activa. Tras un breve periodo de gracia, All-Chat deja de leer tu chat y vuelve a empezar cuando la fuente es visible.',
    shutdownTradeoff:
      'Márcalo si quieres que la captura del chat se detenga mientras el overlay está oculto. Déjalo sin marcar si cambias de escena a menudo y quieres que el feed ya esté en marcha cuando vuelvas.',
    shutdown247:
      '¿Tienes OBS funcionando las 24 horas para un stream IRL o 24/7? Lee antes la sección {section} de la documentación. Explica una URL de overlay pasiva que evita que la detección de YouTube se pause mientras estás desconectado.',
    shutdown247LinkText: '24/7 e IRL',
    themesHeading: 'Temas y CSS personalizado',
    themesBody:
      'Elige uno de los {themes} en el editor de overlays. La fuente, el espaciado, los colores y los avatares se ajustan en el panel Apariencia sin código. Para tener control total, escribe tus propios estilos con {css}.',
    themesLinkText: '16 temas integrados',
    cssLinkText: 'CSS personalizado',
    platformBadges:
      'Cada mensaje puede mostrar una insignia de la plataforma de la que viene. Actívala en el editor de overlays o dale tu propio estilo a cada plataforma con CSS personalizado.',
    emotesHeading: 'Emotes',
    emotesBody:
      'Los emotes de 7TV, BTTV y FFZ se muestran en el overlay junto a los emotes nativos de Twitch y YouTube. No hay que configurar nada para ellos.',
    limitYoutube:
      'El chat de YouTube funciona por sí solo con los directos públicos. Para un directo oculto, pega su enlace en los ajustes de la fuente. El chat solo para miembros de tu propio canal necesita el modo premium de API oficial. Los directos programados se detectan cuando empiezan.',
    limitTiktokReadOnly:
      'No puedes responder ni moderar el chat de TikTok desde All-Chat. TikTok no ofrece ninguna API para ello.',
    limitRetention:
      'Los mensajes de chat se eliminan de All-Chat al cabo de una hora aproximadamente. El overlay es un feed en directo, no un archivo.',
    limitTiktok:
      'TikTok es la plataforma con más probabilidades de caerse durante un momento. La {guide} explica por qué.',
  },
  obsDock: {
    metaTitle: 'Dock de chat para OBS con Twitch, YouTube, Kick y TikTok',
    metaDescription:
      'Acopla dentro de OBS el chat unificado de Twitch, YouTube, Kick, TikTok y Discord con Vista > Paneles > Paneles de navegador personalizados. Lee, responde, modera y lanza encuestas desde un solo panel.',
    breadcrumb: 'Dock de chat para OBS',
    heading: 'Dock de chat para OBS con Twitch, YouTube, Kick, TikTok y Discord',
    intro:
      'Cada overlay de All-Chat incluye un monitor de chat: los mismos mensajes, dispuestos para que los leas tú en lugar de para que los vean tus espectadores. OBS puede tener el monitor como dock junto a Escenas y Fuentes, así el chat se queda en la ventana desde la que haces stream.',
    urlHeading: 'La URL del dock',
    urlBody:
      'El enlace del dock de un overlay es {url}: el monitor dispuesto para un panel estrecho. En el editor de overlays, haz clic en Copiar URL del dock para copiarlo. Sin ?dock=1, la misma dirección abre el monitor a ancho completo en una pestaña del navegador.',
    stepsHeading: 'Añade el monitor como dock de OBS',
    stepMenu: 'En OBS, abre {menu}.',
    stepMenuPath: 'Vista > Paneles > Paneles de navegador personalizados',
    stepPaste:
      'Escribe un nombre para el dock, pega la URL del dock en el campo URL, haz clic en Aplicar y cierra el diálogo.',
    stepSignIn:
      'Inicia sesión dentro del dock. Los docks de OBS usan su propio perfil de navegador, así que la sesión de tu navegador habitual no se traslada. Solo tienes que hacerlo una vez.',
    stepPlace:
      'Arrastra el panel adonde quieras. Una columna estrecha junto a Escenas y Fuentes funciona bien.',
    streamlabsNote:
      'Streamlabs Desktop también tiene docks de navegador personalizados, y la URL del dock funciona allí igual.',
    featuresHeading: 'Qué puedes hacer desde el dock',
    featureRead: 'Leer todas las plataformas en una sola lista, con un feed de actividad al lado.',
    featureSend:
      'Enviar mensajes con tu propia cuenta a Twitch, YouTube o Kick. TikTok y Discord no tienen API de envío, así que aquí son de solo lectura.',
    featureModerate:
      'Eliminar mensajes, expulsar temporalmente, banear y desbanear en la plataforma de la que viene el mensaje: las cuatro acciones en Twitch, Kick y Discord, expulsión temporal y baneo en YouTube. TikTok no tiene API de moderación. Moderar desde All-Chat es una función {premium}.',
    featureEngage:
      'Abrir y cerrar encuestas, y abrir, bloquear y pagar predicciones. Funcionan en todas las plataformas conectadas y los espectadores participan desde el chat.',
    featureMods:
      'Dar acceso al monitor a tus moderadores. Actúan con sus propias cuentas de plataforma y no necesitan Premium. Añadir moderadores requiere {premium} en tu cuenta.',
    sameFeedHeading: 'El mismo feed que el overlay',
    sameFeedBody:
      'El dock y el overlay de OBS muestran el mismo feed. Los ajustes de visualización del monitor, como avatares, marcas de tiempo e iconos de plataforma, solo cambian tu vista, nunca lo que los espectadores ven en el stream.',
    limitPermissions:
      'La primera vez que envías un mensaje o moderas, el monitor te pide conceder permisos adicionales con {enable}. En Discord eso significa volver a invitar al bot.',
    limitPermissionsEmphasis: 'Activar moderación y envío al chat',
    limitTiktok:
      'El chat de TikTok en el dock es de solo lectura, y TikTok es la plataforma con más probabilidades de caerse durante un momento. La {guide} tiene los detalles.',
    moreDocs:
      'Las secciones {monitor}, {moderation} y {engagement} de la documentación entran en más detalle.',
    moreDocsMonitor: 'monitor de chat',
    moreDocsModeration: 'moderación',
    moreDocsEngagement: 'encuestas y predicciones',
  },
  tiktok: {
    metaTitle: 'Overlay de chat de TikTok LIVE para OBS',
    metaDescription:
      'Muestra el chat de TikTok LIVE en OBS junto a los chats de Twitch, YouTube, Kick y Discord, diseñado para un objetivo de disponibilidad del 99,9 % sin una API oficial de TikTok.',
    breadcrumb: 'Overlay de chat de TikTok LIVE',
    heading: 'Overlay de chat de TikTok LIVE para OBS',
    intro:
      'All-Chat muestra el chat de TikTok LIVE en el mismo overlay de OBS que tus chats de Twitch, YouTube, Kick y Discord. TikTok es la plataforma más difícil de soportar y la menos estable de las que conecta All-Chat, y aun así el chat de TikTok se rige por el mismo objetivo de disponibilidad del 99,9 % que el resto de plataformas. Esta página explica cómo.',
    stepsHeading: 'Añade TikTok a un overlay',
    stepOpen: 'Abre tu overlay en el panel de All-Chat.',
    stepConnect:
      'Haz clic en {connect} e introduce el nombre de usuario de TikTok del creador. No hay ningún paso de inicio de sesión en TikTok.',
    stepConnectEmphasis: 'Conectar TikTok',
    stepLive:
      'Empieza un directo en TikTok. El chat aparece en el overlay en cuanto All-Chat detecta que la cuenta está en directo.',
    stepObs: 'Si el overlay todavía no está en OBS, sigue la {guide}.',
    stepObsLinkText: 'guía del overlay de chat para OBS',
    apiHeading: 'Ninguna herramienta tiene una API oficial del chat de TikTok LIVE',
    apiBody:
      'TikTok no tiene una API pública para el chat de LIVE. Todas las herramientas que muestran el chat de TikTok LIVE, All-Chat incluido, lo leen igual que el reproductor web de TikTok.',
    approachHeading: 'Qué hace All-Chat al respecto',
    approachSigner:
      'All-Chat firma las conexiones con TikTok con su propio {signer}; no interviene ningún servidor de firma de terceros. La decisión y su historia están en {adr}.',
    approachSignerLinkText: 'servicio de firma de código abierto',
    approachAdrLinkText: 'ADR-0052',
    approachSessions:
      'El 2026-09-09 TikTok empezó a exigir sesiones de nivel navegador para los datos del chat en directo. All-Chat siguió el cambio esa misma semana; las actualizaciones están registradas en el mismo ADR.',
    approachFallback:
      'Las salas de cuentas {premium} tienen una segunda vía de entrega cuando TikTok rechaza la conexión principal.',
    approachStory: 'La historia completa está contada en {post}.',
    limitInterruptions:
      'TikTok sigue siendo la plataforma menos estable que soporta All-Chat. Cuando TikTok cambia su reproductor web, todas las herramientas que leen el chat de TikTok LIVE tienen que adaptarse, All-Chat incluido, así que puede haber interrupciones breves. Los chats de Twitch, YouTube, Kick y Discord del mismo overlay siguen funcionando mientras tanto.',
    limitReadOnly:
      'No puedes responder ni moderar el chat de TikTok desde All-Chat. TikTok no ofrece ninguna API para ello.',
    verticalNote:
      '¿Haces stream en vertical a TikTok y en horizontal al resto? La {guide} explica cómo usar un segundo overlay para el lienzo vertical.',
    verticalNoteLinkText: 'guía de chat multistream',
  },
  multistream: {
    metaTitle: 'Chat multistream: Twitch, YouTube, Kick y TikTok en un solo feed',
    metaDescription:
      'Une el chat de todas las plataformas a las que haces multistream en un solo overlay de OBS y un solo panel de moderación. Funciona con SE.Live, Restream, Streamlabs Multistream o varias salidas RTMP.',
    breadcrumb: 'Chat multistream',
    heading: 'Chat multistream: un solo feed para Twitch, YouTube, Kick, TikTok y Discord',
    intro:
      'Haz stream a cuatro plataformas a la vez y el chat llega a cuatro sitios. All-Chat lo une, junto con Discord, en un solo feed para tu overlay de OBS y un solo panel para que leas y moderes.',
    methodHeading: 'Funciona con cualquier configuración de multistream',
    methodBody:
      'All-Chat se conecta directamente al chat de cada plataforma y nunca toca tu vídeo. Da igual cómo salga el vídeo: SE.Live, Restream, Streamlabs Multistream, varias salidas RTMP desde OBS o un segundo PC funcionan igual.',
    methodSources:
      'El chat no depende de que envíes tu stream a través de un servicio concreto. Conecta cada plataforma como fuente de chat de tu overlay y el feed la sigue.',
    platformsHeading: 'Distingue las plataformas',
    platformsBody:
      'Cada mensaje puede mostrar una insignia de la plataforma de la que viene, así tú y tus espectadores veis de un vistazo si viene de Kick o de YouTube. Activa las insignias en el editor de overlays. Con {css} puedes dar a cada plataforma su propio color, porque cada mensaje va etiquetado con su plataforma.',
    platformsCssLinkText: 'CSS personalizado',
    moderationHeading: 'Un solo lugar para moderar',
    moderationBody:
      'El monitor de chat muestra juntas todas las plataformas. Desde él puedes eliminar mensajes, expulsar temporalmente, banear y desbanear en Twitch, Kick y Discord, y expulsar temporalmente y banear en YouTube, siempre en la plataforma de la que viene el mensaje. TikTok no tiene API de moderación. Moderar desde All-Chat es una función {premium}, y puedes dar acceso al monitor a tus moderadores.',
    moderationDock: 'Mantén el monitor dentro de OBS con la {guide}.',
    moderationDockLinkText: 'guía del dock de chat para OBS',
    engagementHeading: 'Encuestas y predicciones entre plataformas',
    engagementBody:
      'All-Chat tiene sus propias encuestas, predicciones y puntos de espectador. Los espectadores de todas las plataformas conectadas participan desde el chat, así que una encuesta no se limita a tu audiencia de Twitch.',
    overlaysHeading: 'Overlays separados para lienzos separados',
    overlaysBody:
      'Una cuenta puede tener varios overlays, cada uno con sus propias fuentes de chat y su tema. Una configuración habitual es un overlay con todas las plataformas para el lienzo horizontal y un segundo overlay más estrecho para un lienzo vertical, por ejemplo uno que solo muestre el chat de TikTok.',
    limitTiktok:
      'TikTok es la plataforma con más probabilidades de caerse durante un momento. La {guide} explica por qué.',
    limitYoutube:
      'El chat de YouTube funciona por sí solo con los directos públicos. Para un directo oculto, pega su enlace en los ajustes de la fuente. El chat solo para miembros de tu propio canal necesita el modo premium de API oficial. Los directos programados se detectan cuando empiezan.',
    compareNote:
      '¿Estás eligiendo entre herramientas? La {compare} recoge lo que ofrecen Streamlabs, SE.Live, Social Stream Ninja, SleepyChat y Restream Chat, con fuentes.',
    compareNoteLinkText: 'página de comparativa',
  },
  // Cells about other products state only what their own pages say.
  compare: {
    metaTitle: 'Comparativa de herramientas de chat multistream',
    metaDescription:
      'All-Chat frente a Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat y Restream Chat: plataformas, instalación, overlay y dock de OBS, emotes, precio, código abierto, alertas y TTS, con una fuente para cada celda.',
    breadcrumb: 'Comparativa',
    heading: 'All-Chat comparado con otras herramientas de chat multistream',
    intro:
      'Esta tabla pone All-Chat junto a otras cinco herramientas que unen el chat de varias plataformas. All-Chat lo hacemos nosotros, así que léela teniéndolo en cuenta. Cada afirmación sobre otra herramienta enlaza a las páginas de esa herramienta, y donde no encontramos ninguna declaración escribimos "no documentado" en lugar de suponer.',
    lastChecked: 'Última comprobación: {date}',
    caption:
      'Herramientas de chat multistream por función, última comprobación el {date}. Los enlaces numerados llevan a las fuentes que aparecen debajo de la tabla.',
    colTool: 'Herramienta',
    colPlatforms: 'Chat de Twitch, YouTube, Kick y TikTok',
    colInstall: 'Requiere instalación',
    colOverlay: 'Overlay de OBS',
    colDock: 'Dock de OBS',
    colEmotes: '7TV, BTTV, FFZ',
    colPrice: 'Precio del chat unificado',
    colOpenSource: 'Código abierto',
    colSelfHost: 'Autoalojable',
    colAlerts: 'Alertas',
    colTts: 'TTS',
    notDocumented: 'No documentado',
    yes: 'Sí',
    allThree: 'Sí, los tres',
    sourceRefLabel: 'Fuente {number}',
    toolAllChat: 'All-Chat',
    allChatPlatforms: 'Las cuatro, más Discord',
    allChatInstall: 'No. Pega una URL en una fuente de navegador',
    allChatDock: 'Sí, el monitor de chat',
    allChatPrice: 'Gratis. Extras Premium a través de Patreon',
    allChatOpenSource: 'Sí, AGPL-3.0',
    allChatSelfHost: 'Sí, con Docker Compose',
    allChatAlerts: 'En desarrollo',
    allChatTts: 'Sí. Voces del navegador gratis, ElevenLabs con Premium y tu propia clave',
    toolStreamlabs: 'Streamlabs Chat Box y Multichat',
    streamlabsPlatforms:
      'Twitch, YouTube, Kick. TikTok no está en la lista de plataformas de Chat Box; un artículo de ayuda dice que el chat de TikTok se abre en una ventana de navegador aparte',
    streamlabsInstall:
      'No para Chat Box, que es una URL de fuente de navegador. Multichat viene con Streamlabs Multistream',
    streamlabsOverlay: 'Sí, Chat Box',
    streamlabsDock: 'Multichat en Streamlabs Desktop. No hay documentado un dock de OBS',
    streamlabsPrice:
      'Chat Box gratis. Multistream es una función de Ultra; Dual Output gratuito cubre un destino horizontal y uno vertical',
    streamlabsOpenSource: 'Streamlabs Desktop es GPL-3.0. Widgets: no documentado',
    streamlabsAlerts: 'Sí, Alert Box',
    streamlabsTts: 'Sí, lee en voz alta propinas y bits en Alert Box',
    toolSeLive: 'StreamElements SE.Live',
    seLivePlatforms:
      'Multi-Chat muestra todas las plataformas conectadas, y se pueden conectar Twitch, YouTube y Kick. El chat de TikTok tiene su propio dock, fuera de Multi-Chat',
    seLiveInstall: 'Sí, un plugin de OBS, solo para Windows',
    seLiveOverlay:
      'No hay documentado un overlay de chat unificado. En febrero de 2025 el equipo dijo que el widget de chat solo muestra la plataforma principal',
    seLiveDock: 'Sí, dock de chat y ventana Multi-Chat',
    seLivePrice: 'Plugin gratuito',
    seLiveAlerts: 'Sí, alertas de overlay',
    seLiveTts: 'Sí, mediante canjes de Stream Store',
    toolSsn: 'Social Stream Ninja',
    ssnPlatforms: 'Las cuatro, más otros 100+ sitios',
    ssnInstall:
      'Sí, una extensión de navegador o una app de escritorio. Una app web Lite tiene menos funciones',
    ssnDock: 'Sí, la página de dock de chat',
    ssnPrice: 'Gratis. Algunas integraciones opcionales necesitan cuentas de pago',
    ssnOpenSource: 'Sí, GPL-3.0',
    ssnAlerts: 'Sí, overlays de alertas de eventos',
    ssnTts: 'Sí. Voces del sistema, más proveedores de pago opcionales',
    toolSleepyChat: 'SleepyChat',
    sleepyChatPlatforms: 'Las cuatro',
    sleepyChatInstall: 'No. Copia un enlace desde el panel web',
    sleepyChatPrice: 'Plan gratuito con límites de uso. Premium los amplía',
    sleepyChatOpenSource:
      'No documentado. El repositorio público de GitHub es un gestor de incidencias',
    sleepyChatAlerts: 'Sí, alertas animadas personalizadas',
    sleepyChatTts: 'Sí, una sola cola para las cuatro plataformas',
    toolRestream: 'Restream Chat',
    restreamPlatforms: 'Twitch, YouTube, Kick. No lee el chat de TikTok',
    restreamInstall: 'No. Funciona en el navegador; hay una app de escritorio opcional',
    restreamOverlay: 'Sí, una URL de inserción como fuente de navegador',
    restreamPrice:
      'El plan gratuito incluye chat. Los mensajes solo llegan mientras haces stream a través de Restream',
    restreamTts: 'Alertas de texto a voz en la app de chat de escritorio',
    sourcesHeading: 'Fuentes',
    sourceAllChatReadme: 'README de All-Chat en GitHub',
    sourceStreamlabsChatBox: 'Streamlabs: widget Chat Box',
    sourceStreamlabsMultistream:
      'Streamlabs: página de Multistream y preguntas frecuentes de Multichat',
    sourceStreamlabsWidgets: 'Streamlabs: página de widgets y preguntas frecuentes',
    sourceStreamlabsTiktok:
      'Ayuda de Streamlabs: cómo emitir en directo en TikTok desde un ordenador',
    sourceStreamlabsDesktopRepo: 'Código fuente de Streamlabs Desktop en GitHub',
    sourceStreamlabsTts: 'Streamlabs: cómo añadir texto a voz para propinas (y bits)',
    sourceSeLiveGuide: 'Ayuda de StreamElements: SE.Live, la guía completa',
    sourceSeMultichatOverlay:
      'Comunidad de StreamElements: petición de Multichat Overlay y respuesta del equipo',
    sourceSeAlerts:
      'Ayuda de StreamElements: configurar alertas de Twitch con overlays de StreamElements',
    sourceSeTts: 'Ayuda de StreamElements: Stream Store, la guía completa',
    sourceSsnReadme: 'README de Social Stream Ninja en GitHub',
    sourceSsnFeatures: 'Social Stream Ninja: funciones',
    sourceSsnParameters: 'Social Stream Ninja: parámetros del overlay',
    sourceSleepyChatHome: 'Página de inicio de SleepyChat',
    sourceSleepyChatObs: 'SleepyChat: guía del dock de navegador y el overlay de OBS',
    sourceSleepyChatPricing: 'SleepyChat: precios',
    sourceSleepyChatRepo: 'Repositorio de SleepyChat en GitHub',
    sourceRestreamRead: 'Ayuda de Restream: leer los mensajes de chat entrantes',
    sourceRestreamChat: 'Restream: página del producto Chat',
    sourceRestreamEncoders: 'Ayuda de Restream: cómo usar Restream Chat con codificadores',
    sourceRestreamEmbed: 'Ayuda de Restream: insertar el chat en tu software de streaming',
    sourceRestreamFree: 'Ayuda de Restream: ¿puedo usar Restream gratis?',
    betterHeading: 'Cuándo es mejor elegir otra herramienta',
    betterIntro: 'Ninguna herramienta de esta lista es la mejor en todo. Elige otra si:',
    betterAlerts:
      'Necesitas alertas en el stream hoy mismo. Las alertas de All-Chat siguen en desarrollo, mientras que SleepyChat, Streamlabs y StreamElements ya las ofrecen.',
    betterTts:
      'Quieres texto a voz vinculado a propinas y alertas. Streamlabs lee en voz alta propinas y bits en su Alert Box, y SleepyChat mantiene el TTS de las cuatro plataformas en una sola cola junto a sus alertas.',
    betterPlatforms:
      'Haces stream en plataformas más allá de estas cuatro. Social Stream Ninja cubre más de 100 sitios, entre ellos Facebook, Instagram, Rumble y Zoom.',
    betterRestream:
      'Ya haces multistream a través de Restream y quieres responder y reenviar mensajes entre plataformas desde una sola ventana. Restream Chat está integrado en ese flujo de trabajo.',
    fitHeading: 'Dónde encaja All-Chat',
    fitBody:
      'All-Chat no necesita instalación, cubre TikTok junto a Twitch, YouTube, Kick y Discord, muestra emotes de 7TV, BTTV y FFZ, y es de código abierto bajo AGPL-3.0. El chat unificado es gratis; la moderación desde el monitor de chat y algunos extras son Premium.',
    corrections:
      'Los productos cambian. Si una celda está desactualizada, {issue} y la corregiremos.',
    correctionsLinkText: 'abre una incidencia en GitHub',
  },
}
