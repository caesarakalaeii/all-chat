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
    heading: 'Ambassadeur',
    body: "Vous êtes ambassadeur All-Chat. Choisissez si vous souhaitez être mis en avant sur la page d'accueil publique.",
    featureToggle: "Me mettre en avant sur la page d'accueil",
    cardReads: 'Votre carte affiche : « {tagline} »',
    toastFeatured: "Vous apparaîtrez désormais sur la page d'accueil",
    toastUnfeatured: "Retiré de la vitrine de la page d'accueil",
    toastFailed: 'Échec de la mise à jour du réglage de la vitrine',
  },
  language: {
    heading: 'Langue',
    body: "Choisissez la langue qu'All-Chat utilise dans ce navigateur. Les pages publiques et les overlays affichés dans OBS restent en anglais.",
    selectLabel: "Langue de l'interface",
    betaNote:
      "D'autres langues sont en test bêta. Les bêta-testeurs et les ambassadeurs peuvent déjà les choisir.",
    checkFailed:
      "Impossible de vérifier les langues que vous pouvez utiliser. L'anglais est toujours disponible.",
  },
  connections: {
    heading: 'Plateformes connectées',
    body: 'All-Chat conserve un jeton OAuth pour chaque plateforme que vous avez connectée. Déconnecter une plateforme supprime ses jetons.',
    loading: 'Chargement des plateformes connectées',
    loadFailed: 'Impossible de charger vos plateformes connectées.',
    empty: 'Aucune plateforme connectée.',
    signInBadge: 'Connexion',
    signInNote:
      'Vous vous connectez avec {platform}, cette plateforme ne peut donc pas être déconnectée ici. Ses jetons sont supprimés lorsque vous supprimez votre compte.',
    disconnect: 'Déconnecter',
    disconnectLabel: 'Déconnecter {platform}',
    disconnecting: 'Déconnexion…',
    disconnectTitle: 'Déconnecter {platform} ?',
    effectTwitch:
      "Cette action supprime vos jetons Twitch et demande à Twitch de les révoquer. Vos sources Twitch restent sur vos overlays mais ne reçoivent plus le chat, et l'envoi de messages et la modération sur Twitch s'arrêtent, jusqu'à ce que vous reconnectiez Twitch. D'ici là, vous ne pouvez pas non plus vous connecter à ce compte avec Twitch.",
    effectYoutube:
      "Cette action supprime vos jetons YouTube et demande à Google de les révoquer. Vos sources YouTube continuent de lire les streams publics, ce qui ne nécessite aucun jeton. Les streams non répertoriés et réservés aux membres, les alertes d'abonnés, l'envoi de messages et la modération sur YouTube s'arrêtent jusqu'à ce que vous reconnectiez YouTube. D'ici là, vous ne pouvez pas non plus vous connecter à ce compte avec YouTube.",
    effectKick:
      "Cette action supprime vos jetons Kick et demande à Kick de les révoquer. Vos sources Kick continuent de recevoir le chat. L'envoi de messages et la modération sur Kick s'arrêtent jusqu'à ce que vous reconnectiez Kick. D'ici là, vous ne pouvez pas non plus vous connecter à ce compte avec Kick.",
    effectFacebook:
      'Cette action supprime les jetons de votre Page Facebook et retire vos sources Facebook de vos overlays. Meta ne nous offre aucun moyen de révoquer le jeton, retirez donc aussi All-Chat des applications connectées à votre compte Facebook.',
    effectInstagram:
      'Cette action supprime vos jetons Instagram et retire vos sources Instagram de vos overlays. Meta ne nous offre aucun moyen de révoquer le jeton, retirez donc aussi All-Chat des applications connectées à votre compte Facebook.',
    disconnectCancel: 'Annuler',
    disconnectConfirm: 'Oui, déconnecter',
    disconnectedToast: '{platform} déconnecté',
    disconnectFailedToast: 'Impossible de déconnecter {platform}',
  },
  dataExport: {
    download: 'Télécharger mes données',
    downloading: 'Préparation de votre fichier…',
    failedToast: 'Impossible de télécharger vos données',
    viewerHeading: 'Vos données',
    viewerBody:
      "Téléchargez un fichier JSON contenant tout ce qu'All-Chat stocke sur votre compte spectateur : plateformes liées, sessions, cosmétiques, points, votes, prédictions et messages récents.",
  },
  index: {
    heading: 'Paramètres',
    profileHeading: 'Profil',
    usernameLabel: "Nom d'utilisateur",
    primaryPlatformLabel: 'Plateforme principale',
    primaryPlatformUnknown: 'Inconnue',
    setupGuideHeading: 'Guide de configuration',
    setupGuideBody:
      "Reprenez la configuration de l'overlay pas à pas : créer un overlay, connecter le chat, choisir un thème et obtenir le lien OBS.",
    setupGuideRestart: 'Recommencer',
    setupGuideRestarting: 'Redémarrage…',
    premiumHeading: 'Premium',
    premiumBody: 'Débloquez les fonctionnalités premium en soutenant All-Chat sur Patreon.',
    premiumManage: 'Gérer Premium',
    devicesHeading: 'Appareils associés',
    devicesBody:
      'Consultez et révoquez les surfaces de contrôle Stream Deck et StreamController associées à votre compte. Chacune est verrouillée sur un seul overlay.',
    devicesManage: 'Gérer les appareils',
    tokensHeading: 'Jetons API',
    tokensBody:
      "Créez et révoquez des jetons d'accès personnels. Utilisez-les là où l'association n'est pas possible : un boîtier de capture sans écran, un second PC ou un script.",
    tokensManage: 'Gérer les jetons',
    privacyHeading: 'Données et confidentialité',
    privacyBody:
      'Nous limitons la collecte de données au minimum et en toute transparence. Consultez les documents ci-dessous pour savoir comment les jetons, les overlays et les métadonnées du chat sont traités.',
    privacyPolicyLink: 'Politique de confidentialité',
    termsLink: "Conditions d'utilisation",
    discordHeading: 'Discord',
    discordServersLoading: 'Chargement des serveurs Discord',
    discordNoServer: 'Aucun serveur Discord connecté.',
    discordConnectServer: 'Connecter un serveur Discord',
    discordConnectAnother: 'Connecter un autre serveur',
    discordDisconnect: 'Déconnecter',
    discordDisconnectTitle: 'Déconnecter {guild} ?',
    discordDisconnectBody: 'Toutes les sources Discord connectées à {guild} seront supprimées.',
    discordDisconnectCancel: 'Annuler',
    discordDisconnectConfirm: 'Oui, déconnecter',
    discordAccountHeading: 'Votre compte Discord',
    discordAccountLoading: 'Chargement de la liaison de votre compte Discord',
    discordAccountLinked:
      'Lié en tant que {username}. Nécessaire pour que les modérateurs puissent agir sur vos serveurs Discord.',
    discordAccountFallbackName: 'votre compte Discord',
    discordAccountUnlinked:
      "Non lié. Liez-le pour permettre aux modérateurs que vous invitez d'agir sur vos serveurs Discord.",
    discordAccountUnlink: 'Dissocier',
    discordAccountLink: 'Lier le compte Discord',
    dangerHeading: 'Zone de danger',
    dangerBody:
      'La suppression de votre compte efface tous les overlays, les autorisations OAuth et les sources de chat en cache. Cette action est définitive et irréversible.',
    deleteAccount: 'Supprimer le compte',
    deleteConfirmTitle: 'Supprimer votre compte ?',
    deleteConfirmBody:
      'Cette action supprime définitivement votre compte et tous vos overlays. Elle est irréversible.',
    deleteCancel: 'Annuler',
    deleteConfirm: 'Oui, supprimer mon compte',
    restartGuideFailedToast: 'Impossible de relancer le guide de configuration',
    accountDeletedToast: 'Compte supprimé',
    accountDeleteFailedToast: 'Échec de la suppression du compte',
    discordServerConnectedToast: 'Serveur Discord connecté !',
    discordLinkedToast: 'Compte Discord lié !',
    discordUnlinkedToast: 'Compte Discord dissocié',
    discordUnlinkFailedToast: 'Impossible de dissocier votre compte Discord',
    discordDisconnectFailedToast: 'Échec de la déconnexion du serveur',
  },
  viewer: {
    heading: 'Identité de spectateur',
    subheading: "Personnalisez l'apparence de votre nom sur tous les overlays",
    signInHeading: 'Connectez-vous pour gérer votre identité de spectateur',
    signInBody:
      'Connectez votre compte de plateforme de streaming pour définir une couleur de nom personnalisée et gérer votre identité de spectateur.',
    signInTwitch: 'Se connecter avec Twitch',
    signInYoutube: 'Se connecter avec YouTube',
    signInKick: 'Se connecter avec Kick',
    profileHeading: 'Profil',
    viewerFallbackName: 'Spectateur',
    nameColorHeading: 'Couleur du nom',
    nameColorBody:
      'Définissez une couleur ou un dégradé personnalisé pour votre nom sur les overlays',
    solidTab: 'Couleur unie',
    gradientTab: 'Dégradé',
    premiumPill: 'Premium',
    gradientUpsell: 'Les noms en dégradé sont un cosmétique premium spectateur.',
    unlockPremium: 'Débloquer le premium spectateur',
    colorPickerLabel: 'Sélecteur de couleur du nom',
    colorHexLabel: 'Valeur hexadécimale de la couleur du nom',
    savedFeedback: 'Enregistré ✓',
    autoSaveNote: 'Les modifications sont enregistrées automatiquement',
    previewLabel: 'Aperçu',
    previewMessage: 'Bonjour tout le monde !',
    removeStopLabel: 'Supprimer le point {index}',
    addStop: '+ Ajouter un point',
    angleLabel: 'Angle',
    angleDegreesLabel: 'Angle en degrés',
    saveGradient: 'Enregistrer le dégradé',
    savingGradient: 'Enregistrement…',
    cosmeticsHeading: "Cosmétiques d'avatar",
    cosmeticsBody: 'Choisissez un cadre et un flair pour votre avatar',
    cosmeticsUpsell: 'Certains cadres et flairs sont réservés au premium spectateur.',
    frameHeading: "Cadre d'avatar",
    flairHeading: "Flair d'avatar",
    noneItem: 'Aucun',
    save: 'Enregistrer',
    saving: 'Enregistrement…',
    premiumRequired: 'Premium requis',
    saveFailed: "Échec de l'enregistrement",
    linkedHeading: 'Plateformes liées',
    linkedBody: "Connectez d'autres plateformes pour partager vos cosmétiques dans tous vos chats",
    connected: 'Connecté',
    connect: 'Connecter',
    connecting: 'Connexion…',
    disconnect: 'Déconnecter',
    disconnecting: 'Déconnexion…',
    loadLinkedFailed: 'Impossible de charger les plateformes liées',
    disconnectFailed: 'Échec de la déconnexion de la plateforme',
    deleteHeading: 'Supprimer les données de spectateur',
    deleteBody:
      "Effacez définitivement votre identité de spectateur : plateformes liées, couleur du nom et cosmétiques d'avatar, points, votes aux sondages et prédictions dans toutes les chaînes, les messages que vous avez envoyés via All-Chat et votre connexion Patreon de spectateur. Les votes et les points obtenus par commande de chat sans vous connecter sont inclus. Un compte streamer que vous possédez n'est pas concerné.",
    deleteButton: 'Supprimer mes données de spectateur',
    deleteConfirmTitle: 'Supprimer vos données de spectateur ?',
    deleteConfirmBody:
      "Tout ce qui est listé est effacé immédiatement et vous êtes déconnecté. Cette action est irréversible. Elle n'annule pas un abonnement Patreon ; annulez-le sur Patreon. Si vous continuez à écrire dans le chat de chaînes qui utilisent les sondages ou les points All-Chat, de nouvelles données seront collectées à partir de ce moment-là.",
    deleteCancel: 'Annuler',
    deleteConfirm: 'Oui, supprimer mes données de spectateur',
    deleting: 'Suppression…',
    deleteFailed: 'Impossible de supprimer vos données de spectateur. Veuillez réessayer.',
    deletedToast: 'Vos données de spectateur ont été supprimées',
  },
  apiTokens: {
    heading: 'Jetons API',
    subheading: "Jetons d'accès personnels pour les plugins Stream Deck et StreamController.",
    listHeading: 'Vos jetons',
    listBody:
      'Seules les informations ci-dessous sont stockées : le jeton lui-même est conservé sous forme de hachage et ne pourra plus jamais être affiché.',
    loadFailed: 'Impossible de charger vos jetons. Actualisez la page pour réessayer.',
    revealRegionLabel: 'Nouveau jeton {name}',
    revealHeading: 'Copiez votre nouveau jeton maintenant',
    revealWarning:
      "C'est la seule fois où {name} sera affiché. Nous n'en stockons qu'un hachage, il ne pourra donc plus être affiché : si vous le perdez, révoquez ce jeton et créez-en un nouveau.",
    copyToken: 'Copier le jeton',
    copied: 'Copié ✓',
    copyFailed: 'Copie automatique impossible : sélectionnez le jeton et copiez-le manuellement.',
    dismissReveal: "Je l'ai enregistré",
    createHeading: 'Créer un jeton',
    createBody:
      "Donnez au jeton un nom que vous reconnaîtrez plus tard, et ne lui accordez que ce dont l'appareil a besoin.",
    nameLabel: 'Nom du jeton',
    namePlaceholder: 'Stream Deck (PC du studio)',
    nameDescription: 'Affiché dans la liste ci-dessous pour que vous sachiez quoi révoquer.',
    scopesLegend: 'Autorisations',
    noScopesWarning:
      "Choisissez au moins une autorisation : un jeton qui n'en a aucune peut s'authentifier mais ne peut rien faire.",
    create: 'Créer le jeton',
    creating: 'Création…',
    createFailed: 'Impossible de créer le jeton. Réessayez.',
    scopeChatWriteTitle: 'Envoyer des messages dans le chat',
    scopeChatWriteDescription: 'Permet au plugin de publier des messages dans vos chats connectés.',
    scopeEngagementWriteTitle: 'Gérer les sondages et les prédictions',
    scopeEngagementWriteDescription:
      "Permet au plugin d'ouvrir, de résoudre et d'annuler des sondages et des prédictions.",
    emptyHeading: "Vous n'avez pas encore de jeton API",
    emptyBody:
      "Un jeton d'accès personnel permet à un appareil de se connecter en votre nom sans votre mot de passe : c'est ainsi que les plugins Stream Deck et StreamController envoient des messages dans le chat et gèrent les sondages et les prédictions pour vous. Créez-en un par appareil afin de pouvoir le révoquer séparément.",
    setupGuides: 'Guides de configuration :',
    streamDeckReadme: 'README du plugin Stream Deck',
    streamControllerReadme: 'README du plugin StreamController',
    tokenDates: 'Créé le {created} · Dernière utilisation : {lastUsed}',
    neverUsed: 'jamais',
    unknownDate: '—',
    revokeLabel: 'Révoquer {name}',
    revoke: 'Révoquer',
    revokeConfirmTitle: 'Révoquer ce jeton ?',
    revokeConfirmBody:
      "« {name} » cesse de fonctionner immédiatement. Tout appareil qui l'utilise aura besoin d'un nouveau jeton.",
    revokeCancel: 'Annuler',
    revokeConfirm: 'Révoquer le jeton',
    revoking: 'Révocation…',
    revokedToast: '{name} révoqué',
    revokeFailedToast: 'Impossible de révoquer ce jeton',
  },
  devices: {
    heading: 'Appareils associés',
    subheading:
      "Surfaces de contrôle Stream Deck et StreamController associées à votre compte. Chacune est verrouillée sur un seul overlay et expire d'elle-même si elle n'est plus utilisée.",
    listHeading: 'Vos appareils',
    listBody:
      "Seules les informations ci-dessous sont stockées. L'identifiant d'accès lui-même a été envoyé directement au plugin et est conservé ici sous forme de hachage : il n'est jamais affiché dans ce tableau de bord, c'est pourquoi il n'y a rien à copier sur cette page.",
    loadingLabel: 'Chargement des appareils associés',
    loadFailed: 'Impossible de charger vos appareils associés. Actualisez la page pour réessayer.',
    emptyHeading: 'Aucun appareil associé pour le moment',
    emptyBody:
      "L'association démarre dans le plugin, pas ici : ouvrez les paramètres de votre Stream Deck ou StreamController et appuyez sur {linkAction}. Votre navigateur ouvre un écran d'approbation, vous choisissez un overlay, et le plugin reçoit directement son identifiant d'accès : rien n'est copié ni collé.",
    emptyLinkAction: 'Associer à All-Chat',
    setupGuide: 'Guide de configuration',
    havePairingCode: "J'ai un code d'association",
    controlsOverlay: 'Contrôle {overlay} · {status}',
    rowDates: 'Dernière utilisation : {lastUsed} · Associé le {paired}',
    neverUsed: 'jamais',
    unknownDate: '—',
    revokeLabel: 'Révoquer {name}',
    revoke: 'Révoquer',
    statusRevoked: 'Révoqué le {date}',
    statusExpired: 'Expiré le {date}',
    statusActive: "Actif jusqu'au {date}",
    headlessHeading: 'Sur une seconde machine ou un boîtier sans écran ?',
    headlessBody:
      "L'association nécessite que le plugin et ce navigateur soient sur le même ordinateur. Quand ce n'est pas le cas (un Stream Deck qui pilote un PC de capture, un serveur sans interface graphique), utilisez plutôt un {tokenLink}, ou commencez par {pairingLink} si votre plugin en affiche un.",
    headlessTokenLink: "jeton d'accès personnel",
    headlessPairingLink: "un code d'association",
    revokeConfirmTitle: 'Révoquer cet appareil ?',
    revokeConfirmBody:
      "« {name} » cesse de fonctionner immédiatement. Associez-le à nouveau depuis le plugin si vous voulez continuer à l'utiliser.",
    revokeCancel: 'Annuler',
    revokeConfirm: "Révoquer l'appareil",
    revoking: 'Révocation…',
    revokedToast: '{name} révoqué',
    revokeFailedToast: 'Impossible de révoquer cet appareil',
  },
  premium: {
    back: '← Retour aux paramètres',
    heading: 'Premium',
    connectPitch:
      'Soutenez All-Chat sur Patreon pour débloquer automatiquement les fonctionnalités premium.',
    notAPatronSuffix: 'Pas encore membre sur Patreon ? {link}, puis connectez votre compte.',
    premiumRow: 'Premium',
    statusExpired: 'En dessous du palier premium',
    notGranting:
      'Votre Patreon est lié mais ne donne pas accès au premium. Vérifiez que votre contribution est active et au moins égale au palier premium.',
    disconnectBody:
      'Cette action dissocie votre compte Patreon. Le premium accordé par votre abonnement sera retiré.',
  },
  viewerPremium: {
    back: "← Retour à l'identité de spectateur",
    heading: 'Premium spectateur',
    subheading:
      'Un abonnement moins cher qui débloque les cosmétiques de spectateur (votre badge de chat premium et le dégradé de votre nom) sur tous les overlays où vous apparaissez.',
    connectPitch:
      'Soutenez All-Chat sur Patreon pour débloquer automatiquement vos cosmétiques premium de spectateur.',
    notAPatronSuffix:
      'Pas encore membre sur Patreon ? {link} (palier spectateur à partir de 2 €), puis connectez votre compte.',
    premiumRow: 'Premium spectateur',
    statusExpired: 'En dessous du palier spectateur',
    notGranting:
      'Votre Patreon est lié mais ne donne pas accès au premium spectateur. Vérifiez que votre contribution est active et au moins égale au palier spectateur.',
    disconnectBody:
      'Cette action dissocie votre compte Patreon. Le premium spectateur accordé par votre abonnement sera retiré.',
    signInHeading: 'Connectez-vous pour gérer le premium spectateur',
    signInBody:
      'Connectez-vous avec votre compte de plateforme de streaming, puis soutenez All-Chat sur Patreon pour débloquer votre badge de chat premium et vos cosmétiques.',
    signInLink: 'Aller à la connexion spectateur →',
  },
}
