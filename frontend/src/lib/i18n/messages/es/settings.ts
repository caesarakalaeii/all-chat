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

export const settings: LocaleNamespace<'settings'> = {
  ambassador: {
    heading: 'Embajador',
    body: 'Eres embajador de All-Chat. Elige si quieres aparecer en la página de inicio pública.',
    featureToggle: 'Mostrarme en la página de inicio',
    cardReads: 'Tu tarjeta dice: “{tagline}”',
    toastFeatured: 'Ahora aparecerás en la página de inicio',
    toastUnfeatured: 'Ya no apareces en la página de inicio',
    toastFailed: 'No se pudo actualizar el ajuste de la página de inicio',
  },
  language: {
    heading: 'Idioma',
    body: 'Elige el idioma que All-Chat usa en este navegador. Las páginas públicas y los overlays que se ejecutan en OBS siguen en inglés.',
    selectLabel: 'Idioma de la interfaz',
    betaNote:
      'Hay más idiomas en pruebas beta. Los beta testers y los embajadores ya pueden elegirlos.',
    checkFailed: 'No se pudo comprobar qué idiomas puedes usar. El inglés siempre está disponible.',
  },
  // The effect* keys must match what DisconnectPlatform in auth-service does.
  connections: {
    heading: 'Plataformas conectadas',
    body: 'All-Chat guarda un token de OAuth por cada plataforma que has conectado. Al desconectar una plataforma se eliminan sus tokens.',
    loading: 'Cargando plataformas conectadas',
    loadFailed: 'No se pudieron cargar tus plataformas conectadas.',
    empty: 'No hay plataformas conectadas.',
    signInBadge: 'Inicio de sesión',
    signInNote:
      'Inicias sesión con {platform}, así que no se puede desconectar aquí. Sus tokens se eliminan cuando eliminas tu cuenta.',
    disconnect: 'Desconectar',
    disconnectLabel: 'Desconectar {platform}',
    disconnecting: 'Desconectando…',
    disconnectTitle: '¿Desconectar {platform}?',
    effectTwitch:
      'Esto elimina tus tokens de Twitch y pide a Twitch que los revoque. Tus fuentes de Twitch siguen en tus overlays, pero dejan de recibir chat, y el envío y la moderación en Twitch se detienen hasta que vuelvas a conectar Twitch. Hasta entonces, tampoco puedes iniciar sesión en esta cuenta con Twitch.',
    effectYoutube:
      'Esto elimina tus tokens de YouTube y pide a Google que los revoque. Tus fuentes de YouTube siguen leyendo los directos públicos, que no necesitan token. Los directos ocultos y solo para miembros, las alertas de suscriptores, el envío y la moderación en YouTube se detienen hasta que vuelvas a conectar YouTube. Hasta entonces, tampoco puedes iniciar sesión en esta cuenta con YouTube.',
    effectKick:
      'Esto elimina tus tokens de Kick y pide a Kick que los revoque. Tus fuentes de Kick siguen recibiendo chat. El envío y la moderación en Kick se detienen hasta que vuelvas a conectar Kick. Hasta entonces, tampoco puedes iniciar sesión en esta cuenta con Kick.',
    effectFacebook:
      'Esto elimina los tokens de tu página de Facebook y quita tus fuentes de Facebook de tus overlays. Meta no nos ofrece ninguna forma de revocar el token, así que quita también All-Chat de las aplicaciones conectadas a tu cuenta de Facebook.',
    effectInstagram:
      'Esto elimina tus tokens de Instagram y quita tus fuentes de Instagram de tus overlays. Meta no nos ofrece ninguna forma de revocar el token, así que quita también All-Chat de las aplicaciones conectadas a tu cuenta de Facebook.',
    disconnectCancel: 'Cancelar',
    disconnectConfirm: 'Sí, desconectar',
    disconnectedToast: '{platform} desconectado',
    disconnectFailedToast: 'No se pudo desconectar {platform}',
  },
  dataExport: {
    download: 'Descargar mis datos',
    downloading: 'Preparando tu archivo…',
    failedToast: 'No se pudieron descargar tus datos',
    viewerHeading: 'Tus datos',
    viewerBody:
      'Descarga un archivo JSON con todo lo que All-Chat guarda sobre tu cuenta de espectador: plataformas vinculadas, sesiones, cosméticos, puntos, votos, predicciones y mensajes recientes.',
  },
  index: {
    heading: 'Ajustes',
    profileHeading: 'Perfil',
    usernameLabel: 'Nombre de usuario',
    primaryPlatformLabel: 'Plataforma principal',
    primaryPlatformUnknown: 'Desconocida',
    setupGuideHeading: 'Guía de configuración',
    setupGuideBody:
      'Repasa de nuevo la configuración del overlay: crea un overlay, conecta el chat, elige un tema y consigue el enlace para OBS.',
    setupGuideRestart: 'Reiniciar',
    setupGuideRestarting: 'Reiniciando…',
    premiumHeading: 'Premium',
    premiumBody: 'Desbloquea las funciones premium apoyando a All-Chat en Patreon.',
    premiumManage: 'Gestionar Premium',
    devicesHeading: 'Dispositivos vinculados',
    devicesBody:
      'Consulta y revoca las superficies de control Stream Deck y StreamController vinculadas a tu cuenta. Cada una está asociada a un único overlay.',
    devicesManage: 'Gestionar dispositivos',
    tokensHeading: 'Tokens de API',
    tokensBody:
      'Crea y revoca tokens de acceso personal. Úsalos donde la vinculación no llega: un equipo de captura sin pantalla, un segundo PC o un script.',
    tokensManage: 'Gestionar tokens',
    privacyHeading: 'Datos y privacidad',
    privacyBody:
      'Recogemos los mínimos datos posibles y de forma transparente. Consulta las políticas de abajo para saber cómo se tratan los tokens, los overlays y los metadatos del chat.',
    privacyPolicyLink: 'Política de privacidad',
    termsLink: 'Términos del servicio',
    discordHeading: 'Discord',
    discordServersLoading: 'Cargando servidores de Discord',
    discordNoServer: 'No hay ningún servidor de Discord conectado.',
    discordConnectServer: 'Conectar servidor de Discord',
    discordConnectAnother: 'Conectar otro servidor',
    discordDisconnect: 'Desconectar',
    discordDisconnectTitle: '¿Desconectar {guild}?',
    discordDisconnectBody: 'Esto quitará todas las fuentes de Discord conectadas a {guild}.',
    discordDisconnectCancel: 'Cancelar',
    discordDisconnectConfirm: 'Sí, desconectar',
    discordAccountHeading: 'Tu cuenta de Discord',
    discordAccountLoading: 'Cargando la vinculación de tu cuenta de Discord',
    discordAccountLinked:
      'Vinculada como {username}. Es necesaria para que los moderadores puedan actuar en tus servidores de Discord.',
    discordAccountFallbackName: 'tu cuenta de Discord',
    discordAccountUnlinked:
      'Sin vincular. Vincúlala para que los moderadores que invites puedan actuar en tus servidores de Discord.',
    discordAccountUnlink: 'Desvincular',
    discordAccountLink: 'Vincular cuenta de Discord',
    dangerHeading: 'Zona de peligro',
    dangerBody:
      'Al eliminar tu cuenta se eliminan todos los overlays, las autorizaciones de OAuth y las fuentes de chat en caché. Esta acción es permanente y no se puede deshacer.',
    deleteAccount: 'Eliminar cuenta',
    deleteConfirmTitle: '¿Eliminar tu cuenta?',
    deleteConfirmBody:
      'Esto elimina permanentemente tu cuenta y todos los overlays. Esta acción no se puede deshacer.',
    deleteCancel: 'Cancelar',
    deleteConfirm: 'Sí, eliminar mi cuenta',
    restartGuideFailedToast: 'No se pudo reiniciar la guía de configuración',
    accountDeletedToast: 'Cuenta eliminada',
    accountDeleteFailedToast: 'No se pudo eliminar la cuenta',
    discordServerConnectedToast: '¡Servidor de Discord conectado!',
    discordLinkedToast: '¡Cuenta de Discord vinculada!',
    discordUnlinkedToast: 'Cuenta de Discord desvinculada',
    discordUnlinkFailedToast: 'No se pudo desvincular tu cuenta de Discord',
    discordDisconnectFailedToast: 'No se pudo desconectar el servidor',
  },
  viewer: {
    heading: 'Identidad de espectador',
    subheading: 'Personaliza cómo aparece tu nombre en todos los overlays',
    signInHeading: 'Inicia sesión para gestionar tu identidad de espectador',
    signInBody:
      'Conecta tu cuenta de una plataforma de streaming para elegir un color de nombre personalizado y gestionar tu identidad de espectador.',
    signInTwitch: 'Iniciar sesión con Twitch',
    signInYoutube: 'Iniciar sesión con YouTube',
    signInKick: 'Iniciar sesión con Kick',
    profileHeading: 'Perfil',
    viewerFallbackName: 'Espectador',
    nameColorHeading: 'Color del nombre',
    nameColorBody: 'Elige un color o un degradado personalizado para tu nombre en los overlays',
    solidTab: 'Color sólido',
    gradientTab: 'Degradado',
    premiumPill: 'Premium',
    gradientUpsell: 'Los nombres con degradado son un cosmético premium de espectador.',
    unlockPremium: 'Desbloquear premium de espectador',
    colorPickerLabel: 'Selector de color del nombre',
    colorHexLabel: 'Valor hexadecimal del color del nombre',
    savedFeedback: 'Guardado ✓',
    autoSaveNote: 'Los cambios se guardan automáticamente',
    previewLabel: 'Vista previa',
    previewMessage: '¡Hola, mundo!',
    removeStopLabel: 'Quitar parada {index}',
    addStop: '+ Añadir parada',
    angleLabel: 'Ángulo',
    angleDegreesLabel: 'Ángulo en grados',
    saveGradient: 'Guardar degradado',
    savingGradient: 'Guardando…',
    cosmeticsHeading: 'Cosméticos del avatar',
    cosmeticsBody: 'Elige un marco y un flair para tu avatar',
    cosmeticsUpsell: 'Algunos marcos y flairs son premium de espectador.',
    frameHeading: 'Marco del avatar',
    flairHeading: 'Flair del avatar',
    noneItem: 'Ninguno',
    save: 'Guardar',
    saving: 'Guardando…',
    premiumRequired: 'Requiere premium',
    saveFailed: 'No se pudo guardar',
    linkedHeading: 'Plataformas vinculadas',
    linkedBody: 'Conecta más plataformas para compartir tus cosméticos en todos tus chats',
    connected: 'Conectada',
    connect: 'Conectar',
    connecting: 'Conectando…',
    disconnect: 'Desconectar',
    disconnecting: 'Desconectando…',
    loadLinkedFailed: 'No se pudieron cargar las plataformas vinculadas',
    disconnectFailed: 'No se pudo desconectar la plataforma',
    deleteHeading: 'Eliminar datos de espectador',
    deleteBody:
      'Borra permanentemente tu identidad de espectador: plataformas vinculadas, color del nombre y cosméticos del avatar, puntos, votos en encuestas y predicciones en todos los canales, los mensajes que enviaste a través de All-Chat y tu conexión de Patreon de espectador. Se incluyen los votos y puntos que conseguiste con comandos de chat sin iniciar sesión. Si tienes una cuenta de streamer, no se ve afectada.',
    deleteButton: 'Eliminar mis datos de espectador',
    deleteConfirmTitle: '¿Eliminar tus datos de espectador?',
    deleteConfirmBody:
      'Todo lo indicado se borra de inmediato y se cierra tu sesión. Esto no se puede deshacer. Esto no cancela una suscripción de Patreon; cancélala en Patreon. Si sigues chateando en canales que usan encuestas o puntos de All-Chat, a partir de entonces se recogerán datos nuevos.',
    deleteCancel: 'Cancelar',
    deleteConfirm: 'Sí, eliminar mis datos de espectador',
    deleting: 'Eliminando…',
    deleteFailed: 'No se pudieron eliminar tus datos de espectador. Inténtalo de nuevo.',
    deletedToast: 'Tus datos de espectador se han eliminado',
  },
  apiTokens: {
    heading: 'Tokens de API',
    subheading: 'Tokens de acceso personal para los plugins de Stream Deck y StreamController.',
    listHeading: 'Tus tokens',
    listBody:
      'Solo se guardan los datos de abajo: el token en sí se conserva como hash y no se puede volver a mostrar.',
    loadFailed: 'No se pudieron cargar tus tokens. Actualiza la página para intentarlo de nuevo.',
    revealRegionLabel: 'Nuevo token {name}',
    revealHeading: 'Copia ahora tu nuevo token',
    revealWarning:
      'Esta es la única vez que se mostrará {name}. Solo guardamos un hash, así que no se puede volver a mostrar: si lo pierdes, revoca este token y crea uno nuevo.',
    copyToken: 'Copiar token',
    copied: 'Copiado ✓',
    copyFailed: 'No se pudo copiar automáticamente: selecciona el token y cópialo a mano.',
    dismissReveal: 'Ya lo he guardado',
    createHeading: 'Crear un token',
    createBody:
      'Ponle al token un nombre que reconozcas más adelante y concédele solo lo que el dispositivo necesita.',
    nameLabel: 'Nombre del token',
    namePlaceholder: 'Stream Deck (PC del estudio)',
    nameDescription: 'Aparece en la lista de abajo para que sepas qué revocar.',
    scopesLegend: 'Permisos',
    noScopesWarning:
      'Elige al menos un permiso: un token sin ninguno puede autenticarse, pero no hacer nada.',
    create: 'Crear token',
    creating: 'Creando…',
    createFailed: 'No se pudo crear el token. Inténtalo de nuevo.',
    scopeChatWriteTitle: 'Enviar mensajes de chat',
    scopeChatWriteDescription: 'Permite al plugin publicar mensajes en tus chats conectados.',
    scopeEngagementWriteTitle: 'Gestionar encuestas y predicciones',
    scopeEngagementWriteDescription:
      'Permite al plugin abrir, resolver y cancelar encuestas y predicciones.',
    emptyHeading: 'Todavía no tienes ningún token de API',
    emptyBody:
      'Un token de acceso personal permite que un dispositivo inicie sesión en tu nombre sin tu contraseña: así es como los plugins de Stream Deck y StreamController envían mensajes de chat y gestionan encuestas y predicciones por ti. Crea uno por dispositivo para poder revocarlo por separado.',
    setupGuides: 'Guías de configuración:',
    streamDeckReadme: 'README del plugin de Stream Deck',
    streamControllerReadme: 'README del plugin de StreamController',
    tokenDates: 'Creado el {created} · Último uso: {lastUsed}',
    neverUsed: 'nunca',
    unknownDate: '—',
    revokeLabel: 'Revocar {name}',
    revoke: 'Revocar',
    revokeConfirmTitle: '¿Revocar este token?',
    revokeConfirmBody:
      '“{name}” deja de funcionar de inmediato. Cualquier dispositivo que lo use necesitará un token nuevo.',
    revokeCancel: 'Cancelar',
    revokeConfirm: 'Revocar token',
    revoking: 'Revocando…',
    revokedToast: 'Se ha revocado {name}',
    revokeFailedToast: 'No se pudo revocar ese token',
  },
  devices: {
    heading: 'Dispositivos vinculados',
    subheading:
      'Superficies de control Stream Deck y StreamController vinculadas a tu cuenta. Cada una está asociada a un único overlay y caduca por sí sola si deja de usarse.',
    listHeading: 'Tus dispositivos',
    listBody:
      'Solo se guardan los datos de abajo. La credencial en sí se envió directamente al plugin y aquí se conserva como hash: nunca se muestra en este panel, y por eso no hay nada que copiar en esta página.',
    loadingLabel: 'Cargando dispositivos vinculados',
    loadFailed:
      'No se pudieron cargar tus dispositivos vinculados. Actualiza la página para intentarlo de nuevo.',
    emptyHeading: 'Todavía no hay dispositivos vinculados',
    emptyBody:
      'La vinculación empieza en el plugin, no aquí: abre los ajustes de tu Stream Deck o StreamController y pulsa {linkAction}. Tu navegador abre una pantalla de aprobación, eliges un overlay y el plugin recibe su credencial directamente, sin copiar ni pegar nada.',
    emptyLinkAction: 'Vincular con All-Chat',
    setupGuide: 'Guía de configuración',
    havePairingCode: 'Tengo un código de vinculación',
    controlsOverlay: 'Controla {overlay} · {status}',
    rowDates: 'Último uso: {lastUsed} · Vinculado el {paired}',
    neverUsed: 'nunca',
    unknownDate: '—',
    revokeLabel: 'Revocar {name}',
    revoke: 'Revocar',
    statusRevoked: 'Revocado el {date}',
    statusExpired: 'Caducó el {date}',
    statusActive: 'Activo hasta el {date}',
    headlessHeading: '¿En un segundo equipo o en uno sin pantalla?',
    headlessBody:
      'La vinculación necesita que el plugin y este navegador estén en el mismo ordenador. Cuando no es así (un Stream Deck que controla un PC de captura, un servidor sin escritorio), usa un {tokenLink}, o empieza con {pairingLink} si tu plugin muestra uno.',
    headlessTokenLink: 'token de acceso personal',
    headlessPairingLink: 'un código de vinculación',
    revokeConfirmTitle: '¿Revocar este dispositivo?',
    revokeConfirmBody:
      '“{name}” deja de funcionar de inmediato. Vuelve a vincularlo desde el plugin si quieres seguir usándolo.',
    revokeCancel: 'Cancelar',
    revokeConfirm: 'Revocar dispositivo',
    revoking: 'Revocando…',
    revokedToast: 'Se ha revocado {name}',
    revokeFailedToast: 'No se pudo revocar ese dispositivo',
  },
  premium: {
    back: '← Volver a Ajustes',
    heading: 'Premium',
    connectPitch:
      'Apoya a All-Chat en Patreon para desbloquear las funciones premium automáticamente.',
    notAPatronSuffix: '¿Todavía no eres mecenas? {link} y luego conecta.',
    premiumRow: 'Premium',
    statusExpired: 'Por debajo del nivel premium',
    notGranting:
      'Tu Patreon está vinculado, pero no concede premium. Comprueba que tu aportación esté activa y sea del nivel premium o superior.',
    disconnectBody:
      'Esto desvincula tu cuenta de Patreon. Se retirará el premium concedido por tu suscripción.',
  },
  viewerPremium: {
    back: '← Volver a Identidad de espectador',
    heading: 'Premium de espectador',
    subheading:
      'Una suscripción más barata que desbloquea los cosméticos de espectador (tu insignia de chat premium y el degradado del nombre) en todos los overlays en los que apareces.',
    connectPitch:
      'Apoya a All-Chat en Patreon para desbloquear tus cosméticos premium de espectador automáticamente.',
    notAPatronSuffix:
      '¿Todavía no eres mecenas? {link} (nivel de espectador desde 2 €) y luego conecta.',
    premiumRow: 'Premium de espectador',
    statusExpired: 'Por debajo del nivel de espectador',
    notGranting:
      'Tu Patreon está vinculado, pero no concede premium de espectador. Comprueba que tu aportación esté activa y sea del nivel de espectador o superior.',
    disconnectBody:
      'Esto desvincula tu cuenta de Patreon. Se retirará el premium de espectador concedido por tu suscripción.',
    signInHeading: 'Inicia sesión para gestionar el premium de espectador',
    signInBody:
      'Inicia sesión con tu cuenta de una plataforma de streaming y luego apoya a All-Chat en Patreon para desbloquear tu insignia de chat premium y tus cosméticos.',
    signInLink: 'Ir al inicio de sesión de espectador →',
  },
}
