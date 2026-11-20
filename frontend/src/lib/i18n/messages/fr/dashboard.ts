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
    newOverlay: 'Nouvel overlay',
    loading: 'Chargement des overlays',
    extensionBadge: 'Extension',
    deactivateExtension: "Désactiver l'extension",
    setAsExtension: "Définir comme overlay de l'extension",
    deleteLabel: 'Supprimer {name}',
    sourceCountOne: '{count} source',
    sourceCountOther: '{count} sources',
  },
  discoveryPaused: {
    title: 'Détection YouTube en pause',
    body: "Aucun stream en direct n'a été trouvé pendant une heure, All-Chat a donc arrêté de chercher. Relancez la détection depuis le moniteur de chat quand vous passez en live.",
    action: 'Ouvrir le moniteur de chat',
  },
  empty: {
    heading: 'Aucun overlay pour le moment',
    body: 'Créez votre premier overlay pour voir le chat de toutes vos plateformes au même endroit.',
    createFirst: 'Créer votre premier overlay',
  },
  deleteOverlay: {
    title: 'Supprimer « {name} » ?',
    description: 'Cette action est irréversible. Toutes les sources seront supprimées.',
    cancel: 'Annuler',
    confirm: 'Supprimer',
  },
  toasts: {
    overlayDeleted: 'Overlay supprimé',
    overlayDeleteFailed: "Échec de la suppression de l'overlay",
    extensionOverlayUpdated: "Overlay de l'extension mis à jour",
    extensionOverlayDeactivated: "Overlay de l'extension désactivé",
    overlayUpdateFailed: "Échec de la mise à jour de l'overlay",
  },
  shares: {
    heading: 'Demandes de partage',
    tabPending: 'En attente ({count})',
    tabHistory: 'Historique ({count})',
    loading: 'Chargement des demandes...',
    emptyPending: 'Aucune demande de partage en attente',
    emptyHistory: 'Aucun historique de demandes',
    unknownUser: 'Utilisateur inconnu',
    statusPending: 'En attente',
    statusAccepted: 'Actif',
    statusExpired: 'Expiré',
    statusRevoked: 'Révoqué',
    statusRejected: 'Refusé',
    loadingUser: "Chargement des informations de l'utilisateur...",
    revoke: 'Révoquer',
    accept: 'Accepter',
    reject: 'Refuser',
    userFallbackName: 'Utilisateur',
    acceptTitle: '{sender} souhaite partager avec vous',
    cannotAcceptTitle: "Impossible d'accepter le partage",
    noOverlaysError: "Créez d'abord un overlay pour accepter des partages",
    loadOverlaysFailed: 'Échec du chargement des overlays',
    close: 'Fermer',
    loadingOverlays: 'Chargement des overlays...',
    shareBackLabel: 'Quel overlay partager en retour ? {required}',
    requiredMarker: '*',
    cancel: 'Annuler',
    acceptButton: 'Accepter',
    accepting: 'Acceptation...',
    expiryLegend: 'Quand le partage doit-il expirer ?',
    expiryThisStream: 'Ce stream',
    expiryThisStreamHint: 'Expire à la fin de votre stream',
    expiryKickUnavailable:
      "(indisponible pour Kick : la détection du stream n'est pas encore prise en charge)",
    expiryCustom: 'Durée personnalisée',
    expiryCustomPlaceholder: 'heures',
    expiryCustomLabel: 'Durée personnalisée en heures',
    expiryCustomHint: 'heures (1-168)',
    expiryCustomError: 'Doit être compris entre 1 et 168 heures',
    expiryUnlimited: 'Illimité',
    expiryUnlimitedHint: "N'expire jamais",
    addSourceTitle: "Ajouter l'overlay de {sender} à l'un des vôtres ?",
    addSourcePreview: 'Overlay de {sender} (chat partagé)',
    addSourceSelectLabel: "À quel overlay l'ajouter ?",
    addSourceSkip: 'Passer',
    addSourceAdd: 'Ajouter',
    addSourceAdding: 'Ajout...',
    revokeTitle: 'Révoquer le partage avec {partner} ?',
    revokeBody: 'La transmission des messages cessera immédiatement.',
    revokeCancel: 'Annuler',
    revokeConfirm: 'Révoquer',
    revoking: 'Révocation...',
    loadRequestsFailed: 'Échec du chargement des demandes de partage',
    retry: 'Réessayer',
    notificationUpdateFailed: 'Échec de la mise à jour du statut de notification',
    acceptedToast: 'Partage de {sender} accepté !',
    // Mid-sentence, so lowercase.
    acceptedToastUnknownSender: "l'utilisateur",
    circularTitle: "Impossible d'accepter",
    circularBody: 'Cela créerait une dépendance de partage circulaire',
    addSourceToast: 'Overlay de {sender} ajouté !',
    revokedToast: 'Partage révoqué',
    revokeFailedToast: 'Échec de la révocation du partage',
  },
}
