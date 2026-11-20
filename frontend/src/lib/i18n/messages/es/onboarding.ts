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
    createOverlay: 'Crea tu overlay',
    connectSource: 'Conecta una fuente de chat',
    chooseTheme: 'Elige un tema',
    copyObs: 'Añádelo a OBS',
  },
  checklist: {
    title: 'Guía de configuración',
    allDone: '¡Todos los pasos completados!',
    progress: '{done} de {total} pasos completados',
    expand: 'Expandir la guía de configuración',
    minimize: 'Minimizar la guía de configuración',
    dismiss: 'Descartar la guía de configuración',
    // The leading space separates this from the step label for screen readers.
    stepDone: ' (completado)',
    create: 'Crear',
    showMe: 'Enséñamelo',
    openEditor: 'Abrir editor',
    copyLink: 'Copiar enlace',
    copied: '¡Copiado!',
  },
  obs: {
    heading: 'En OBS:',
    step1: 'En OBS, haz clic en {plus} debajo de Fuentes y elige {browser}.',
    step1Plus: '+',
    step1Browser: 'Navegador',
    step2: 'Pega el enlace del overlay que has copiado en el campo URL.',
    step3:
      'Ajusta el tamaño de la fuente al área que debe ocupar el chat (un recuadro alto y estrecho como 450×800 funciona bien, no todo tu lienzo) y arrástrala a su sitio. El chat aparece en cuanto el overlay se conecta.',
  },
  obsDock: {
    step1: 'En OBS, abre {menu}.',
    step1Menu: 'Vista → Paneles → Paneles de navegador personalizados',
    step2:
      'Ponle un nombre al dock, pega el enlace del dock que has copiado en el campo URL, haz clic en Aplicar y cierra el diálogo.',
    step3:
      'El dock tiene su propio inicio de sesión: inicia sesión una vez dentro del panel y se mantiene. Arrastra el panel adonde quieras; una columna estrecha junto a Escenas y Fuentes funciona bien.',
  },
  extras: {
    heading: 'Opcional: ve más allá',
    ttsTitle: 'Texto a voz',
    ttsBody:
      'Lee el chat en voz alta. Las voces del navegador son gratis; las voces de ElevenLabs son Premium.',
    moderationTitle: 'Modera desde tu overlay',
    moderationBody:
      'Elimina mensajes, expulsa temporalmente, banea y desbanea desde el botón Vista de monitor, en la parte superior del editor. Controles completos en Twitch, Kick y Discord; expulsión temporal y baneo en YouTube. (Premium)',
    moderatorsTitle: 'Deja que tus mods te ayuden',
    moderatorsBody:
      'Da acceso a la Vista de monitor a tus moderadores actuales en Moderadores. Actúan con sus propias cuentas de plataforma y nunca necesitan Premium. (Premium)',
    sharedChatTitle: 'Chat compartido',
    sharedChatBody:
      'Combina varios canales en una sola conversación con el botón Compartir overlay. (Premium)',
    streamSelectionTitle: 'Selección de directo de YouTube',
    streamSelectionBody:
      'Elige exactamente qué emisión escucha un overlay, por cada fuente de YouTube en Fuentes. (Premium)',
    youtubeOfficialApiTitle: 'Directos de YouTube ocultos y solo para miembros',
    youtubeOfficialApiBody:
      'Activa API oficial de YouTube en una fuente de YouTube en Fuentes y tu propio canal se leerá a través de tu conexión de YouTube: los directos ocultos y solo para miembros se encuentran automáticamente. (Premium)',
    bubbleColorsTitle: 'Burbujas de distintos colores',
    bubbleColorsBody:
      'Dale a cada plataforma su propio color de burbuja, o alterna una paleta a lo largo del feed, en Colores de burbuja dentro de Apariencia. Gratis.',
    adBreaksTitle: 'Avisos de pausas publicitarias de Twitch',
    adBreaksBody:
      'Actividad y eventos, en el monitor, muestra cuándo empieza una pausa publicitaria de Twitch, programada o manual, para que puedas hacer una pausa. Actívalo desde el banner del monitor. Gratis.',
    alertsOverlayTitle: 'Overlays de alertas',
    alertsOverlayBody:
      'Muestra en el stream suscripciones, bits, raids, Super Chats y regalos de TikTok como una fuente propia de OBS, con un botón de prueba para colocarlos. Gratis.',
    alertsEditorTitle: 'Editor de alertas personalizadas',
    alertsEditorBody:
      'Dale a cada alerta su propio texto, GIF y sonido, busca GIF directamente en el editor o escribe tu propio HTML, CSS y JavaScript para ella. Gratis.',
    goalOverlayTitle: 'Overlays de objetivos',
    goalOverlayBody:
      'Una barra de progreso hacia un objetivo de seguidores, suscriptores, bits o Super Chat que se llena en directo en el stream.',
    listOverlayTitle: 'Overlays de clasificación',
    listOverlayBody:
      'Muestra en el stream la clasificación de quienes más bits, regalos o Super Chats han enviado en los últimos 30 días, actualizada con cada uno nuevo.',
    mediaUploadsTitle: 'Sube tus propios archivos',
    mediaUploadsBody:
      'Premium: sube tus propias imágenes, GIF y sonidos para alertas y alertas de comandos. Pegar un enlace y buscar GIF siguen siendo gratis.',
    chatCommandsTitle: 'Comandos de chat',
    chatCommandsBody:
      '!comandos personalizados y activadores por palabra clave para todos los chats: responde con tu cuenta, crea clips de tu stream de Twitch al momento, muestra tu propia alerta (imagen, sonido, texto) en tu overlay de alertas y usa variables, consultas web o JavaScript en las respuestas. Las respuestas en el chat de YouTube son premium.',
    youtubeCommandRepliesTitle: 'Respuestas de comandos en el chat de YouTube',
    youtubeCommandRepliesBody:
      'Premium: tus comandos de chat y temporizadores también responden en el chat de YouTube, no solo en Twitch y Kick.',
    customChatBotTitle: 'Tu propio bot de chat',
    customChatBotBody:
      'Premium: conecta tu propia cuenta de bot en Twitch, Kick o YouTube, y las respuestas de comandos y los temporizadores saldrán con el nombre de tu bot.',
    flairsTitle: 'Flairs de espectador',
    flairsBody:
      'Cosméticos premium para los usuarios del chat, como degradados de nombre animados, en Flairs dentro de la navegación.',
    streamDeckTitle: 'Botones de Stream Deck',
    streamDeckBody:
      'Controla encuestas, predicciones y mensajes predefinidos desde un botón físico. Vincula un Stream Deck o StreamController en Dispositivos vinculados, sin copiar ni pegar nada. Iniciar una encuesta o una predicción sigue siendo Premium.',
    languageTitle: 'Usa All-Chat en tu idioma',
    languageBody:
      'Cambia el idioma de la interfaz en Ajustes. Están disponibles alemán, francés y español, y la elección se recuerda en cada navegador.',
    translateTitle: 'Ayuda a traducir All-Chat',
    translateBody:
      'Los beta testers pueden traducir la interfaz a otros idiomas en Traducir, dentro de la navegación. Todo se revisa antes de publicarse.',
    seePremium: 'Ver todo lo que incluye Premium',
    done: 'Hecho',
    skip: 'Omitir',
  },
  finale: {
    title: '¡Ya estás en directo! 🎉',
    body: '¿Preguntas, comentarios o peticiones de temas? Nuestra comunidad estará encantada de ayudarte.',
    joinDiscord: 'Únete al Discord',
    finish: 'Finalizar',
  },
  dismissDialog: {
    title: '¿Ocultar la guía de configuración?',
    body: 'Puedes reiniciarla cuando quieras desde Ajustes → Guía de configuración.',
    keep: 'Conservarla',
    hide: 'Ocultar guía',
  },
  createDialog: {
    title: 'Crea tu overlay',
    body: 'Ponle un nombre: conectarás tu chat justo después.',
    nameLabel: 'Nombre del overlay',
    cancel: 'Cancelar',
    submit: 'Crear overlay',
    submitting: 'Creando…',
    failedToast: 'No se pudo crear el overlay',
  },
}
