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
 * Copy lock for the overlay editor surface. See __tests__/dashboard.test.ts for
 * why the copy is pinned here rather than through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('theme marketplace copy', () => {
  it('keeps the two marketplace titles as whole sentences', () => {
    // The render site built these as `{cond ? 'Credit Roll' : ''} Theme
    // Marketplace`, which is the concatenation the catalog exists to remove: a
    // language that puts the qualifier after the noun cannot reassemble it.
    expect(t('overlayEditor.themeMarketplace.title')).toBe('Theme Marketplace')
    expect(t('overlayEditor.themeMarketplace.titleCreditRoll')).toBe(
      'Credit Roll Theme Marketplace'
    )
    expect(t('overlayEditor.themeMarketplace.description')).toBe(
      'Browse and apply custom CSS themes for your overlay'
    )
    expect(t('overlayEditor.themeMarketplace.descriptionCreditRoll')).toBe(
      'Browse and apply custom CSS themes for your credit roll'
    )
  })

  it('keeps the marketplace states', () => {
    expect(t('overlayEditor.themeMarketplace.loading')).toBe('Loading themes')
    expect(t('overlayEditor.themeMarketplace.loadingEllipsis')).toBe('Loading themes...')
    expect(t('overlayEditor.themeMarketplace.errorTitle')).toBe('Error Loading Themes')
    expect(t('overlayEditor.themeMarketplace.emptyTitle')).toBe('No themes found')
    expect(t('overlayEditor.themeMarketplace.emptyBody')).toBe('Try adjusting your filters')
  })

  it('keeps the result count as one sentence with both numbers in it', () => {
    // Was `Showing {filtered} of {total} themes` split across three text nodes.
    expect(t('overlayEditor.themeMarketplace.showingCount', { shown: 4, total: 12 })).toBe(
      'Showing 4 of 12 themes'
    )
  })

  it('keeps the marketplace controls', () => {
    expect(t('overlayEditor.themeMarketplace.applyTheme')).toBe('Apply Theme')
    expect(t('overlayEditor.themeMarketplace.clearFilters')).toBe('Clear Filters')
    expect(t('overlayEditor.themeMarketplace.searchLabel')).toBe('Search themes')
    expect(t('overlayEditor.themeMarketplace.searchPlaceholder')).toBe('Search themes...')
    expect(t('overlayEditor.themeMarketplace.sync')).toBe('Sync')
    expect(t('overlayEditor.themeMarketplace.syncTitleInline')).toBe(
      'Force refresh themes from GitHub (Admin)'
    )
    expect(t('overlayEditor.themeMarketplace.syncLabel')).toBe('Force refresh themes from GitHub')
    expect(t('overlayEditor.themeMarketplace.syncTitle')).toBe('Force refresh themes (Admin)')
    expect(t('overlayEditor.themeMarketplace.closeLabel')).toBe('Close theme marketplace')
  })
})

describe('credit roll preview copy', () => {
  it('keeps the sample credits the preview renders', () => {
    expect(t('overlayEditor.creditRollPreview.heading')).toBe('🎬 Stream Credits')
    expect(t('overlayEditor.creditRollPreview.subheading')).toBe('Thank you for your support!')
    expect(t('overlayEditor.creditRollPreview.leaderboardHeading')).toBe('Top Subscribers')
    expect(t('overlayEditor.creditRollPreview.footerHeading')).toBe('Thank you! ❤️')
    expect(t('overlayEditor.creditRollPreview.footerBody')).toBe('See you next stream!')
  })
})

describe('editor chrome copy', () => {
  it('keeps the advanced disclosure count inside one key', () => {
    // Was `Advanced ({count})` — a text node, an expression and a closing
    // parenthesis. The parentheses are punctuation a language may drop.
    expect(t('overlayEditor.nav.advancedCount', { count: 4 })).toBe('Advanced (4)')
  })

  it('keeps the editor navigation and preview labels', () => {
    expect(t('overlayEditor.nav.settingsLabel')).toBe('Overlay settings')
    expect(t('overlayEditor.previewBackdrop.heading')).toBe('Backdrop')
    expect(t('overlayEditor.previewBackdrop.appBackground')).toBe('Preview on app background')
    expect(t('overlayEditor.previewBackdrop.lightBackground')).toBe('Preview on light background')
    expect(t('overlayEditor.previewBackdrop.chromaGreen')).toBe('Preview on chroma green')
    expect(t('overlayEditor.previewBackdrop.customColor')).toBe('Custom preview background color')
  })

  it('keeps the settings search copy, typographic quotes included', () => {
    expect(t('overlayEditor.settingsSearch.label')).toBe('Search settings')
    expect(t('overlayEditor.settingsSearch.placeholder')).toBe(
      'Search settings\u2026 (e.g. badge, fade, banned words)'
    )
    expect(t('overlayEditor.settingsSearch.clearLabel')).toBe('Clear search')
    expect(t('overlayEditor.settingsSearch.resultsLabel')).toBe('Matching settings')
    expect(t('overlayEditor.settingsSearch.noResults', { query: 'badge' })).toBe(
      'No settings match \u201cbadge\u201d'
    )
  })
})

