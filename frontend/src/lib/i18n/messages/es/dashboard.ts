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

export const dashboard: LocaleNamespace<'dashboard'> = {
  overlays: {
    heading: 'Overlays',
    newOverlay: 'Nuevo overlay',
    loading: 'Cargando overlays',
    extensionBadge: 'Extensión',
    deactivateExtension: 'Desactivar extensión',
    setAsExtension: 'Usar como overlay de la extensión',
    deleteLabel: 'Eliminar {name}',
    sourceCountOne: '{count} fuente',
    sourceCountOther: '{count} fuentes',
  },
  discoveryPaused: {
    title: 'Detección de YouTube en pausa',
    body: 'No se encontró ningún directo durante una hora, así que All-Chat dejó de buscar. Vuelve a lanzar la detección desde el monitor de chat cuando empieces tu directo.',
    action: 'Abrir monitor de chat',
  },
  empty: {
    heading: 'Todavía no hay overlays',
    body: 'Crea tu primer overlay para ver el chat de todas tus plataformas en un solo lugar.',
    createFirst: 'Crea tu primer overlay',
  },
  deleteOverlay: {
    title: '¿Eliminar “{name}”?',
    description: 'Esta acción no se puede deshacer. Se quitarán todas las fuentes.',
    cancel: 'Cancelar',
    confirm: 'Eliminar',
  },
  toasts: {
    overlayDeleted: 'Overlay eliminado',
    overlayDeleteFailed: 'No se pudo eliminar el overlay',
    extensionOverlayUpdated: 'Overlay de la extensión actualizado',
    extensionOverlayDeactivated: 'Overlay de la extensión desactivado',
    overlayUpdateFailed: 'No se pudo actualizar el overlay',
  },
  shares: {
    heading: 'Solicitudes para compartir',
    tabPending: 'Pendientes ({count})',
    tabHistory: 'Historial ({count})',
    loading: 'Cargando solicitudes...',
    emptyPending: 'No hay solicitudes pendientes',
    emptyHistory: 'No hay historial de solicitudes',
    unknownUser: 'Usuario desconocido',
    statusPending: 'Pendiente',
    statusAccepted: 'Activa',
    statusExpired: 'Caducada',
    statusRevoked: 'Revocada',
    statusRejected: 'Rechazada',
    loadingUser: 'Cargando datos del usuario...',
    revoke: 'Revocar',
    accept: 'Aceptar',
    reject: 'Rechazar',
    userFallbackName: 'Usuario',
    acceptTitle: '{sender} quiere compartir contigo',
    cannotAcceptTitle: 'No se puede aceptar',
    noOverlaysError: 'Crea primero un overlay para aceptar solicitudes',
    loadOverlaysFailed: 'No se pudieron cargar los overlays',
    close: 'Cerrar',
    loadingOverlays: 'Cargando overlays...',
    shareBackLabel: '¿Qué overlay quieres compartir a cambio? {required}',
    requiredMarker: '*',
    cancel: 'Cancelar',
    acceptButton: 'Aceptar',
    accepting: 'Aceptando...',
    expiryLegend: '¿Cuándo debe caducar?',
    expiryThisStream: 'Este stream',
    expiryThisStreamHint: 'Caduca cuando termina tu stream',
    expiryKickUnavailable:
      '(no disponible para Kick: la detección de streams aún no es compatible)',
    expiryCustom: 'Duración personalizada',
    expiryCustomPlaceholder: 'horas',
    expiryCustomLabel: 'Duración personalizada en horas',
    expiryCustomHint: 'horas (1-168)',
    expiryCustomError: 'Debe estar entre 1 y 168 horas',
    expiryUnlimited: 'Sin límite',
    expiryUnlimitedHint: 'No caduca nunca',
    addSourceTitle: '¿Añadir el overlay de {sender} a uno de los tuyos?',
    addSourcePreview: 'Overlay de {sender} (chat compartido)',
    addSourceSelectLabel: '¿A qué overlay quieres añadirlo?',
    addSourceSkip: 'Omitir',
    addSourceAdd: 'Añadir',
    addSourceAdding: 'Añadiendo...',
    revokeTitle: '¿Revocar el chat compartido con {partner}?',
    revokeBody: 'Esto detendrá la entrega de mensajes de inmediato.',
    revokeCancel: 'Cancelar',
    revokeConfirm: 'Revocar',
    revoking: 'Revocando...',
    loadRequestsFailed: 'No se pudieron cargar las solicitudes',
    retry: 'Reintentar',
    notificationUpdateFailed: 'No se pudo actualizar el estado de la notificación',
    acceptedToast: '¡Solicitud de {sender} aceptada!',
    // Mid-sentence, so lowercase.
    acceptedToastUnknownSender: 'un usuario',
    circularTitle: 'No se puede aceptar',
    circularBody: 'Esto crearía una dependencia circular entre overlays compartidos',
    addSourceToast: '¡Overlay de {sender} añadido!',
    revokedToast: 'Chat compartido revocado',
    revokeFailedToast: 'No se pudo revocar el chat compartido',
  },
}
