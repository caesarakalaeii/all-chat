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

export const errors: LocaleNamespace<'errors'> = {
  unauthorized: {
    title: 'Inicio de sesión necesario',
    message: 'Tienes que iniciar sesión para enviar mensajes.',
    step1: 'Haz clic en el botón «Iniciar sesión con {platform}»',
    step2: 'Autoriza a la aplicación a enviar mensajes en tu nombre',
    step3: 'Vuelve a enviar tu mensaje',
  },
  tokenExpired: {
    title: 'Sesión caducada',
    message: 'Tu sesión de autenticación ha caducado.',
    step1: 'Vuelve a iniciar sesión para renovarla',
    step2: 'Asegúrate de autorizar la aplicación',
    step3: 'Vuelve a enviar tu mensaje',
  },
  rateLimited: {
    title: 'Límite de envío alcanzado',
    message: 'Estás enviando mensajes demasiado rápido. Ve más despacio.',
    step1: 'Espera un momento antes de enviar otro mensaje',
    step2: 'Evita enviar mensajes muy seguidos',
    step3: 'El límite se restablece automáticamente',
  },
  banned: {
    title: 'No puedes enviar mensajes',
    message: 'Ahora mismo estás baneado y no puedes enviar mensajes.',
    step1: 'Consulta el motivo del baneo más abajo',
    step2: 'Contacta con el streamer o los moderadores si crees que es un error',
    step3: 'Si el baneo es temporal, espera a que termine',
  },
  streamerOffline: {
    title: 'El stream no está en directo',
    message: 'Este streamer no está en directo ahora mismo.',
    step1: 'Comprueba si el stream ha terminado o aún no ha empezado',
    step2: 'Recarga la página para actualizar el estado del stream',
    step3: 'Solo puedes enviar mensajes cuando el stream está en directo',
  },
  platformApiError: {
    title: 'Error de la plataforma',
    message: 'La plataforma de streaming ha encontrado un error.',
    step1: 'Lo más probable es que sea un problema temporal de la plataforma',
    step2: 'Espera un momento y vuelve a intentarlo',
    step3: 'Comprueba si la plataforma está sufriendo caídas',
    step4: 'Vuelve a enviar tu mensaje dentro de unos instantes',
  },
  networkError: {
    title: 'Error de conexión',
    message: 'No se ha podido conectar con el servidor.',
    step1: 'Comprueba tu conexión a internet',
    step2: 'Prueba a recargar la página',
    step3: 'Si el problema continúa, puede que el servidor esté teniendo problemas',
  },
  validationError: {
    title: 'Mensaje no válido',
    message: 'Tu mensaje no cumple los requisitos.',
    step1: 'Comprueba que tu mensaje no esté vacío',
    step2: 'Asegúrate de que tu mensaje no sea demasiado largo',
    step3: 'Evita usar caracteres o contenido prohibidos',
  },
  unknownError: {
    title: 'Error inesperado',
    message: 'Se ha producido un error inesperado al enviar tu mensaje.',
    step1: 'Vuelve a enviar tu mensaje',
    step2: 'Recarga la página si el problema continúa',
    step3: 'Contacta con soporte si el error sigue apareciendo',
  },

  display: {
    iconLabel: 'Icono de error',
    dismissLabel: 'Descartar error',

    rateLimitCountdown: 'Podrás enviar otro mensaje en {countdown}',
    reasonLabel: 'Motivo:',
    expiresLabel: 'Caduca:',
    platformMessage: 'Mensaje de la plataforma: {message}',

    whatYouCanDo: 'Qué puedes hacer:',
    tryAgain: 'Reintentar',
    signInWith: 'Iniciar sesión con {platform}',
    showDetails: 'Mostrar detalles',
    hideDetails: 'Ocultar detalles',
  },
}