describe('appearance control copy', () => {
  it('keeps the background group labels', () => {
    expect(t('overlayEditor.background.overlayHeading')).toBe('Overlay background')
    expect(t('overlayEditor.background.overlayColor')).toBe('Overlay background')
    expect(t('overlayEditor.background.bubbleHeading')).toBe('Bubble background')
    expect(t('overlayEditor.background.bubbleColor')).toBe('Bubble background')
    expect(t('overlayEditor.background.borderColor')).toBe('Border color')
    expect(t('overlayEditor.background.borderRadius')).toBe('Border radius')
    expect(t('overlayEditor.background.borderWidth')).toBe('Border width')
    expect(t('overlayEditor.background.padding')).toBe('Padding')
    expect(t('overlayEditor.background.messageGap')).toBe('Message gap')
    expect(t('overlayEditor.background.backdropBlur')).toBe('Backdrop blur')
  })

  it('keeps the colors group labels', () => {
    expect(t('overlayEditor.colors.message')).toBe('Message color')
    expect(t('overlayEditor.colors.username')).toBe('Username color')
    expect(t('overlayEditor.colors.timestamp')).toBe('Timestamp color')
  })

  it('keeps the sizing group labels and the emote-scale caveat', () => {
    expect(t('overlayEditor.sizing.avatarSize')).toBe('Avatar size')
    expect(t('overlayEditor.sizing.badgeSize')).toBe('Badge size')
    expect(t('overlayEditor.sizing.emoteScale')).toBe('Emote scale')
    expect(t('overlayEditor.sizing.emoteScaleNote')).toBe(
      'Emote scale applies to third-party emotes (7TV, BTTV, FFZ). Standard emoji are not affected.'
    )
  })

  it('keeps the per-event size modifier label', () => {
    expect(t('overlayEditor.events.sizeModifier')).toBe('Size modifier')
  })

  it('names the colour picker controls after the swatch they belong to', () => {
    // All three took the control's own `label` prop, so the placeholder carries
    // it through rather than the catalog holding one key per swatch.
    expect(t('overlayEditor.colorPicker.swatchLabel', { label: 'Border color' })).toBe(
      'Pick color for Border color'
    )
    expect(t('overlayEditor.colorPicker.popoverTitle', { label: 'Border color' })).toBe(
      'Color for Border color'
    )
    expect(t('overlayEditor.colorPicker.hexLabel', { label: 'Border color' })).toBe(
      'Hex value for Border color'
    )
  })

  it('keeps the font picker labels and group headings', () => {
    expect(t('overlayEditor.fontPicker.openLabel')).toBe('Open font picker')
    expect(t('overlayEditor.fontPicker.empty')).toBe('No fonts found')
    expect(t('overlayEditor.fontPicker.systemGroup')).toBe('System Fonts')
    expect(t('overlayEditor.fontPicker.googleGroup')).toBe('Google Fonts')
  })
})

