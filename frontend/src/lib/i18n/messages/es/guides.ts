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
    linkAlerts: 'Alertas de stream para OBS',
    linkChatBot: 'Bot de chat multistream',
    linkCompare: 'Comparativa con otras herramientas de chat',
  },
  index: {
    metaTitle:
      'Guías: overlay de chat para OBS, dock de chat, alertas de stream, bot de chat y chat multistream',
    metaDescription:
      'Guías paso a paso de All-Chat: lleva el chat unificado a OBS, acóplalo dentro de OBS, muestra el chat de TikTok LIVE, une el chat multistream, añade alertas de stream y un bot de chat, y compara herramientas de chat.',
    breadcrumb: 'Guías',
    heading: 'Guías',
    intro:
      'Cómo llevar a OBS el chat unificado de Twitch, YouTube, Kick, TikTok y Discord, cómo añadirle alertas y un bot de chat, y cómo se compara All-Chat con otras herramientas de chat.',
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
      'All-Chat frente a Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat, Restream Chat y Multistream: plataformas, instalación, overlay y dock de OBS, emotes, precio, código abierto, alertas y TTS, con una fuente para cada celda.',
    breadcrumb: 'Comparativa',
    heading: 'All-Chat comparado con otras herramientas de chat multistream',
    intro:
      'Esta tabla pone All-Chat junto a otras seis herramientas que unen el chat de varias plataformas. All-Chat lo hacemos nosotros, así que léela teniéndolo en cuenta. Cada afirmación sobre otra herramienta enlaza a las páginas de esa herramienta, y donde no encontramos ninguna declaración escribimos "no documentado" en lugar de suponer.',
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
    allChatAlerts: 'Sí, un overlay de alertas, con HTML, CSS y JavaScript propios si quieres',
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
    toolMultistream: 'Multistream (multistream.gg)',
    multistreamPlatforms: 'Las cuatro, más Instagram',
    multistreamInstall: 'Sí, una app de escritorio para Windows o Mac. Linux no es compatible',
    multistreamOverlay:
      'Sí, un enlace de fuente de navegador a la app local. Solo funciona mientras la app está abierta en el mismo PC que OBS',
    multistreamDock:
      'Los widgets privados se pueden añadir como docks de OBS. Un dock de chat no está documentado',
    multistreamPrice:
      'Gratis en la app de escritorio. El chat en la web forma parte de Pro, 10 $ al mes',
    multistreamAlerts: 'Sí, follows, suscripciones, regalos y raids',
    multistreamTts: 'Sí, voces integradas, mientras la app está abierta',
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
    sourceMultistreamHome: 'Página de inicio de Multistream',
    sourceMultistreamDownload: 'Multistream: descargar la app',
    sourceMultistreamOverlay: 'Tutorial de Multistream: añadir un overlay a OBS',
    sourceMultistreamChats: 'Guía de Multistream: ver todos los chats de tu stream en una ventana',
    sourceMultistreamPricing: 'Multistream: precios',
    sourceMultistreamTts: 'Tutorial de Multistream: leer el chat en voz alta',
    betterHeading: 'Cuándo es mejor elegir otra herramienta',
    betterIntro: 'Ninguna herramienta de esta lista es la mejor en todo. Elige otra si:',
    betterAllInOne:
      'Quieres que la herramienta que une tu chat envíe también tu vídeo a todas las plataformas. Multistream y Restream hacen las dos cosas, y Streamlabs Multistream también con Ultra. All-Chat no retransmite vídeo; añade chat, alertas y un bot de chat sobre lo que sea que emita tu stream.',
    betterTts:
      'Quieres texto a voz vinculado a propinas y alertas. Streamlabs lee en voz alta propinas y bits en su Alert Box, y SleepyChat mantiene el TTS de las cuatro plataformas en una sola cola junto a sus alertas.',
    betterPlatforms:
      'Haces stream en plataformas más allá de estas cuatro. Social Stream Ninja cubre más de 100 sitios, entre ellos Facebook, Instagram, Rumble y Zoom.',
    betterRestream:
      'Ya haces multistream a través de Restream y quieres responder y reenviar mensajes entre plataformas desde una sola ventana. Restream Chat está integrado en ese flujo de trabajo.',
    fitHeading: 'Dónde encaja All-Chat',
    fitBody:
      'All-Chat no necesita instalación, cubre TikTok junto a Twitch, YouTube, Kick y Discord, muestra emotes de 7TV, BTTV y FFZ, y es de código abierto bajo AGPL-3.0. Los overlays de alertas, los comandos de chat, los activadores por palabra clave y los temporizadores vienen integrados, así que el chat, las alertas y el bot de chat salen de una sola pestaña del navegador, sin ninguna app abierta junto a OBS. El chat unificado, las alertas y los comandos son gratis; la moderación desde el monitor de chat y algunos extras son Premium.',
    corrections:
      'Los productos cambian. Si una celda está desactualizada, {issue} y la corregiremos.',
    correctionsLinkText: 'abre una incidencia en GitHub',
  },
  alerts: {
    metaTitle:
      'Alertas de stream para OBS: Twitch, YouTube, Kick y TikTok en una sola caja de alertas',
    metaDescription:
      'Caja de alertas gratuita para OBS que cubre Twitch, YouTube, Kick y TikTok a la vez: suscripciones, bits, raids, follows, Super Chats, membresías y regalos de TikTok, con tu propio GIF, sonido y texto o con HTML, CSS y JavaScript propios.',
    breadcrumb: 'Alertas de stream',
    heading: 'Alertas de stream para Twitch, YouTube, Kick y TikTok en una sola caja de alertas',
    intro:
      'Un overlay de alertas de All-Chat es una caja de alertas que añades a OBS como fuente de navegador. Muestra los eventos de todas las plataformas a las que está conectado, así que un multistream necesita una sola fuente de alertas en lugar de una por plataforma. Es gratis y funciona en el navegador, sin nada que instalar.',
    eventsHeading: 'Qué muestra',
    eventsTwitch:
      'Twitch: suscripciones, resuscripciones, suscripciones regaladas, bits, raids, follows, canjes de puntos de canal y donaciones benéficas.',
    eventsYoutube:
      'YouTube: Super Chats, Super Stickers, nuevos miembros, hitos de miembro, membresías regaladas y nuevos suscriptores.',
    eventsKick: 'Kick: suscripciones, suscripciones regaladas, hosts, regalos de KICKs y follows.',
    eventsTiktok: 'TikTok LIVE: regalos, compartidos y suscripciones.',
    eventsBody:
      'Las alertas aparecen de una en una con su texto, una imagen o GIF y un sonido, y después la fuente vuelve a quedar vacía.',
    stepsHeading: 'Añade una caja de alertas a OBS',
    stepCreate: 'Inicia sesión en allch.at y crea un overlay de tipo {kind}.',
    stepCreateKind: 'Alertas',
    stepSources:
      'Añade una fuente de chat por cada plataforma en la que haces stream. Un overlay de alertas usa las mismas fuentes que un overlay de chat.',
    stepStyle:
      'En la sección {section} del editor del overlay, define el aspecto de todas las alertas en {all}: una imagen o GIF, un sonido, una duración y un volumen.',
    stepStyleSection: 'Alertas',
    stepStyleAll: 'Todos los eventos (estilo por defecto)',
    stepObs: 'Copia la URL del overlay y añádela a OBS como fuente de navegador.',
    stepTest: 'Haz clic en {test}. La alerta se reproduce en la vista previa del editor y en OBS.',
    stepTestButton: 'Enviar alerta de prueba',
    customHeading: 'Haz tuya cada alerta',
    customBody:
      'Elige un tipo de evento para darle su propio texto o su propia imagen y sonido. El texto admite estas variables: {placeholders}. Los campos que dejes vacíos usan el estilo por defecto. Busca GIF en el editor o pega un enlace.',
    codeBody:
      'Para tener control total, escribe tu propio HTML, CSS y JavaScript para una alerta. Se ejecuta en un entorno aislado dentro del overlay.',
    commandsBody:
      'Un {commands} puede mostrar su propia alerta en el mismo overlay: una imagen, un sonido y un texto, activados desde cualquier chat.',
    commandsLinkText: 'comando de chat',
    limitKick:
      'Las suscripciones, las suscripciones regaladas y los hosts de Kick vienen de un feed que Kick no documenta y puede cambiar sin aviso. Los regalos de KICKs anónimos no se muestran.',
    limitUploads:
      'Subir tus propias imágenes, GIF y sonidos es una función {premium}. Buscar GIF y pegar un enlace es gratis.',
    limitTiktok:
      'TikTok es la plataforma con más probabilidad de caerse durante un rato, y sus alertas se detienen con ella. La {guide} tiene los detalles.',
  },
  chatBot: {
    metaTitle: 'Bot de chat multistream: comandos y temporizadores para Twitch, YouTube y Kick',
    metaDescription:
      'Bot de chat gratuito para multistreamers: define una sola vez tus !comandos, activadores por palabra clave y temporizadores, y responden en los chats de Twitch, Kick y YouTube. Variables, consultas web, JavaScript, clips de Twitch y alertas en el stream.',
    breadcrumb: 'Bot de chat',
    heading:
      'Un solo bot de chat para Twitch, YouTube y Kick: comandos, palabras clave y temporizadores',
    intro:
      'All-Chat lleva un bot de chat integrado. Defines un comando una vez y responde en todos los chats que llevan tus overlays, así que un multistream no necesita un bot por plataforma. Funciona en los servidores de All-Chat: nada que instalar y nada que dejar abierto.',
    kindsHeading: 'Comandos, palabras clave y temporizadores',
    kindCommand:
      'Un comando se activa cuando un mensaje empieza por su activador, por ejemplo !discord.',
    kindKeyword:
      'Una palabra clave se activa cuando su palabra o frase aparece en cualquier parte de un mensaje, por ejemplo clip that.',
    kindTimer:
      'Un temporizador publica un mensaje cada X minutos, cada X mensajes de chat, o cada X minutos una vez que han llegado suficientes mensajes.',
    stepsHeading: 'Crea tu primer comando',
    stepOpen: 'Inicia sesión en allch.at y abre {commands} en la barra superior.',
    stepOpenNav: 'Comandos',
    stepNew: 'Haz clic en {create} y escribe un activador como !discord.',
    stepNewButton: 'Nuevo comando',
    stepReply: 'Escribe la respuesta. Se publica en el chat del que vino el mensaje.',
    stepSave: 'Guarda. El comando queda activo en todos los chats que llevan tus overlays.',
    repliesHeading: 'Qué puede hacer una respuesta',
    repliesBody:
      'Las respuestas son plantillas. Las variables insertan al usuario del chat, el texto tras el activador, un contador, un número aleatorio, un elemento de una lista o la hora en una zona horaria. Una variable urlfetch inserta lo que responde una dirección web, y un bloque js ejecuta JavaScript en un entorno aislado.',
    actionsBody:
      'Un comando también puede crear un clip de tu directo de Twitch, o mostrar su propia alerta con una imagen, un sonido y un texto en tu {alerts}.',
    actionsAlertsLinkText: 'overlay de alertas',
    accessBody:
      'Cada comando tiene un tiempo de espera y se puede limitar a suscriptores, VIP, moderadores o a ti.',
    senderHeading: 'Quién envía las respuestas',
    senderBody:
      'Por defecto responde el All-Chat Bot, en Twitch, Kick y YouTube, sin una cuenta aparte. También puedes hacer que las respuestas salgan de tu propia cuenta. Conectar tu propia cuenta de bot, para que las respuestas lleven su nombre, es una función {premium}.',
    limitYoutube:
      'Las respuestas y los temporizadores son gratis en los chats de Twitch y Kick. En el chat de YouTube son una función {premium}, porque cada uno gasta cuota de la API de YouTube.',
    limitReadOnly:
      'Los chats de TikTok y Discord pueden activar un comando, por ejemplo un clip o una alerta, pero no se puede responder en ellos.',
  },
  vs: {
    colFeature: 'Función',
    caption:
      'All-Chat y {tool} por función, última comprobación el {date}. Los enlaces numerados llevan a las fuentes listadas más abajo.',
    pickThemHeading: 'Cuándo es mejor elegir {tool}',
    pickUsHeading: 'Cuándo es mejor elegir All-Chat',
    fullComparison: 'Todas las herramientas, una junto a otra, están en la {compare}.',
    fullComparisonLinkText: 'página de comparativa',
    listHeading: 'Comparativas uno a uno',
    multistreamName: 'Multistream',
    multistreamMetaTitle:
      'All-Chat vs Multistream (multistream.gg): overlay de chat, alertas y bot de chat',
    multistreamMetaDescription:
      'All-Chat y Multistream frente a frente: los dos unen los chats de Twitch, YouTube, Kick y TikTok y añaden alertas y un bot de chat. Uno funciona en el navegador, el otro es una app de escritorio para Windows y Mac. Una fuente para cada celda.',
    multistreamHeading: 'All-Chat vs Multistream',
    multistreamIntro:
      'Las dos herramientas ponen los chats de Twitch, YouTube, Kick y TikTok en un solo overlay de OBS y añaden alertas, un bot de chat y texto a voz. La diferencia está en dónde se ejecutan. Multistream es una app de escritorio que además envía tu vídeo a las plataformas. All-Chat es un servicio alojado y de código abierto que añades a OBS como una URL. All-Chat lo hacemos nosotros; cada celda de Multistream enlaza a sus propias páginas.',
    multistreamPickThem:
      'Quieres que la misma app envíe también tu stream a todas las plataformas. Multistream emite gratis desde tu propio ordenador y desde su nube con Pro, y su chat, sus alertas y su bot de chat viven en la misma app de Windows o Mac.',
    multistreamPickUs:
      'No quieres otra app abierta junto a OBS. Un overlay de All-Chat es una URL que funciona en cualquier ordenador, Linux incluido, mientras que el enlace de overlay de Multistream solo funciona mientras su app está abierta en el mismo PC que OBS. All-Chat además lee Discord, muestra emotes de 7TV, BTTV y FFZ, acepta tu propio CSS, y es de código abierto y se puede autoalojar.',
    ssnName: 'Social Stream Ninja',
    ssnMetaTitle:
      'All-Chat vs Social Stream Ninja: overlay de chat alojado o basado en una extensión',
    ssnMetaDescription:
      'All-Chat y Social Stream Ninja frente a frente: los dos son gratis y de código abierto y unen los chats de Twitch, YouTube, Kick y TikTok. Uno está alojado, el otro funciona en una extensión de navegador o una app de escritorio.',
    ssnHeading: 'All-Chat vs Social Stream Ninja',
    ssnIntro:
      'Las dos son herramientas gratuitas y de código abierto que unen el chat de varias plataformas en un overlay de OBS y un dock. Social Stream Ninja captura el chat con una extensión de navegador o una app de escritorio en tu ordenador. All-Chat lee las plataformas en sus servidores y te da una URL. All-Chat lo hacemos nosotros; cada celda de Social Stream Ninja enlaza a sus propias páginas.',
    ssnPickThem:
      'Haces stream en plataformas más allá de Twitch, YouTube, Kick, TikTok y Discord, o quieres que todo se ejecute en tu propia máquina. Social Stream Ninja cubre más de 100 sitios.',
    ssnPickUs:
      'No quieres instalar nada ni tener nada que deba seguir abierto. All-Chat no necesita extensión ni app: la URL del overlay funciona en OBS en cualquier ordenador. La moderación, las encuestas, las alertas, los comandos de chat y los temporizadores se gestionan desde la misma cuenta.',
    streamlabsName: 'Streamlabs',
    streamlabsMetaTitle: 'All-Chat vs Streamlabs Chat Box: chat multistream con TikTok',
    streamlabsMetaDescription:
      'All-Chat frente a Chat Box y Multichat de Streamlabs: plataformas, chat de TikTok, overlay y dock de OBS, emotes, precio, alertas y TTS, con una fuente para cada celda.',
    streamlabsHeading: 'All-Chat vs Streamlabs Chat Box y Multichat',
    streamlabsIntro:
      'Streamlabs tiene dos herramientas de chat: el widget Chat Box, una fuente de navegador para Twitch, YouTube y Kick, y Multichat, que viene con Streamlabs Multistream. All-Chat es un overlay de chat alojado y de código abierto que incluye TikTok y Discord. All-Chat lo hacemos nosotros; cada celda de Streamlabs enlaza a sus propias páginas.',
    streamlabsPickThem:
      'Ya haces stream con Streamlabs Desktop y quieres el chat, la Alert Box y las propinas en una sola cuenta, o quieres que las propinas y los bits se lean en voz alta con texto a voz.',
    streamlabsPickUs:
      'También haces stream en TikTok. TikTok no está en la lista de plataformas de Chat Box, mientras que All-Chat une el chat de TikTok LIVE en el mismo overlay que Twitch, YouTube, Kick y Discord. El chat unificado de todas las plataformas es gratis en All-Chat, mientras que Streamlabs Multistream es una función de Ultra, y All-Chat es de código abierto y se puede autoalojar.',
    seLiveName: 'StreamElements SE.Live',
    seLiveMetaTitle: 'All-Chat vs StreamElements SE.Live: overlay de chat unificado para OBS',
    seLiveMetaDescription:
      'All-Chat y StreamElements SE.Live frente a frente: overlay de chat unificado, dock Multi-Chat, TikTok, instalación, precio, alertas y TTS, con una fuente para cada celda.',
    seLiveHeading: 'All-Chat vs StreamElements SE.Live',
    seLiveIntro:
      'SE.Live es el plugin de StreamElements para OBS en Windows, con una ventana Multi-Chat para el streamer. All-Chat es un servicio alojado y de código abierto que pone el chat unificado en el stream como fuente de navegador. All-Chat lo hacemos nosotros; cada celda de StreamElements enlaza a sus propias páginas.',
    seLivePickThem:
      'Ya usas los overlays, las alertas y una Stream Store de StreamElements en Windows y sobre todo quieres leer tú todos los chats en un dock de OBS.',
    seLivePickUs:
      'Quieres que los espectadores vean el chat unificado en el stream. No encontramos ningún overlay de chat unificado en la documentación de StreamElements, y en febrero de 2025 su equipo dijo que el widget de chat solo muestra la plataforma principal. All-Chat además mantiene TikTok dentro del chat unificado en lugar de en un dock aparte, y no necesita ningún plugin ni un sistema operativo concreto.',
    sleepyChatName: 'SleepyChat',
    sleepyChatMetaTitle: 'All-Chat vs SleepyChat: comparativa de overlays de chat multistream',
    sleepyChatMetaDescription:
      'All-Chat y SleepyChat frente a frente: los dos unen los chats de Twitch, YouTube, Kick y TikTok desde un panel web, sin instalación. Emotes, precio, código abierto, alertas y TTS, con una fuente para cada celda.',
    sleepyChatHeading: 'All-Chat vs SleepyChat',
    sleepyChatIntro:
      'Son las dos herramientas más parecidas de nuestra comparativa: las dos están alojadas, no necesitan instalación y te dan un overlay de OBS y un dock para los chats de Twitch, YouTube, Kick y TikTok. All-Chat lo hacemos nosotros; cada celda de SleepyChat enlaza a sus propias páginas.',
    sleepyChatPickThem:
      'Quieres el texto a voz en el centro del stream. SleepyChat mantiene el TTS de las cuatro plataformas en una sola cola junto a sus alertas animadas personalizadas.',
    sleepyChatPickUs:
      'Quieres código abierto. All-Chat es AGPL-3.0 y se puede autoalojar con Docker Compose. Además lee Discord y muestra emotes de 7TV, BTTV y FFZ, y su chat unificado es gratis, mientras que el plan gratuito de SleepyChat tiene límites de uso.',
    restreamName: 'Restream Chat',
    restreamMetaTitle: 'All-Chat vs Restream Chat: chat multistream con TikTok',
    restreamMetaDescription:
      'All-Chat y Restream Chat frente a frente: plataformas, chat de TikTok, overlay y dock de OBS, precio, alertas y TTS, con una fuente para cada celda.',
    restreamHeading: 'All-Chat vs Restream Chat',
    restreamIntro:
      'Restream Chat es la herramienta de chat integrada en Restream, el servicio que envía un stream a varias plataformas. All-Chat no retransmite vídeo; une el chat llegue como llegue tu stream a las plataformas. All-Chat lo hacemos nosotros; cada celda de Restream enlaza a sus propias páginas.',
    restreamPickThem:
      'Ya haces multistream a través de Restream y quieres responder y reenviar mensajes entre plataformas desde una sola ventana.',
    restreamPickUs:
      'Haces stream en TikTok, o no envías tu stream a través de Restream. Restream Chat no lee el chat de TikTok, y sus mensajes solo llegan mientras haces stream a través de Restream. All-Chat lee el chat de TikTok LIVE y funciona con cualquier forma de emitir en directo, y añade emotes de 7TV, BTTV y FFZ, alertas y un bot de chat.',
  },
}
