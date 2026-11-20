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

export const auth: LocaleNamespace<'auth'> = {
  link: {
    title: 'Vincular una superficie de control',
    intro:
      'Aprueba un Stream Deck, un StreamController u otra superficie de control de escritorio. No se copia ni se pega nada: la credencial va directamente al plugin, y puedes revocarla cuando quieras desde {devices}.',
    introDevices: 'tus dispositivos vinculados',
    loading: 'Cargando solicitud de vinculación',
    codeTitle: 'Introduce el código de vinculación',
    codeBody:
      'Tu plugin muestra un código de ocho caracteres. Escríbelo aquí. Esta es la vía para una superficie de control que está en un equipo distinto al de este navegador.',
    codeLabel: 'Código de vinculación',
    codePlaceholder: 'ABCD-EFGH',
    codeHint:
      'No se distingue entre mayúsculas y minúsculas, y el guion es opcional. Los códigos caducan a los diez minutos.',
    codeChecking: 'Comprobando…',
    codeContinue: 'Continuar',
    codeInvalid:
      'Ese código no es válido. Revisa el código que muestra tu plugin o vuelve a iniciar la vinculación desde el plugin.',
    approveTitle: 'Un dispositivo quiere controlar tu chat',
    approveBody:
      'Dice que es “{name}”. Ese nombre está {selfReported}: no tenemos forma de verificarlo, así que tómalo como una pista, no como una prueba. Si no acabas de empezar a vincular una superficie de control, deniega esta solicitud.',
    approveSelfReported: 'declarado por el propio plugin',
    overlayLegend: '¿Qué overlay puede controlar?',
    overlayBody:
      'El dispositivo queda asociado a este overlay mientras siga vinculado. Para cambiarlo, revócalo y vuelve a vincularlo.',
    overlayNone: 'Todavía no tienes overlays. Crea uno primero y luego vincula tu dispositivo.',
    scopeLegend: '¿Qué puede hacer?',
    scopeBody:
      'El plugin ha pedido lo que aparece abajo. Desactiva lo que no quieras que tenga: puedes conceder menos de lo que pidió, nunca más.',
    scopeNone:
      'Elige al menos uno: un dispositivo sin ninguno puede iniciar sesión, pero no hacer nada.',
    chatWriteCaveat:
      'Conviene saberlo: el envío de chat no va por overlay. Llega a todas las plataformas que tu cuenta tiene conectadas, así que el overlay que elegiste arriba no limita este permiso. Solo limita las acciones de encuestas y predicciones.',
    scopeChatWriteTitle: 'Enviar mensajes de chat',
    scopeChatWriteBody: 'Permite a este dispositivo publicar mensajes en tus chats conectados.',
    scopeEngagementWriteTitle: 'Gestionar encuestas y predicciones',
    scopeEngagementWriteBody:
      'Permite a este dispositivo abrir, cerrar, resolver y cancelar encuestas y predicciones.',
    scopeUnknownBody: 'Solicitado por este dispositivo.',
    nameLabel: 'Ponle un nombre a este dispositivo',
    namePlaceholder: 'Stream Deck (PC del estudio)',
    nameHint:
      'Aparece en tu lista de dispositivos vinculados, para que sepas qué estás revocando más adelante. Empieza con la sugerencia del propio plugin; cámbialo por lo que quieras.',
    approving: 'Aprobando…',
    approve: 'Aprobar este dispositivo',
    deny: 'Denegar',
    approvedTitle: 'Dispositivo aprobado',
    approvedBody: 'Vuelve a tu plugin: terminará la vinculación por sí solo.',
    deniedTitle: 'Dispositivo denegado',
    approveFailed:
      'No se pudo aprobar este dispositivo. Puede que la solicitud haya caducado: vuelve a iniciar la vinculación desde el plugin.',
    denyFailed:
      'No se pudo denegar esta solicitud. Caducará por sí sola en un máximo de diez minutos.',
    overlaysFailed:
      'No se pudieron cargar tus overlays. Actualiza la página para intentarlo de nuevo.',
    requestExpired:
      'Esta solicitud de vinculación ha caducado o ya se usó. Vuelve a iniciar la vinculación desde el plugin.',
  },
  callback: {
    authenticating: 'Autenticando...',
    noCode: 'No se recibió ningún código de autenticación',
    exchangeFailed:
      'La autenticación ha fallado. Puede que el código haya caducado: inténtalo de nuevo.',
    failed: 'La autenticación ha fallado. Inténtalo de nuevo.',
    duplicateAccount:
      'Tu canal de {platform} ya está conectado a la cuenta de All-Chat "{username}". Inicia sesión con esa cuenta. No necesitas un inicio de sesión de {platform} aparte.',
    returnHome: 'Volver al inicio',
  },
  viewerSuccess: {
    completing: 'Completando la autenticación...',
    pleaseWait: 'Espera un momento',
    succeeded: '¡Autenticación correcta!',
    closeWindow: 'Ya puedes cerrar esta ventana',
    noCode: 'No se recibió ningún código de autenticación',
    exchangeFailed:
      'La autenticación ha fallado. Puede que el código haya caducado: inténtalo de nuevo.',
    viewerInfoFailed: 'No se pudo completar la autenticación. Inténtalo de nuevo.',
  },
  viewerError: {
    title: 'Error de autenticación',
    defaultError: 'La autenticación ha fallado',
    body: 'Se produjo un error al autenticarte con tu {account}. Inténtalo de nuevo o contacta con soporte si el problema continúa.',
    accountNamed: 'cuenta de {platform}',
    accountNeutral: 'cuenta de streaming',
    returnHome: 'Volver al inicio',
    goBack: 'Volver',
  },
}