describe('typography and visibility group copy', () => {
  it('keeps the font family field labels', () => {
    // Label and aria-label were byte-identical at every one of the three
    // sites, so they share one key each rather than doubling up.
    expect(t('overlayEditor.typography.bodyFont')).toBe('Body Font')
    expect(t('overlayEditor.typography.usernameFont')).toBe('Username Font')
    expect(t('overlayEditor.typography.timestampFont')).toBe('Timestamp Font')
  })

  it('keeps the font picker placeholder and default accessible name', () => {
    expect(t('overlayEditor.fontPicker.placeholder')).toBe('Select font…')
    expect(t('overlayEditor.fontPicker.defaultLabel')).toBe('Font family')
  })

  it('keeps the font weight options in numeric order', () => {
    expect(t('overlayEditor.typography.fontWeight')).toBe('Font Weight')
    expect(t('overlayEditor.typography.fontWeightPlaceholder')).toBe('Select weight…')
    expect(t('overlayEditor.typography.fontWeight100')).toBe('100 Thin')
    expect(t('overlayEditor.typography.fontWeight300')).toBe('300 Light')
    expect(t('overlayEditor.typography.fontWeight400')).toBe('400 Regular')
    expect(t('overlayEditor.typography.fontWeight500')).toBe('500 Medium')
    expect(t('overlayEditor.typography.fontWeight600')).toBe('600 SemiBold')
    expect(t('overlayEditor.typography.fontWeight700')).toBe('700 Bold')
    expect(t('overlayEditor.typography.fontWeight800')).toBe('800 ExtraBold')
    expect(t('overlayEditor.typography.fontWeight900')).toBe('900 Black')
  })

  it('keeps the font size fields and their pixel unit', () => {
    expect(t('overlayEditor.typography.bodySize')).toBe('Body Size')
    expect(t('overlayEditor.typography.usernameSize')).toBe('Username Size')
    expect(t('overlayEditor.typography.timestampSize')).toBe('Timestamp Size')
    // Rendered as the accessible description beside each number input, so it is
    // read out and therefore copy, not a CSS unit token.
    expect(t('overlayEditor.typography.pixelUnit')).toBe('px')
  })

  it('keeps the text shadow presets', () => {
    expect(t('overlayEditor.typography.textShadow')).toBe('Text Shadow')
    expect(t('overlayEditor.typography.textShadowNone')).toBe('None (default)')
    expect(t('overlayEditor.typography.textShadowSoft')).toBe('Soft shadow')
    expect(t('overlayEditor.typography.textShadowStrong')).toBe('Strong shadow')
    expect(t('overlayEditor.typography.textShadowOutline')).toBe('Outline')
    expect(t('overlayEditor.typography.textShadowCustom')).toBe('Custom')
    expect(t('overlayEditor.typography.textShadowNote')).toBe(
      'Keeps chat readable over bright gameplay. Try it with a light preview backdrop.'
    )
  })

  it('keeps the advanced typography sliders', () => {
    expect(t('overlayEditor.typography.lineHeight')).toBe('Line Height')
    expect(t('overlayEditor.typography.letterSpacing')).toBe('Letter Spacing')
  })

  it('keeps the visibility toggles', () => {
    expect(t('overlayEditor.visibility.showAvatars')).toBe('Show avatars')
    expect(t('overlayEditor.visibility.showBadges')).toBe('Show badges')
    expect(t('overlayEditor.visibility.showTimestamps')).toBe('Show timestamps')
    expect(t('overlayEditor.visibility.showEmotes')).toBe('Show emotes')
    expect(t('overlayEditor.visibility.showUsername')).toBe('Show username')
    expect(t('overlayEditor.visibility.showPlatformBadge')).toBe('Show platform badge')
    expect(t('overlayEditor.visibility.showPlatformIndicators')).toBe('Show platform indicators')
    expect(t('overlayEditor.visibility.showPronouns')).toBe('Show pronouns')
  })

  it('keeps the badge and pronoun placement options', () => {
    expect(t('overlayEditor.visibility.position')).toBe('Position')
    expect(t('overlayEditor.visibility.style')).toBe('Style')
    expect(t('overlayEditor.visibility.beforeUsername')).toBe('Before username')
    expect(t('overlayEditor.visibility.afterUsername')).toBe('After username')
    expect(t('overlayEditor.visibility.styleText')).toBe('Text')
    expect(t('overlayEditor.visibility.styleIcon')).toBe('Icon')
    expect(t('overlayEditor.visibility.pronounPillColor')).toBe('Pill color')
  })

  it('keeps the event visibility rows', () => {
    expect(t('overlayEditor.events.showSuperChat')).toBe('Super Chat')
    expect(t('overlayEditor.events.showSubscriptions')).toBe('Subscriptions')
    expect(t('overlayEditor.events.showRaids')).toBe('Raids')
    expect(t('overlayEditor.events.showBits')).toBe('Bits')
    expect(t('overlayEditor.events.showMembershipGift')).toBe('Membership Gift')
  })
})

