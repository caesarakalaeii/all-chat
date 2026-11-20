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

export const moderation: LocaleNamespace<'moderation'> = {
  actions: {
    delete: 'Eliminar mensajes',
    timeout: 'Expulsión temporal',
    ban: 'Banear',
    unban: 'Desbanear',
  },
  readiness: {
    notAModerator:
      'Aún no es moderador en {platform}: añádelo desde las herramientas de {platform}.',
    needsConsent: 'Esperando a que conecte su cuenta de {platform}.',
    needsDiscordLink: 'Esperando a que vincule Discord.',
    unavailable: 'No se ha podido comprobar {platform} ahora mismo.',
  },
  status: {
    pending: 'Invitación pendiente',
    suspended: 'En pausa tras 90 días de inactividad',
    revoked: 'Eliminado',
    active: 'Activo',
  },
  unnamedInvite: 'Invitación sin nombre',
  roster: {
    loadFailed: 'No se han podido cargar los moderadores de este overlay.',
    tryAgain: 'Reintentar',
    loading: 'Cargando moderadores…',
    explainer:
      'Los moderadores actúan con {emphasis} cuentas de plataforma, así que Twitch, YouTube y Kick comprueban su rol de moderador en cada acción. Eliminar a alguien aquí tiene efecto inmediato.',
    explainerEmphasis: 'sus propias',
    inviteButton: 'Invitar a un moderador',
    usage: '{used} de {cap} usados',
    removeAll: 'Eliminar a todos',
    atCap:
      'Este overlay ha llegado a su límite de {cap} moderadores. Elimina a alguien para invitar a otra persona.',
    empty:
      'Nadie modera este overlay todavía. Invita a alguien y podrá eliminar mensajes y expulsar temporalmente a espectadores desde la vista de monitor, con sus propias cuentas de plataforma.',
  },
  row: {
    legChangeFailed: 'No se ha podido cambiar {platform} para {name}.',
    needsOneAction: '{name} necesita al menos una acción. Si no, elimínalo.',
    updateFailed: 'No se ha podido actualizar a {name}.',
    removed: 'Has eliminado a {name}.',
    removeFailed: 'No se ha podido eliminar a {name}.',
    removeAllFailed: 'No se ha podido eliminar a todos. Inténtalo de nuevo.',
    removeButton: 'Eliminar',
    removeLabel: 'Eliminar a {name}',
    forAccount: 'para {account}',
    platformToggleLabel: 'Moderación de {platform} para {name}',
    removedCountOne: 'Se ha eliminado {count} moderador.',
    removedCountMany: 'Se han eliminado {count} moderadores.',
  },
  revoke: {
    title: '¿Eliminar a {name}?',
    description:
      'Perderá el acceso a la moderación de este overlay en su próxima acción. Sus acciones anteriores se conservan en el registro.',
    cancel: 'Cancelar',
    confirm: 'Eliminar',
  },
  revokeAll: {
    title: '¿Eliminar a todos los moderadores?',
    description:
      'Esto elimina a todas las personas de este overlay, incluidas las invitaciones que nadie ha aceptado aún. Después podrás volver a invitar a quien quieras.',
    confirm: 'Eliminar a todos',
  },
  invite: {
    title: 'Invitar a un moderador',
    sendLink:
      'Envía este enlace a la persona que quieres que modere. Funciona una sola vez, caduca en 7 días y {emphasis}: si se pierde, crea una invitación nueva.',
    sendLinkEmphasis: 'no se volverá a mostrar',
    copied: 'Copiado',
    copyLink: 'Copiar enlace',
    done: 'Listo',
    acceptExplainer:
      'La acepta con su propia cuenta de All-Chat y luego conecta cada plataforma la primera vez que modera en ella.',
    labelPrompt: '¿Para quién es? (opcional)',
    labelPlaceholder: 'Sarah, mi mod de Twitch',
    actionsLegend: '¿Qué puede hacer?',
    platformsLegend: '¿En qué plataformas? Desactivada significa que no puede actuar allí.',
    cancel: 'Cancelar',
    creating: 'Creando…',
    create: 'Crear invitación',
    capReached: 'Este overlay ya tiene el número máximo de moderadores.',
    createFailed: 'No se ha podido crear la invitación. Inténtalo de nuevo.',
    boundToAccount: 'Esa invitación pertenece a {account}.',
    copyFailed: 'No se ha podido copiar. Selecciona el enlace y cópialo a mano.',
    premiumGate:
      'Delegar la moderación forma parte de All-Chat premium. Tus moderadores nunca pagan: solo cuenta tu propio plan.',
    upgradeLink: 'Hazte premium para invitar a moderadores',
  },
  accept: {
    heading: 'Invitación de moderación',
    loadingInvite: 'Cargando invitación',
    goToChannels: 'Ir a tus canales',
    accept: 'Aceptar y empezar a moderar',
    accepting: 'Aceptando…',
    notNow: 'Ahora no',
    actionDelete: 'Eliminar mensajes',
    actionTimeout: 'Expulsar temporalmente a espectadores',
    actionBan: 'Banear a espectadores',
    actionUnban: 'Levantar baneos y expulsiones temporales',
    signInHeading: 'Inicia sesión para aceptar esta invitación',
    signInBody:
      'La moderación va ligada a una cuenta de All-Chat, así que necesitamos saber a cuál asignarla. Inicia sesión y vuelve a abrir el enlace de invitación: sigue siendo válido.',
    signIn: 'Iniciar sesión',
    askingToHelp: '{owner} te pide que le ayudes a moderar',
    addressedTo: 'Ha dirigido esta invitación a “{label}”.',
    actionsHeading: 'Lo que podrías hacer',
    platformsHeading: 'En estas plataformas',
    noPlatforms: 'Ninguna todavía: {owner} aún tiene que activar una plataforma.',
    ownAccountNote:
      'Actuarás con tu propia cuenta de plataforma, así que cada plataforma sigue comprobando que {owner} te ha hecho moderador allí. Ahora no tienes que hacer nada: conectas una plataforma la primera vez que moderas en ella.',
    expectedAccount: 'Esta invitación es para {account}.',
    expectedAccountOnPlatform: 'Esta invitación es para {account} en {platform}.',
    errorMissingToken:
      'A este enlace le falta el código de invitación. Pídele al streamer que te lo envíe de nuevo.',
    errorNotFound:
      'Esta invitación ya no es válida: puede que ya se haya usado o que el streamer la haya retirado. Pídele una nueva.',
    errorExpired: 'Esta invitación ha caducado. Pídele una nueva al streamer.',
    errorAlreadyModerator: 'Ya moderas este canal. Está en tu página de canales.',
    errorOwnerCannotAccept: 'Este overlay es tuyo: ya tienes la moderación completa.',
    errorBoundToOther:
      'Esta invitación es para una cuenta concreta. Inicia sesión con esa cuenta o pídele al streamer que envíe una invitación nueva para esta.',
    errorBoundToOtherAccount:
      'Esta invitación es para una cuenta concreta ({account}). Inicia sesión con esa cuenta o pídele al streamer que envíe una invitación nueva para esta.',
    errorBoundToOtherPlatform:
      'Esta invitación es para una cuenta concreta de {platform}. Inicia sesión con esa cuenta o pídele al streamer que envíe una invitación nueva para esta.',
    errorBoundToOtherBoth:
      'Esta invitación es para una cuenta concreta de {platform} ({account}). Inicia sesión con esa cuenta o pídele al streamer que envíe una invitación nueva para esta.',
    errorUnknown: 'No se ha podido abrir esta invitación. Revisa el enlace e inténtalo de nuevo.',
  },
  channels: {
    heading: 'Canales que moderas',
    subheading:
      'Overlays que otros streamers te han confiado. Actúas con tu propia cuenta de plataforma, así que cada plataforma sigue comprobando que eres uno de sus moderadores.',
    loading: 'Cargando canales',
    loadFailed: 'No se han podido cargar tus canales.',
    tryAgain: 'Reintentar',
    emptyHeading: 'Aún no hay canales',
    emptyBody:
      'Cuando un streamer te invita a moderar su overlay, te envía un enlace privado. Ábrelo con la sesión iniciada en esta cuenta y su canal aparecerá aquí.',
    forOwner: 'para {owner}',
    noPlatforms: 'Aún no hay plataformas activadas: pídele a {owner} que active una.',
    suspendedNote: 'En pausa tras 90 días sin acciones. Pídele a {owner} que lo reactive.',
    unavailableNote:
      'El plan de {owner} no incluye la moderación ahora mismo, así que las acciones no están disponibles hasta que lo renueve.',
    openMonitor: 'Abrir monitor de chat',
    discordPromptBody:
      'Vincula tu cuenta de Discord para moderar Discord. All-Chat comprueba tus propios permisos en el servidor antes de actuar, así que necesita saber cuál es tu cuenta de Discord.',
    linkDiscord: 'Vincular Discord',
    noticeDiscordAlreadyLinked:
      'Esa cuenta de Discord ya está vinculada a otra cuenta de All-Chat. Vincula una distinta o desvincúlala antes de la otra cuenta.',
    noticeConnectFailed:
      'La conexión no se ha completado. Abre un canal de abajo e inténtalo de nuevo desde allí.',
    noticeDiscordLinked:
      'Cuenta de Discord vinculada. All-Chat ya puede comprobar tus propios permisos en el servidor cuando moderes Discord.',
    noticeConnected:
      '{platform} conectado. Ahora cubre todos los canales que te han delegado {platform}.',
  },
}
