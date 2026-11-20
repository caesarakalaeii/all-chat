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
  heading: 'Commandes de chat',
  intro:
    "Répondez au chat automatiquement, créez des clips sur demande et affichez des images à l'écran. Une commande se déclenche quand un message commence par son déclencheur, un mot-clé quand le déclencheur apparaît n'importe où dans un message. Les réponses partent dans le chat d'où vient le message, envoyées par l'All-Chat Bot ou, si vous le souhaitez, par votre propre compte.",
  newCommand: 'Nouvelle commande',
  loading: 'Chargement des commandes de chat',
  loadFailed: 'Impossible de charger vos commandes de chat.',
  retry: 'Réessayer',
  empty: "Aucune commande pour le moment. Partez d'un exemple ou créez la vôtre.",
  examplesHeading: "Partir d'un exemple",
  examples: {
    hello: 'Salutation',
    clip: 'Clip sur demande',
    dice: 'Lancer de dé',
    shoutout: 'Shoutout (modos)',
    dog: 'Chien aléatoire',
  },
  list: {
    kindCommand: 'Commande',
    kindKeyword: 'Mot-clé',
    clips: 'Clips',
    alert: 'Alerte',
    cooldown: 'Délai de {seconds} s',
    uses: '{count} utilisations',
    noReply: 'Aucune réponse dans le chat',
    edit: 'Modifier',
    editLabel: 'Modifier {trigger}',
    delete: 'Supprimer',
    deleteLabel: 'Supprimer {trigger}',
    enabledLabel: 'Activer {trigger}',
    toggleFailed: 'Impossible de mettre à jour la commande.',
  },
  roles: {
    everyone: 'Tout le monde',
    subscriber: 'Abonnés et plus',
    vip: 'VIP et plus',
    moderator: 'Modérateurs et vous',
    broadcaster: 'Vous uniquement',
  },
  sendFailures: {
    heading: 'Messages du bot non arrivés',
    item: 'Dernier message dans le chat {platform}, {time} :',
    dropped: "{platform} l'a refusé.",
    platformSays: '{platform} indique : « {detail} »',
    reauthBot:
      "L'All-Chat Bot y a perdu son autorisation. Prévenez-nous sur Discord pour que nous le reconnections, ou choisissez votre propre compte comme expéditeur dans Bot et expéditeur.",
    reauthCustom:
      'Votre compte de bot a perdu son autorisation. Reconnectez-le dans Bot et expéditeur.',
    reauthSelf: 'Votre compte a perdu son autorisation. Déconnectez-vous puis reconnectez-vous.',
    missingScope: 'Une autorisation manque. Accordez-la ci-dessus.',
    notConnected: "Aucun compte pouvant y publier n'est connecté.",
    offline: "Votre stream n'était pas en live.",
    quota: 'Le quota YouTube du jour est épuisé.',
    failed: "Il n'a pas pu être envoyé. Réessayez plus tard.",
    modHint:
      "Nommez le bot modérateur ou VIP sur votre chaîne : les restrictions du chat comme le mode réservé aux followers, le mode lent ou l'obligation d'un numéro de téléphone vérifié ne s'appliquent pas à eux.",
    clears: "Cet avis disparaît dès qu'un message passe.",
  },
  status: {
    heading: 'Où les commandes peuvent répondre',
    twitchNoLogin:
      'Les réponses et les clips Twitch nécessitent que vous vous connectiez avec Twitch.',
    twitchReplies: 'Réponses dans le chat Twitch',
    twitchClips: 'Clips Twitch',
    allowed: 'Autorisé',
    missing: 'Autorisation requise',
    grant: 'Autoriser sur Twitch',
    granting: 'Ouverture de Twitch',
    grantFailed: "Impossible de lancer la demande d'autorisation Twitch.",
    grantNeedsOverlay:
      "Ajoutez d'abord votre chaîne Twitch à l'un de vos overlays, puis autorisez-la ici.",
    youtubePremium: 'Les réponses dans le chat YouTube sont activées : vous avez premium.',
    youtubeLocked:
      "Les réponses dans le chat YouTube sont premium, car chacune consomme du quota de l'API YouTube. Les commandes créent tout de même des clips depuis le chat YouTube.",
    upgrade: 'Voir premium',
    kick: "Les réponses dans le chat Kick utilisent l'autorisation de chat des contrôles avancés d'un overlay.",
    others:
      'TikTok, Discord et les plateformes en bêta peuvent déclencher des clips, mais les commandes ne peuvent pas y répondre.',
  },
  editor: {
    createTitle: 'Nouvelle commande de chat',
    editTitle: 'Modifier {trigger}',
    kindLabel: 'Se déclenche quand',
    kindCommand: 'Un message commence par lui',
    kindKeyword: 'Un message le contient',
    triggerLabel: 'Déclencheur',
    triggerHintCommand:
      "Un seul mot, par exemple !bonjour. Les majuscules et minuscules n'ont pas d'importance.",
    triggerHintKeyword:
      "Un mot ou une expression, par exemple clip ça. Reconnu n'importe où dans un message.",
    responseLabel: 'Réponse',
    responseHint:
      "Laissez vide pour seulement créer un clip ou afficher une alerte. Cliquez sur une variable pour l'insérer.",
    insertVariable: 'Insérer {token}',
    alertHeading: "Alerte à l'écran",
    alertHint:
      "Affichée sur vos overlays d'alertes chaque fois que la commande se déclenche, depuis n'importe quel chat. Remplissez ce dont vous avez besoin. Le style Commande de chat de l'éditeur d'alertes définit la durée, le volume et le texte lorsque vous laissez le vôtre vide.",
    imageLabel: "Image à l'écran",
    imageHint:
      "Un lien https vers une image ou un GIF. Les modèles fonctionnent ici, mais ce que les spectateurs saisissent n'atteint jamais l'image.",
    soundLabel: "Son de l'alerte",
    alertTextLabel: "Texte de l'alerte",
    alertTextHint:
      "Remplace le texte par défaut de l'alerte à l'écran. Utilise les mêmes variables que la réponse.",
    imagePreviewAlt: "Aperçu de l'image",
    clipLabel: 'Créer un clip Twitch',
    clipHint:
      'Crée un clip de votre stream Twitch en live. Placez {token} dans la réponse pour publier le lien.',
    roleLabel: "Qui peut l'utiliser",
    cooldownLabel: 'Délai de récupération (secondes)',
    cooldownHint: "La commande ignore le chat pendant cette durée après s'être déclenchée.",
    enabledLabel: 'Activée',
    previewHeading: 'Aperçu',
    previewInputLabel: 'Un spectateur saisit après le déclencheur',
    previewInputPlaceholder: '@ami quelques mots',
    previewEmpty: 'La réponse apparaît ici.',
    previewFailed: 'Aperçu indisponible.',
    cancel: 'Annuler',
    save: 'Enregistrer',
    saving: 'Enregistrement',
    saveFailed: "Impossible d'enregistrer la commande.",
    saved: 'Commande enregistrée',
    referenceHeading: 'Référence des modèles',
    jsHeading: 'JavaScript',
    jsBody:
      "Placez du code dans un bloc js pour calculer une réponse. La valeur de la dernière expression est insérée. Les variables user, touser, query, args, count, platform, channel, role et clip sont disponibles. Le code lui-même n'a accès ni au réseau ni aux fichiers, et chaque réponse dispose de 100 ms et de 16 Mo.",
    jsExample: 'Exemple',
    urlfetchHeading: 'Interroger une adresse web',
    urlfetchBody:
      "{token} insère le texte renvoyé par une adresse web. Dans un bloc js, il arrive sous forme de chaîne, prêt pour JSON.parse. Seules les adresses https publiques fonctionnent, 3 au maximum par réponse et 3 secondes chacune. Les variables dans l'adresse sont échappées, les spectateurs ne peuvent donc pas modifier sa destination.",
  },
  reference: {
    user: 'Nom du chatteur qui a utilisé la commande',
    touser: "Premier mot après le déclencheur, sans @, ou le chatteur s'il n'y en a pas",
    query: 'Tout ce qui suit le déclencheur ; pour un mot-clé, le message entier',
    args: 'Les mots un par un après le déclencheur',
    count: "Nombre d'exécutions de la commande",
    random: 'Un nombre aléatoire de 1 à 100, ou dans votre plage',
    pick: 'Une des options, au hasard',
    time: "L'heure (24 h) dans un fuseau horaire ; UTC s'il est omis",
    where: "La plateforme et la chaîne d'où vient le message",
    clip: 'Lien vers le clip créé par cette commande',
    urlfetch: 'Le texte renvoyé par une adresse web',
  },
  delete: {
    title: 'Supprimer {trigger} ?',
    body: "La commande cesse de répondre immédiatement. Son compteur d'utilisations est perdu.",
    cancel: 'Conserver',
    confirm: 'Supprimer',
    failed: 'Impossible de supprimer la commande.',
  },
  bot: {
    heading: 'Bot et expéditeur',
    senderBot: "Les réponses viennent de l'All-Chat Bot",
    senderBotHint: 'Fonctionne sur Twitch, Kick et YouTube sans compte séparé.',
    senderSelf: 'Les réponses viennent de votre propre compte',
    senderSelfHint:
      "Le message apparaît comme si vous l'aviez saisi. Nécessite l'autorisation d'envoi dans le chat.",
    senderToggleLabel: 'Changer qui envoie les réponses : All-Chat Bot ou votre compte',
    senderSaveFailed: "Impossible de changer l'expéditeur des réponses.",
    twitchGranted: "L'All-Chat Bot est autorisé dans votre chat Twitch.",
    twitchNotGranted:
      "L'All-Chat Bot a besoin de votre autorisation pour publier dans votre chat Twitch.",
    grant: 'Autoriser dans le chat Twitch',
    granting: 'Ouverture de Twitch',
    grantFailed: "Impossible de lancer la demande d'autorisation Twitch.",
    connectedToast: 'Bot connecté.',
    connectFailedToast: 'La connexion du bot a échoué.',
    customHeading: 'Comptes de bot personnalisés',
    customIntro:
      'Connectez votre propre compte de bot par plateforme. Les réponses partent alors sous ce nom.',
    customPremium:
      'Les comptes de bot personnalisés sont une fonctionnalité premium. Consultez premium pour connecter votre propre bot sur Twitch, Kick ou YouTube.',
    noneConnected: 'Aucun bot personnalisé connecté',
    connectedAs: '{name} ({login})',
    connect: 'Connecter un bot {platform}',
    connecting: 'Connexion',
    connectFailed: 'Impossible de lancer la connexion du bot.',
    disconnect: 'Déconnecter',
    disconnecting: 'Déconnexion',
    disconnectLabel: 'Déconnecter le bot {platform}',
    disconnected: 'Bot déconnecté.',
    disconnectFailed: 'La déconnexion du bot a échoué.',
    removeTitle: 'Déconnecter le bot {platform} ?',
    removeBody:
      "Les réponses repassent par l'All-Chat Bot ou votre propre compte sur cette plateforme.",
    removeCancel: 'Conserver',
    removeConfirm: 'Déconnecter',
  },
  timers: {
    heading: 'Minuteurs',
    intro:
      "Les minuteurs publient un message dans vos chats Twitch et Kick, et dans le chat YouTube avec premium. Donnez-leur un intervalle en minutes, un nombre minimum de messages dans le chat, ou les deux : avec les deux, le minuteur se déclenche toutes les X minutes, mais seulement si au moins Y messages sont arrivés depuis la dernière publication. Un minuteur qui n'a qu'un intervalle publie aussi quand vous êtes hors ligne.",
    new: 'Nouveau minuteur',
    empty: 'Aucun minuteur pour le moment.',
    edit: 'Modifier',
    delete: 'Supprimer',
    deleteLabel: 'Supprimer {name}',
    enabledLabel: 'Activer {name}',
    toggleFailed: 'Impossible de mettre à jour le minuteur.',
    deleteFailed: 'Impossible de supprimer le minuteur.',
    saved: 'Minuteur enregistré',
    createTitle: 'Nouveau minuteur',
    editTitle: 'Modifier {name}',
    dialogIntro:
      "Un minuteur publie son message dans votre chat selon un horaire ou selon l'activité du chat.",
    nameLabel: 'Nom',
    nameRequired: 'Donnez un nom au minuteur.',
    messageLabel: 'Message',
    messageHint:
      "Ce qui est publié. Les variables de modèle qui n'ont pas besoin d'un chatteur fonctionnent, comme count, random, pick, time et js.",
    messageRequired: 'Rédigez le message à publier.',
    intervalLabel: 'Intervalle (minutes, facultatif)',
    intervalHint: "Entre {min} et {max} minutes. Laissez vide pour ne pas définir d'intervalle.",
    messagesLabel: 'Minimum de messages (facultatif)',
    messagesHint: 'Entre {min} et {max} messages. Laissez vide pour ne pas définir de minimum.',
    combinedRule:
      "Définissez au moins l'un des deux. Avec les deux, le minuteur attend l'intervalle et le nombre minimum de messages depuis sa dernière publication.",
    triggerRequired: 'Définissez un intervalle, un nombre minimum de messages, ou les deux.',
    intervalRange: "L'intervalle doit être un nombre entier compris entre {min} et {max} minutes.",
    messagesRange: 'Le minimum doit être un nombre entier compris entre {min} et {max} messages.',
    triggerBoth: 'toutes les {minutes} minutes, au moins {messages} messages depuis le dernier',
    triggerEvery: 'toutes les {minutes} minutes',
    triggerMessages: 'après {messages} nouveaux messages',
    cancel: 'Annuler',
    save: 'Enregistrer',
    saving: 'Enregistrement',
    saveFailed: "Impossible d'enregistrer le minuteur.",
    deleteTitle: 'Supprimer {name} ?',
    deleteBody: 'Le minuteur cesse de publier immédiatement.',
    deleteCancel: 'Conserver',
    deleteConfirm: 'Supprimer',
  },
}