describe('filter group copy', () => {
  it('keeps the blocked-user and blocked-keyword fields', () => {
    expect(t('overlayEditor.filters.blockedUsernames')).toBe('Blocked usernames')
    expect(t('overlayEditor.filters.blockedUsernamesPlaceholder')).toBe(
      'Type username, press Enter'
    )
    expect(t('overlayEditor.filters.addCommonBots')).toBe('Add common bots')
    expect(t('overlayEditor.filters.blockedKeywords')).toBe('Blocked keywords')
    expect(t('overlayEditor.filters.blockedKeywordsPlaceholder')).toBe(
      'Type keyword or regex, press Enter'
    )
  })

  it('names the tag remove button after the tag it removes', () => {
    expect(t('overlayEditor.filters.removeTag', { tag: 'nightbot' })).toBe('Remove nightbot')
  })

  it('keeps the say-hi filter copy', () => {
    // sectionRegistry.ts duplicates this label so the editor search can index
    // it, and sectionRegistry.test.ts fails if the two disagree.
    expect(t('overlayEditor.filters.hideSayHi')).toBe('Hide YouTube "said hi" greetings')
    expect(t('overlayEditor.filters.hideSayHiNote')).toBe(
      'Only YouTube messages whose entire text is the greeting posted by the vertical-stream \u201cSay hi!\u201d button. Hidden messages also make no sound and are not read by TTS.'
    )
    expect(t('overlayEditor.filters.sayHiPhrases')).toBe('Extra \u201csaid hi\u201d phrases')
    expect(t('overlayEditor.filters.sayHiPhrasesPlaceholder')).toBe('Type phrase, press Enter')
    expect(t('overlayEditor.filters.sayHiPhrasesNote')).toBe(
      'The button\u2019s text is localised, so add what it posts in your language \u2014 for example the German phrase.'
    )
  })

  it('keeps the remaining filter controls', () => {
    expect(t('overlayEditor.filters.hideCommands')).toBe('Hide bot commands (!)')
    expect(t('overlayEditor.filters.minMessageLength')).toBe('Min message length')
    // Suffixed onto the slider's number by SliderControl, leading space and all.
    expect(t('overlayEditor.filters.charsUnit')).toBe(' chars')
  })
})

describe('bubble colours group copy', () => {
  it('keeps the locked-state upsell as one sentence around the link', () => {
    expect(t('overlayEditor.bubbleColors.lockedNotice')).toBe(
      'Different bubble colours per platform, or a palette cycled down the feed, are a {emphasis} feature.'
    )
    expect(t('overlayEditor.bubbleColors.lockedNoticeEmphasis')).toBe('Premium')
  })

  it('keeps the per-platform section copy', () => {
    expect(t('overlayEditor.bubbleColors.perPlatformHeading')).toBe('Per platform')
    expect(t('overlayEditor.bubbleColors.perPlatformBody')).toBe(
      'Tell sources apart at a glance. Unset platforms keep the normal bubble background.'
    )
    expect(t('overlayEditor.bubbleColors.resetPlatform', { platform: 'Twitch' })).toBe(
      'Reset Twitch bubble colour'
    )
  })

  it('keeps the palette section copy', () => {
    expect(t('overlayEditor.bubbleColors.paletteHeading')).toBe('Palette')
    expect(t('overlayEditor.bubbleColors.paletteBody')).toBe(
      'Two or more colours, cycled down the feed. A row keeps its colour while it is on screen. Needs at least two to take effect.'
    )
    expect(t('overlayEditor.bubbleColors.swatchLabel', { index: 2 })).toBe('Colour 2')
    expect(t('overlayEditor.bubbleColors.removeSwatch', { index: 2 })).toBe('Remove colour 2')
    expect(t('overlayEditor.bubbleColors.addSwatch')).toBe('Add colour')
    expect(t('overlayEditor.bubbleColors.singleSwatchNote')).toBe(
      'One colour behaves the same as Bubble background. Add a second to start cycling.'
    )
  })
})

