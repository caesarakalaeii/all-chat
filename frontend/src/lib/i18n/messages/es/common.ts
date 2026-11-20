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

export const common: LocaleNamespace<'common'> = {
  brand: {
    wordmark: 'all-chat',
  },
  appNav: {
    dashboard: 'Panel',
    flairs: 'Flairs',
    admin: 'Admin',
    settings: 'Ajustes',
    docs: 'Docs',
    translate: 'Traducir',
    commands: 'Comandos',
    logOut: 'Cerrar sesión',
    signIn: 'Iniciar sesión',
  },
  theme: {
    lightMode: 'Modo claro',
  },
  soundPresets: {
    chime: 'Chime',
    pop: 'Pop',
    ping: 'Ping',
  },
  platforms: {
    twitch: 'Twitch',
    youtube: 'YouTube',
    kick: 'Kick',
    tiktok: 'TikTok',
    owncast: 'Owncast',
    goodgame: 'GoodGame',
    picarto: 'Picarto',
    facebook: 'Facebook',
    instagram: 'Instagram',
    altstream: 'Altstream',
    discord: 'Discord',
  },
  overlayKinds: {
    chat: 'Chat',
    alerts: 'Alertas',
    goal: 'Meta',
    list: 'Lista',
  },
  patreon: {
    heading: 'Patreon',
    connect: 'Conectar Patreon',
    connecting: 'Redirigiendo…',
    notAPatron: '¿Aún no eres mecenas?',
    subscribe: 'Suscribirse en Patreon',
    subscriptionRow: 'Suscripción',
    renewsRow: 'Se renueva',
    active: 'Activa',
    inactive: 'Inactiva',
    disconnect: 'Desconectar Patreon',
    disconnectTitle: '¿Desconectar Patreon?',
    disconnectCancel: 'Cancelar',
    disconnectConfirm: 'Sí, desconectar',
    statusActive: 'Activa',
    statusDeclined: 'Pago rechazado',
    statusFormer: 'Finalizada',
    statusNotSubscribed: 'Sin suscripción',
    connectedToast: '¡Patreon conectado!',
    connectFailedToast: 'No se pudo conectar Patreon',
    connectStartFailedToast: 'No se pudo iniciar la conexión con Patreon',
    disconnectedToast: 'Patreon desconectado',
    disconnectFailedToast: 'No se pudo desconectar',
  },
  duration: {
    justNow: 'ahora mismo',
    minutes: '{minutes}m',
    hours: '{hours}h',
    hoursAndMinutes: '{hours}h {minutes}m',
    days: '{days}d',
    daysAndHours: '{days}d {hours}h',
  },
  badges: {
    allChatLabel: 'Insignia de All-Chat',
    premiumLabel: 'Insignia premium',
  },
  dialog: {
    closeLabel: 'Cerrar diálogo',
  },
  toast: {
    closeLabel: 'Cerrar notificación',
    tryAgain: 'Inténtalo de nuevo.',
    elevenLabsFallback: 'ElevenLabs no está disponible: se usa la voz del navegador.',
  },
  splitPane: {
    resizeLabel: 'Redimensionar paneles',
    shrinkConfigLabel: 'Reducir el panel de configuración',
    growConfigLabel: 'Ampliar el panel de configuración',
    previewTitle: 'Vista previa en directo del overlay',
  },
  cssEditor: {
    regionLabel: 'Editor de CSS personalizado',
    keyboardHint:
      'Pulsa Ctrl+M para activar o desactivar la captura de Tab; con Escape y luego Tab sales del editor.',
    loading: 'Cargando editor...',
  },
  impersonation: {
    bannerLabel: 'Modo admin:',
    viewingAs: 'Viendo como {username}',
    exitButton: 'Salir y volver a Admin',
  },
  moderatingElsewhere: {
    channelOne: '{count} canal',
    channelMany: '{count} canales',
    sentence: 'Moderas {channels} de otros streamers.',
    openLink: 'Abrir',
  },
  maintenanceInfo: {
    buttonLabel: 'Avisos del servicio',
    popoverHeading: 'Avisos del servicio',
  },
  eventSubMigration: {
    body: 'La antigua conexión de chat por IRC se va a retirar y puede perder mensajes cuando hay muchos streams en directo. Vuelve a conectar para pasar a la nueva conexión y mantener tu chat fiable.',
    reconnectButton: 'Reconectar ahora',
    dismissLabel: 'Descartar',
    connectedToast: 'Chat de Twitch conectado',
    failedToast: 'No se pudo iniciar la actualización',
  },
  forbidden: {
    heading: '403 Forbidden',
    body: 'No tienes permiso para acceder a esta página. Se necesitan privilegios de administrador.',
    dashboardButton: 'Ir al panel',
  },
  betaWarning: {
    youtubeTitle: 'YouTube: verificación de OAuth en curso',
    tiktokTitle: 'TikTok: beta cerrada',
    youtubeBody:
      'La integración con YouTube está en revisión por la verificación de OAuth de Google. Durante este periodo no podemos añadir nuevos usuarios de prueba.',
    tiktokBody:
      'La integración con TikTok está en beta cerrada. Si todavía no te han añadido al programa beta, la autenticación fallará.',
    youtubeDiscordPrompt:
      'Únete a nuestra comunidad de Discord para seguir el avance de la verificación y recibir ayuda:',
    tiktokDiscordPrompt: 'Para entrar en la beta, únete a nuestra comunidad de Discord:',
    youtubeExistingUser:
      'Si ya te añadieron como usuario de prueba, puedes seguir usando la integración con YouTube.',
    tiktokExistingUser: 'Si ya estás en la beta, puedes continuar con la autenticación.',
    discordLink: 'Unirse al servidor de Discord',
    cancelButton: 'Cancelar',
    continueButton: 'Entendido, continuar',
  },
  breadcrumb: {
    label: 'Ruta de navegación',
    home: 'Inicio',
  },
}
