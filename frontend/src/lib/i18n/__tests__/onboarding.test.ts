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

/**
 * Copy lock for the first-run setup guide. See __tests__/dashboard.test.ts for
 * why the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('setup guide chrome', () => {
  it('keeps the guide title and its progress line', () => {
    expect(t('onboarding.checklist.title')).toBe('Setup guide')
    expect(t('onboarding.checklist.allDone')).toBe('All steps done!')
    expect(t('onboarding.checklist.progress', { done: '2', total: '4' })).toBe('2 of 4 steps done')
  })

  it('keeps the minimize, expand and dismiss labels', () => {
    expect(t('onboarding.checklist.expand')).toBe('Expand setup guide')
    expect(t('onboarding.checklist.minimize')).toBe('Minimize setup guide')
    expect(t('onboarding.checklist.dismiss')).toBe('Dismiss setup guide')
  })

  it('marks a completed step for screen readers', () => {
    // Rendered inside VisuallyHidden after the step label, so the leading space
    // is part of the string.
    expect(t('onboarding.checklist.stepDone')).toBe(' (done)')
  })
})

describe('setup guide steps', () => {
  it('names the four core steps', () => {
    expect(t('onboarding.steps.createOverlay')).toBe('Create your overlay')
    expect(t('onboarding.steps.connectSource')).toBe('Connect a chat source')
    expect(t('onboarding.steps.chooseTheme')).toBe('Pick a theme')
    expect(t('onboarding.steps.copyObs')).toBe('Add it to OBS')
  })

  it('keeps every step call to action', () => {
    expect(t('onboarding.checklist.create')).toBe('Create')
    expect(t('onboarding.checklist.showMe')).toBe('Show me')
    expect(t('onboarding.checklist.openEditor')).toBe('Open editor')
    expect(t('onboarding.checklist.copyLink')).toBe('Copy link')
    expect(t('onboarding.checklist.copied')).toBe('Copied!')
  })
})

describe('OBS instructions', () => {
  it('keeps the three add-to-OBS steps whole', () => {
    expect(t('onboarding.obs.heading')).toBe('In OBS:')
    // The two emphasised runs are the OBS UI affordances the streamer must
    // click, so the sentence stays whole and interpolateElements wraps them.
    expect(t('onboarding.obs.step1')).toBe(
      'In OBS, click {plus} under Sources and choose {browser}.'
    )
    expect(t('onboarding.obs.step1Plus')).toBe('+')
    expect(t('onboarding.obs.step1Browser')).toBe('Browser')
    expect(t('onboarding.obs.step2')).toBe('Paste the copied overlay link into the URL field.')
    expect(t('onboarding.obs.step3')).toBe(
      'Size the source to the area chat should fill (a tall, narrow box like 450×800 works well, not your full canvas), then drag it into place. Chat appears as soon as the overlay connects.'
    )
  })
})

describe('optional extras', () => {
  it('keeps the section heading and its footers', () => {
    expect(t('onboarding.extras.heading')).toBe('Optional: go further')
    expect(t('onboarding.extras.seePremium')).toBe('See everything Premium includes')
    expect(t('onboarding.extras.done')).toBe('Done')
    expect(t('onboarding.extras.skip')).toBe('Skip')
  })

  it('keeps the text-to-speech entry', () => {
    expect(t('onboarding.extras.ttsTitle')).toBe('Text-to-speech')
    expect(t('onboarding.extras.ttsBody')).toBe(
      'Read chat aloud. Browser voices are free; ElevenLabs voices are Premium.'
    )
  })

  it('keeps the overlay moderation entry', () => {
    expect(t('onboarding.extras.moderationTitle')).toBe('Moderate from your overlay')
    expect(t('onboarding.extras.moderationBody')).toBe(
      'Delete, timeout, ban and unban from the Monitor View button at the top of the editor. Full controls on Twitch, Kick and Discord; timeout and ban on YouTube. (Premium)'
    )
  })

  it('keeps the delegated moderators entry', () => {
    expect(t('onboarding.extras.moderatorsTitle')).toBe('Let your mods help')
    expect(t('onboarding.extras.moderatorsBody')).toBe(
      'Hand the Monitor View to your existing moderators under Moderators. They act with their own platform accounts, and they never need Premium themselves. (Premium)'
    )
  })

  it('keeps the shared chat entry', () => {
    expect(t('onboarding.extras.sharedChatTitle')).toBe('Shared chat')
    expect(t('onboarding.extras.sharedChatBody')).toBe(
      'Combine several channels into one conversation via the Share Overlay button. (Premium)'
    )
  })

  it('keeps the YouTube stream selection entry', () => {
    expect(t('onboarding.extras.streamSelectionTitle')).toBe('YouTube stream selection')
    expect(t('onboarding.extras.streamSelectionBody')).toBe(
      'Pick exactly which broadcast an overlay listens to, per YouTube source in Sources. (Premium)'
    )
  })

  it('keeps the bubble colours entry', () => {
    expect(t('onboarding.extras.bubbleColorsTitle')).toBe('Differently-coloured bubbles')
    expect(t('onboarding.extras.bubbleColorsBody')).toBe(
      'Give each platform its own bubble colour, or cycle a palette down the feed, under Bubble colors in Appearance. Free.'
    )
  })

  it('keeps the viewer flairs entry', () => {
    expect(t('onboarding.extras.flairsTitle')).toBe('Viewer flairs')
    expect(t('onboarding.extras.flairsBody')).toBe(
      'Premium cosmetics for chatters, like animated name gradients, under Flairs in the navigation.'
    )
  })

  it('keeps the Stream Deck entry', () => {
    expect(t('onboarding.extras.streamDeckTitle')).toBe('Stream Deck buttons')
    expect(t('onboarding.extras.streamDeckBody')).toBe(
      'Drive polls, predictions and canned messages from a physical button. Link a Stream Deck or StreamController under Paired devices — nothing to copy or paste. Starting a poll or prediction is still Premium.'
    )
  })
})

describe('finale and dismissal', () => {
  it('keeps the finale copy', () => {
    expect(t('onboarding.finale.title')).toBe("You're live! 🎉")
    expect(t('onboarding.finale.body')).toBe(
      'Questions, feedback, or theme requests? Our community is happy to help.'
    )
    expect(t('onboarding.finale.joinDiscord')).toBe('Join the Discord')
    expect(t('onboarding.finale.finish')).toBe('Finish')
  })

  it('keeps the dismiss confirmation', () => {
    expect(t('onboarding.dismissDialog.title')).toBe('Hide the setup guide?')
    expect(t('onboarding.dismissDialog.body')).toBe(
      'You can restart it anytime from Settings → Setup guide.'
    )
    expect(t('onboarding.dismissDialog.keep')).toBe('Keep it')
    expect(t('onboarding.dismissDialog.hide')).toBe('Hide guide')
  })
})

describe('create overlay dialog', () => {
  it('keeps the dialog copy', () => {
    expect(t('onboarding.createDialog.title')).toBe('Create your overlay')
    expect(t('onboarding.createDialog.body')).toBe(
      "Give it a name — you'll connect your chat right after."
    )
    expect(t('onboarding.createDialog.nameLabel')).toBe('Overlay name')
    expect(t('onboarding.createDialog.cancel')).toBe('Cancel')
    expect(t('onboarding.createDialog.submit')).toBe('Create overlay')
    expect(t('onboarding.createDialog.submitting')).toBe('Creating…')
  })
})

describe('create overlay dialog toast copy', () => {
  it('keeps the failure title', () => {
    expect(t('onboarding.createDialog.failedToast')).toBe('Could not create the overlay')
  })
})