describe('sound group copy', () => {
  it('keeps the OBS-versus-monitor explanation whole', () => {
    expect(t('overlayEditor.sounds.scopeNote')).toBe(
      'These sounds play on your public OBS overlay, for everyone watching your stream. Want a private alert only you hear when new activity arrives (channel-point redeems, a TikTok Rose, and so on)? Open the Monitor view and turn on Activity sound in its Display menu. That is a separate setting and stays on that device.'
    )
  })

  it('keeps the sound controls', () => {
    expect(t('overlayEditor.sounds.enable')).toBe('Enable notification sounds')
    expect(t('overlayEditor.sounds.preset')).toBe('Sound preset')
    expect(t('overlayEditor.sounds.volume')).toBe('Volume')
    expect(t('overlayEditor.sounds.test')).toBe('Test sound')
    expect(t('overlayEditor.sounds.cooldown')).toBe('Cooldown')
    // Suffixed onto the slider's number, leading space and all.
    expect(t('overlayEditor.sounds.millisecondsUnit')).toBe(' ms')
  })

  it('keeps the preset names the picker offers', () => {
    // The render site title-cased soundPlayer's lowercase PRESET_NAMES with a
    // local capitalize(). A language whose casing rules differ cannot derive
    // the display name from the stored value, so each preset gets a key.
    expect(t('overlayEditor.sounds.presetChime')).toBe('Chime')
    expect(t('overlayEditor.sounds.presetPop')).toBe('Pop')
    expect(t('overlayEditor.sounds.presetPing')).toBe('Ping')
  })

  it('keeps the custom sound URL field and its upsell', () => {
    expect(t('overlayEditor.sounds.customUrl')).toBe('Custom sound URL')
    expect(t('overlayEditor.sounds.customUrlUpsell')).toBe(
      '\u2014 Upload your own notification sound ({emphasis})'
    )
    expect(t('overlayEditor.sounds.customUrlUpsellEmphasis')).toBe('Premium')
    expect(t('overlayEditor.sounds.customUrlPlaceholder')).toBe('https://example.com/sound.mp3')
  })
})

