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

export const commands: LocaleNamespace<'commands'> = {
  heading: 'Comandos de chat',
  intro:
    'Responde al chat automáticamente, crea clips al momento y muestra imágenes en el stream. Un comando se activa cuando un mensaje empieza por su activador; una palabra clave, cuando el activador aparece en cualquier parte del mensaje. Las respuestas van al chat del que vino el mensaje, enviadas por el All-Chat Bot o, si lo prefieres, por tu propia cuenta.',
  newCommand: 'Nuevo comando',
  loading: 'Cargando comandos de chat',
  loadFailed: 'No se pudieron cargar tus comandos de chat.',
  retry: 'Reintentar',
  empty: 'Todavía no hay comandos. Empieza con un ejemplo o crea el tuyo.',
  examplesHeading: 'Empieza con un ejemplo',
  examples: {
    hello: 'Saludo',
    clip: 'Clip al momento',
    dice: 'Tirada de dado',
    shoutout: 'Shoutout (mods)',
    dog: 'Perro aleatorio',
  },
  list: {
    kindCommand: 'Comando',
    kindKeyword: 'Palabra clave',
    clips: 'Clips',
    alert: 'Alerta',
    cooldown: '{seconds} s de espera',
    uses: '{count} usos',
    noReply: 'Sin respuesta en el chat',
    edit: 'Editar',
    editLabel: 'Editar {trigger}',
    delete: 'Eliminar',
    deleteLabel: 'Eliminar {trigger}',
    enabledLabel: 'Activar {trigger}',
    toggleFailed: 'No se pudo actualizar el comando.',
  },
  roles: {
    everyone: 'Todos',
    subscriber: 'Suscriptores y superiores',
    vip: 'VIP y superiores',
    moderator: 'Moderadores y tú',
    broadcaster: 'Solo tú',
  },
  sendFailures: {
    heading: 'Mensajes del bot que no llegaron',
    item: 'Último mensaje en el chat de {platform}, {time}:',
    dropped: '{platform} lo rechazó.',
    platformSays: '{platform} dice: “{detail}”',
    reauthBot:
      'El All-Chat Bot perdió su autorización allí. Avísanos en Discord para que lo reconectemos, o cambia el remitente a tu propia cuenta en Bot y remitente.',
    reauthCustom: 'Tu cuenta de bot perdió su autorización. Reconéctala en Bot y remitente.',
    reauthSelf: 'Tu cuenta perdió su autorización. Cierra sesión y vuelve a iniciarla.',
    missingScope: 'Falta un permiso. Concédelo arriba.',
    notConnected: 'No hay conectada ninguna cuenta que pueda publicar allí.',
    offline: 'Tu stream no estaba en directo.',
    quota: 'La cuota de YouTube de hoy se ha agotado.',
    failed: 'No se pudo enviar. Inténtalo más tarde.',
    modHint:
      'Haz al bot moderador o VIP en tu canal: las restricciones de chat como el modo solo seguidores, el modo lento o el requisito de teléfono verificado no se les aplican.',
    clears: 'Esto desaparece en cuanto un mensaje consigue llegar.',
  },
  status: {
    heading: 'Dónde pueden responder los comandos',
    twitchNoLogin: 'Las respuestas y los clips de Twitch requieren que inicies sesión con Twitch.',
    twitchReplies: 'Respuestas en el chat de Twitch',
    twitchClips: 'Clips de Twitch',
    allowed: 'Permitido',
    missing: 'Necesita permiso',
    grant: 'Permitir en Twitch',
    granting: 'Abriendo Twitch',
    grantFailed: 'No se pudo iniciar el flujo de permisos de Twitch.',
    grantNeedsOverlay:
      'Añade primero tu canal de Twitch a uno de tus overlays y luego permítelo aquí.',
    youtubePremium: 'Las respuestas en el chat de YouTube están activas: tienes premium.',
    youtubeLocked:
      'Las respuestas en el chat de YouTube son premium, porque cada una gasta cuota de la API de YouTube. Los comandos siguen creando clips desde el chat de YouTube.',
    upgrade: 'Ver premium',
    kick: 'Las respuestas en el chat de Kick usan el permiso de chat de los controles avanzados de un overlay.',
    others:
      'TikTok, Discord y las plataformas en beta pueden activar clips, pero los comandos no pueden responder allí.',
  },
  editor: {
    createTitle: 'Nuevo comando de chat',
    editTitle: 'Editar {trigger}',
    kindLabel: 'Se activa cuando',
    kindCommand: 'Un mensaje empieza por él',
    kindKeyword: 'Un mensaje lo contiene',
    triggerLabel: 'Activador',
    triggerHintCommand:
      'Una sola palabra, por ejemplo !hello. No se distingue entre mayúsculas y minúsculas.',
    triggerHintKeyword:
      'Una palabra o frase, por ejemplo clip that. Se busca en cualquier parte del mensaje.',
    responseLabel: 'Respuesta',
    responseHint:
      'Déjala vacía para solo crear un clip o mostrar una alerta. Haz clic en una variable para insertarla.',
    insertVariable: 'Insertar {token}',
    alertHeading: 'Alerta en el stream',
    alertHint:
      'Se muestra en tus overlays de alertas cada vez que el comando se activa, desde cualquier chat. Rellena lo que necesites. El estilo Comando de chat del editor de alertas define la duración, el volumen y el texto cuando dejas el tuyo vacío.',
    imageLabel: 'Imagen en el stream',
    imageHint:
      'Un enlace https a una imagen o un GIF. Aquí funcionan las plantillas, pero lo que escriben los espectadores nunca llega a la imagen.',
    soundLabel: 'Sonido de la alerta',
    alertTextLabel: 'Texto de la alerta',
    alertTextHint:
      'Sustituye el texto predeterminado de la alerta en el stream. Usa las mismas variables que la respuesta.',
    imagePreviewAlt: 'Vista previa de la imagen',
    clipLabel: 'Crear un clip de Twitch',
    clipHint:
      'Crea un clip de tu stream de Twitch en directo. Pon {token} en la respuesta para publicar el enlace.',
    roleLabel: 'Quién puede usarlo',
    cooldownLabel: 'Tiempo de espera (segundos)',
    cooldownHint: 'El comando ignora el chat durante este tiempo después de activarse.',
    enabledLabel: 'Activado',
    previewHeading: 'Vista previa',
    previewInputLabel: 'Un espectador escribe después del activador',
    previewInputPlaceholder: '@amigo unas palabras',
    previewEmpty: 'La respuesta aparece aquí.',
    previewFailed: 'Vista previa no disponible.',
    cancel: 'Cancelar',
    save: 'Guardar',
    saving: 'Guardando',
    saveFailed: 'No se pudo guardar el comando.',
    saved: 'Comando guardado',
    referenceHeading: 'Referencia de plantillas',
    jsHeading: 'JavaScript',
    jsBody:
      'Envuelve el código en un bloque js para calcular una respuesta. Se inserta el valor de la última expresión. Están disponibles las variables user, touser, query, args, count, platform, channel, role y clip. El código en sí no tiene acceso a la red ni a archivos, y cada respuesta dispone de 100 ms y 16 MB.',
    jsExample: 'Ejemplo',
    urlfetchHeading: 'Consultar una dirección web',
    urlfetchBody:
      '{token} inserta el texto con el que responde una dirección web. Dentro de un bloque js llega como una cadena, lista para JSON.parse. Solo funcionan direcciones https públicas, como máximo 3 por respuesta y 3 segundos cada una. Las variables de la dirección se escapan, así que los espectadores no pueden cambiar adónde apunta.',
  },
  reference: {
    user: 'Nombre del usuario del chat que usó el comando',
    touser: 'Primera palabra tras el activador sin @, o el usuario del chat si no hay ninguna',
    query: 'Todo lo que va tras el activador; en una palabra clave, el mensaje completo',
    args: 'Palabras sueltas tras el activador',
    count: 'Cuántas veces se ha ejecutado el comando',
    random: 'Un número aleatorio del 1 al 100, o de tu rango',
    pick: 'Una de las opciones, al azar',
    time: 'La hora (24 h) en una zona horaria; UTC si se omite',
    where: 'La plataforma y el canal de los que vino el mensaje',
    clip: 'Enlace al clip que creó este comando',
    urlfetch: 'El texto con el que responde una dirección web',
  },
  delete: {
    title: '¿Eliminar {trigger}?',
    body: 'El comando deja de responder de inmediato. Su contador de usos se pierde.',
    cancel: 'Conservarlo',
    confirm: 'Eliminar',
    failed: 'No se pudo eliminar el comando.',
  },

  bot: {
    heading: 'Bot y remitente',
    senderBot: 'Las respuestas las envía el All-Chat Bot',
    senderBotHint: 'Funciona en Twitch, Kick y YouTube sin una cuenta aparte.',
    senderSelf: 'Las respuestas las envía tu propia cuenta',
    senderSelfHint:
      'El mensaje aparece como si lo hubieras escrito tú. Requiere el permiso de envío de chat.',
    senderToggleLabel: 'Cambiar quién envía las respuestas: el All-Chat Bot o tu cuenta',
    senderSaveFailed: 'No se pudo cambiar quién envía las respuestas.',
    twitchGranted: 'El All-Chat Bot tiene permiso en tu chat de Twitch.',
    twitchNotGranted: 'El All-Chat Bot necesita tu permiso para publicar en tu chat de Twitch.',
    grant: 'Permitir en el chat de Twitch',
    granting: 'Abriendo Twitch',
    grantFailed: 'No se pudo iniciar el flujo de permisos de Twitch.',
    connectedToast: 'Bot conectado.',
    connectFailedToast: 'No se pudo conectar el bot.',
    customHeading: 'Cuentas de bot personalizadas',
    customIntro:
      'Conecta tu propia cuenta de bot por plataforma. Las respuestas saldrán con ese nombre.',
    customPremium:
      'Las cuentas de bot personalizadas son una función premium. Consulta premium para conectar tu propio bot en Twitch, Kick o YouTube.',
    noneConnected: 'No hay ningún bot personalizado conectado',
    connectedAs: '{name} ({login})',
    connect: 'Conectar bot de {platform}',
    connecting: 'Conectando',
    connectFailed: 'No se pudo iniciar la conexión del bot.',
    disconnect: 'Desconectar',
    disconnecting: 'Desconectando',
    disconnectLabel: 'Desconectar el bot de {platform}',
    disconnected: 'Bot desconectado.',
    disconnectFailed: 'No se pudo desconectar el bot.',
    removeTitle: '¿Desconectar el bot de {platform}?',
    removeBody:
      'Las respuestas vuelven a salir del All-Chat Bot o de tu propia cuenta en esa plataforma.',
    removeCancel: 'Conservarlo',
    removeConfirm: 'Desconectar',
  },

  timers: {
    heading: 'Temporizadores',
    intro:
      'Los temporizadores publican un mensaje en tu chat de Twitch y Kick, y en el chat de YouTube con premium. Dale a cada uno un intervalo en minutos, un número mínimo de mensajes de chat o ambos: con los dos definidos se activa cada X minutos, pero solo si han llegado al menos Y mensajes desde la última publicación. Un temporizador que solo tiene intervalo también publica mientras estás desconectado.',
    new: 'Nuevo temporizador',
    empty: 'Todavía no hay temporizadores.',
    edit: 'Editar',
    delete: 'Eliminar',
    deleteLabel: 'Eliminar {name}',
    enabledLabel: 'Activar {name}',
    toggleFailed: 'No se pudo actualizar el temporizador.',
    deleteFailed: 'No se pudo eliminar el temporizador.',
    saved: 'Temporizador guardado',
    createTitle: 'Nuevo temporizador',
    editTitle: 'Editar {name}',
    dialogIntro:
      'Un temporizador publica su mensaje en tu chat según un horario o según la actividad del chat.',
    nameLabel: 'Nombre',
    nameRequired: 'Ponle un nombre al temporizador.',
    messageLabel: 'Mensaje',
    messageHint:
      'Lo que se publica. Funcionan las variables de plantilla que no necesitan un usuario del chat, como count, random, pick, time y js.',
    messageRequired: 'Escribe el mensaje que se va a publicar.',
    intervalLabel: 'Intervalo (minutos, opcional)',
    intervalHint: 'Entre {min} y {max} minutos. Déjalo vacío para no usar intervalo.',
    messagesLabel: 'Mensajes mínimos (opcional)',
    messagesHint: 'Entre {min} y {max} mensajes. Déjalo vacío para no usar mínimo.',
    combinedRule:
      'Define al menos uno de los dos. Con ambos definidos, el temporizador espera al intervalo y al número mínimo de mensajes desde su última publicación.',
    triggerRequired: 'Define un intervalo, un número mínimo de mensajes o ambos.',
    intervalRange: 'El intervalo debe ser un número entero entre {min} y {max} minutos.',
    messagesRange: 'El mínimo debe ser un número entero entre {min} y {max} mensajes.',
    triggerBoth: 'cada {minutes} minutos, al menos {messages} mensajes desde el último',
    triggerEvery: 'cada {minutes} minutos',
    triggerMessages: 'tras {messages} mensajes nuevos',
    cancel: 'Cancelar',
    save: 'Guardar',
    saving: 'Guardando',
    saveFailed: 'No se pudo guardar el temporizador.',
    deleteTitle: '¿Eliminar {name}?',
    deleteBody: 'El temporizador deja de publicar de inmediato.',
    deleteCancel: 'Conservarlo',
    deleteConfirm: 'Eliminar',
  },
}