describe('text-to-speech group copy', () => {
  it('keeps the section headers, which the panel renders upper-case as written', () => {
    expect(t('overlayEditor.tts.sectionVoice')).toBe('VOICE')
    expect(t('overlayEditor.tts.sectionAdvanced')).toBe('ADVANCED (ELEVENLABS)')
    expect(t('overlayEditor.tts.sectionThrottling')).toBe('THROTTLING')
    expect(t('overlayEditor.tts.sectionContent')).toBe('CONTENT')
    expect(t('overlayEditor.tts.sectionPriority')).toBe('PRIORITY')
  })

  it('keeps the top-level toggle and its unsupported-browser notice', () => {
    expect(t('overlayEditor.tts.enable')).toBe('Enable text-to-speech')
    expect(t('overlayEditor.tts.unsupported')).toBe('This browser does not support text-to-speech.')
  })

  it('keeps the provider choice and its upsell', () => {
    expect(t('overlayEditor.tts.provider')).toBe('Voice provider')
    expect(t('overlayEditor.tts.providerBrowser')).toBe('Browser (free)')
    expect(t('overlayEditor.tts.providerElevenLabs')).toBe('ElevenLabs (premium)')
    // Rendered three times as `<PremiumUpsellLink /> to use ElevenLabs voices.`
    // — the link has its own copy, so only the trailing clause is here. It is
    // one key, not three, because the three sites were byte-identical.
    expect(t('overlayEditor.tts.upsellSuffix')).toBe('to use ElevenLabs voices.')
  })

  it('keeps the browser voice picker', () => {
    expect(t('overlayEditor.tts.browserVoice')).toBe('Voice')
    expect(t('overlayEditor.tts.browserVoiceDefault')).toBe('Default')
    expect(t('overlayEditor.tts.browserVoiceNote')).toBe(
      'Browser voice \u2014 list depends on your OS/browser.'
    )
  })

  it('keeps the speech sliders and the test button', () => {
    expect(t('overlayEditor.tts.volume')).toBe('Volume')
    expect(t('overlayEditor.tts.rate')).toBe('Speech rate')
    expect(t('overlayEditor.tts.pitch')).toBe('Pitch')
    expect(t('overlayEditor.tts.test')).toBe('Test voice')
  })

  it('keeps the throttling controls', () => {
    expect(t('overlayEditor.tts.filterMode')).toBe('Which messages are spoken')
    expect(t('overlayEditor.tts.filterModeAll')).toBe('All')
    expect(t('overlayEditor.tts.filterModeSample')).toBe('Sample')
    expect(t('overlayEditor.tts.filterModePriorityOnly')).toBe('Priority-only')
    expect(t('overlayEditor.tts.sampleRate')).toBe('Sample rate')
    expect(t('overlayEditor.tts.sampleRateNote')).toBe('Chance a non-priority message is spoken.')
    expect(t('overlayEditor.tts.maxQueue')).toBe('Max queue length')
    expect(t('overlayEditor.tts.messagesPerMinute')).toBe('Messages per minute')
    expect(t('overlayEditor.tts.userCooldown')).toBe('Per-user cooldown')
    expect(t('overlayEditor.tts.staleness')).toBe('Drop messages older than')
    // Suffixed onto the number by NumberControl; the leading space is the
    // separator and is part of the copy.
    expect(t('overlayEditor.tts.secondsUnit')).toBe(' s')
  })

  it('keeps the content controls', () => {
    expect(t('overlayEditor.tts.readUsername')).toBe('Read username')
    expect(t('overlayEditor.tts.readPlatform')).toBe('Read platform name')
    expect(t('overlayEditor.tts.maxMessageLength')).toBe('Max message length')
    expect(t('overlayEditor.tts.charsUnit')).toBe(' chars')
    expect(t('overlayEditor.tts.skipEmoteOnly')).toBe('Skip emote-only messages')
    expect(t('overlayEditor.tts.skipLinks')).toBe('Skip link-only messages')
    expect(t('overlayEditor.tts.platforms')).toBe('Platforms')
  })

  it('keeps the priority controls', () => {
    expect(t('overlayEditor.tts.priorityEvents')).toBe('Announce priority events')
    expect(t('overlayEditor.tts.priorityBitsMin')).toBe('Minimum bits to announce')
  })

  it('keeps the API key field and its states', () => {
    expect(t('overlayEditor.tts.apiKeyLabel')).toBe('ElevenLabs API key')
    expect(t('overlayEditor.tts.apiKeyPlaceholder')).toBe('sk-...')
    expect(t('overlayEditor.tts.apiKeyEncrypted')).toBe(
      'Your key is encrypted server-side and never returned.'
    )
    expect(t('overlayEditor.tts.saveKey')).toBe('Save key')
    expect(t('overlayEditor.tts.savingKey')).toBe('Saving\u2026')
    expect(t('overlayEditor.tts.keySaved')).toBe(
      'Key saved and encrypted. Click Test key to verify.'
    )
    expect(t('overlayEditor.tts.testKey')).toBe('Test key')
    expect(t('overlayEditor.tts.testingKey')).toBe('Testing\u2026')
    expect(t('overlayEditor.tts.removeKey')).toBe('Remove key')
    expect(t('overlayEditor.tts.removingKey')).toBe('Removing\u2026')
    expect(t('overlayEditor.tts.confirmRemoveKey')).toBe('Confirm remove')
  })

  it('keeps the quota readout as one sentence with all three numbers in it', () => {
    // Was five sibling nodes: two locale-formatted numbers, the words, the
    // percentage and a bare '%)'. A language that orders the count after the
    // unit cannot reassemble that.
    expect(t('overlayEditor.tts.quota', { remaining: '9,000', limit: '10,000', percent: 90 })).toBe(
      '9,000 / 10,000 characters this month (90%)'
    )
    expect(t('overlayEditor.tts.quotaUnknown')).toBe('Click Test key to see your remaining quota.')
  })

  it('keeps the ElevenLabs voice picker states', () => {
    expect(t('overlayEditor.tts.elevenLabsVoice')).toBe('ElevenLabs voice')
    expect(t('overlayEditor.tts.voicesLoading')).toBe('Loading voices\u2026')
    expect(t('overlayEditor.tts.voicesError')).toBe('Could not load voices')
    expect(t('overlayEditor.tts.voicesPending')).toBe('Voices will load shortly\u2026')
    expect(t('overlayEditor.tts.voicesNeedKey')).toBe('Enter your API key above to load voices.')
    expect(t('overlayEditor.tts.voicesEmpty')).toBe('No voices available')
    expect(t('overlayEditor.tts.saveVoice')).toBe('Save voice')
    expect(t('overlayEditor.tts.savingVoice')).toBe('Saving voice\u2026')
  })

  it('keeps the OBS URL panel copy', () => {
    expect(t('overlayEditor.tts.obsUrlNote')).toBe(
      'Paste this URL into OBS as your browser source to enable ElevenLabs TTS.'
    )
    expect(t('overlayEditor.tts.obsUrlLabel')).toBe(
      'OBS URL \u2014 copy and paste into OBS browser source'
    )
    expect(t('overlayEditor.tts.copyObsUrl')).toBe('Copy OBS URL')
    expect(t('overlayEditor.tts.regenerateObsUrl')).toBe('Regenerate URL')
    expect(t('overlayEditor.tts.regeneratingObsUrl')).toBe('Regenerating\u2026')
    expect(t('overlayEditor.tts.regenerateConfirmTitle')).toBe('Regenerate OBS URL?')
    expect(t('overlayEditor.tts.regenerateConfirmBody')).toBe(
      'This invalidates the current OBS URL. You\u2019ll need to paste the new URL into OBS.'
    )
    expect(t('overlayEditor.tts.cancel')).toBe('Cancel')
  })
})
