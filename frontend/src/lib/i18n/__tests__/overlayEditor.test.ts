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

  it('keeps the outline thickness slider label', () => {
    expect(t('overlayEditor.typography.outlineThickness')).toBe('Outline thickness')
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
    // The three preset display names moved to common.soundPresets.* once the
    // monitor view's activity sound became their second reader.
    // __tests__/common.test.ts pins them now.
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

describe('source card copy', () => {
  it('keeps the per-source status and action labels', () => {
    expect(t('overlayEditor.sources.chatViaEventsub')).toBe('Chat via EventSub')
    expect(t('overlayEditor.sources.reconnectChat')).toBe('Reconnect to enable chat')
    expect(t('overlayEditor.sources.revoke')).toBe('Revoke')
    expect(t('overlayEditor.sources.configureRelay')).toBe('Configure relay')
    expect(t('overlayEditor.sources.streamSelection')).toBe('Stream selection')
    expect(t('overlayEditor.sources.removeLabel', { channel: 'somechannel' })).toBe(
      'Remove somechannel'
    )
  })

  it('keeps the remove confirmation as one sentence naming the channel', () => {
    // The render site wrapped the channel name in <strong> mid-sentence, so the
    // sentence stays whole and `emphasise` re-wraps the run. See
    // src/lib/i18n/emphasise.tsx for why the fragments are not split.
    expect(t('overlayEditor.sources.removeConfirmTitle')).toBe('Remove source?')
    expect(t('overlayEditor.sources.removeConfirmBody', { emphasis: 'somechannel' })).toBe(
      'Remove somechannel from this overlay. Chat messages from this source will stop appearing.'
    )
    expect(t('overlayEditor.sources.remove')).toBe('Remove')
    expect(t('overlayEditor.sources.cancel')).toBe('Cancel')
  })

  it('keeps the empty and skeleton states', () => {
    expect(t('overlayEditor.sources.empty')).toBe('No sources added yet. Add a platform below.')
    expect(t('overlayEditor.sources.sharedHeading')).toBe('Shared Overlays')
    expect(t('overlayEditor.sources.sharedOwner', { owner: 'someone' })).toBe("someone's overlay")
    expect(t('overlayEditor.sources.add')).toBe('+ Add')
  })
})

describe('stream selection panel copy', () => {
  it('keeps the strategy field labels and hints', () => {
    expect(t('overlayEditor.streamSelection.strategyLabel')).toBe('Stream selection strategy')
    expect(t('overlayEditor.streamSelection.strategyHint')).toBe(
      'When this channel has multiple concurrent live streams, choose which one to monitor.'
    )
    expect(t('overlayEditor.streamSelection.premiumSuffix')).toBe(' (Premium)')
    expect(t('overlayEditor.streamSelection.upsellSuffix')).toBe(
      ' to use advanced stream selection.'
    )
    expect(t('overlayEditor.streamSelection.locked')).toBe(
      'Non-default strategies require a premium subscription.'
    )
  })

  it('keeps the title-keyword field', () => {
    expect(t('overlayEditor.streamSelection.matchLabel')).toBe('Title keyword')
    expect(t('overlayEditor.streamSelection.matchPlaceholder')).toBe('e.g. synthwave, lofi, jazz')
  })

  it('keeps every strategy label and description', () => {
    // The keys are suffixed rather than nested one level deeper: the three-level
    // cap in messages.test.ts forbids streamSelection.firstFound.label.
    expect(t('overlayEditor.streamSelection.firstFoundLabel')).toBe('First found')
    expect(t('overlayEditor.streamSelection.firstFoundDescription')).toBe(
      'Picks the first live stream (default)'
    )
    expect(t('overlayEditor.streamSelection.mostViewersLabel')).toBe('Most viewers')
    expect(t('overlayEditor.streamSelection.mostViewersDescription')).toBe(
      'Picks the stream with the highest viewer count'
    )
    expect(t('overlayEditor.streamSelection.fewestViewersLabel')).toBe('Fewest viewers')
    expect(t('overlayEditor.streamSelection.fewestViewersDescription')).toBe(
      'Picks the stream with the lowest viewer count'
    )
    expect(t('overlayEditor.streamSelection.titleMatchLabel')).toBe('Title match')
    expect(t('overlayEditor.streamSelection.titleMatchDescription')).toBe(
      'Picks the first stream whose title contains a keyword'
    )
    expect(t('overlayEditor.streamSelection.titleMatchAllLabel')).toBe('Title match (all)')
    expect(t('overlayEditor.streamSelection.titleMatchAllDescription')).toBe(
      'Monitors all streams whose title contains a keyword'
    )
    expect(t('overlayEditor.streamSelection.allLabel')).toBe('All streams')
    expect(t('overlayEditor.streamSelection.allDescription')).toBe(
      'Monitors all concurrent live streams simultaneously'
    )
  })
})

describe('Discord relay panel copy', () => {
  it('keeps the relay controls', () => {
    expect(t('overlayEditor.relay.loopFilter')).toBe(
      'Loop filter: active \u2014 Discord messages are never relayed back to Discord.'
    )
    expect(t('overlayEditor.relay.enable')).toBe('Enable relay')
    expect(t('overlayEditor.relay.outboundChannelLabel')).toBe('Outbound channel')
    expect(t('overlayEditor.relay.selectChannel')).toBe('Select a channel...')
  })
})

describe('engagement panel copy', () => {
  it('keeps the load failure and the toggles', () => {
    expect(t('overlayEditor.engagement.loadError')).toBe(
      'Could not load engagement settings. Reload the page to try again.'
    )
    expect(t('overlayEditor.engagement.enablePoints')).toBe('Enable viewer points')
    expect(t('overlayEditor.engagement.announceRounds')).toBe('Announce new rounds in chat')
    expect(t('overlayEditor.engagement.announceRoundsHint')).toBe(
      'Posts the question, numbered options and the participate link to your chat when a round ' +
        'starts. Needs the \u201Cadvanced controls\u201D send permission (the same grant the ' +
        'Monitor view\u2019s chat sending uses) \u2014 without it the announcement is skipped.'
    )
  })

  it('keeps the points explainer as one sentence with the four inline code runs', () => {
    // The paragraph interleaved five text nodes with <code> elements. It stays a
    // single sentence: the code samples are chat commands, not copy, so they
    // arrive as placeholders and the render site re-wraps them in <code>.
    expect(
      t('overlayEditor.engagement.pointsExplainer', {
        pointsName: 'Points',
        voteCommand: '!vote 2',
        bareVote: '2',
        predictCommand: '!predict 1 500',
      })
    ).toBe(
      'Viewers earn Points by supporting the stream (subs, bits, donations, gifts) and by ' +
        'keeping the participation page open, and wager them on predictions. Run polls and ' +
        'predictions from the Monitor View; viewers join straight from chat (!vote 2 or just 2, ' +
        '!predict 1 500) or the participation page \u2014 no install required.'
    )
  })

  it('keeps the points-name field and the save button', () => {
    expect(t('overlayEditor.engagement.pointsNameLabel')).toBe('Points name')
    expect(t('overlayEditor.engagement.pointsNamePlaceholder')).toBe('Points')
    expect(t('overlayEditor.engagement.save')).toBe('Save Engagement Settings')
    expect(t('overlayEditor.engagement.saving')).toBe('Saving...')
  })

  it('keeps every earn-rate field label and hint', () => {
    expect(t('overlayEditor.engagement.bitsMultiplierLabel')).toBe('Points per bit')
    expect(t('overlayEditor.engagement.bitsMultiplierHint')).toBe('Twitch cheers')
    expect(t('overlayEditor.engagement.usdMultiplierLabel')).toBe('Points per USD')
    expect(t('overlayEditor.engagement.usdMultiplierHint')).toBe('donations & Super Chats')
    expect(t('overlayEditor.engagement.subHighLabel')).toBe('Tier 3 sub')
    expect(t('overlayEditor.engagement.subHighHint')).toBe('Twitch Tier 3')
    expect(t('overlayEditor.engagement.subMediumLabel')).toBe('Tier 2 sub')
    expect(t('overlayEditor.engagement.subMediumHint')).toBe('Twitch Tier 2')
    expect(t('overlayEditor.engagement.subLowLabel')).toBe('Base sub / member')
    expect(t('overlayEditor.engagement.subLowHint')).toBe('Tier 1, Prime, Kick & YouTube members')
    expect(t('overlayEditor.engagement.giftPerSubLabel')).toBe('Per gifted sub')
    expect(t('overlayEditor.engagement.giftPerSubHint')).toBe('awarded to the gifter')
    expect(t('overlayEditor.engagement.chatPerMinuteLabel')).toBe('Chatting, per minute')
    expect(t('overlayEditor.engagement.chatPerMinuteHint')).toBe('active chatters')
    expect(t('overlayEditor.engagement.watchPerMinuteLabel')).toBe('Participation page, per min')
    expect(t('overlayEditor.engagement.watchPerMinuteHint')).toBe(
      'while the viewer keeps the participate page open (not stream-watch time)'
    )
    expect(t('overlayEditor.engagement.comingSoonSuffix')).toBe(' (coming soon)')
  })

  it('keeps the earn-rate validation toasts naming the offending field', () => {
    expect(t('overlayEditor.engagement.invalidValue', { field: 'Points per bit' })).toBe(
      'Invalid value for "Points per bit"'
    )
    expect(t('overlayEditor.engagement.mustBeWhole', { field: 'Tier 3 sub' })).toBe(
      '"Tier 3 sub" must be a whole number'
    )
  })

  it('keeps the widget and viewer link copy', () => {
    expect(t('overlayEditor.engagement.linksHeading')).toBe('Widget & viewer links')
    expect(t('overlayEditor.engagement.pollWidgetLabel')).toBe('OBS poll widget')
    expect(t('overlayEditor.engagement.pollWidgetDescription')).toBe(
      'Browser source that shows the live poll'
    )
    expect(t('overlayEditor.engagement.predictionWidgetLabel')).toBe('OBS prediction widget')
    expect(t('overlayEditor.engagement.predictionWidgetDescription')).toBe(
      'Browser source that shows the live prediction'
    )
    expect(t('overlayEditor.engagement.participateLabel')).toBe('Viewer participation page')
    expect(t('overlayEditor.engagement.participateDescription')).toBe(
      'Viewers vote, wager and check their balance \u2014 no install needed'
    )
    expect(t('overlayEditor.engagement.copyLink')).toBe('Copy link')
    expect(t('overlayEditor.engagement.copiedLink')).toBe('Copied!')
    expect(t('overlayEditor.engagement.copyLinkFailed')).toBe('Could not copy the link')
  })

  it('keeps the browser-source guidance with the emphasised source type', () => {
    expect(t('overlayEditor.engagement.browserSourceHint', { emphasis: 'Browser Source' })).toBe(
      'In OBS/Streamlabs: add a Browser Source, paste a widget URL, and set it to your canvas ' +
        'size (e.g. 1920\u00D71080). The widgets are transparent and only appear while a round ' +
        'is live.'
    )
    expect(t('overlayEditor.engagement.browserSourceHintEmphasis')).toBe('Browser Source')
    expect(t('overlayEditor.engagement.participateShareHint')).toBe(
      'Share the participation link with mobile viewers \u2014 put it on-screen or in your ' +
        'channel panels so they can join without the extension.'
    )
  })

  it('keeps the Twitch mirroring section', () => {
    expect(t('overlayEditor.engagement.mirroringHeading')).toBe('Twitch native mirroring')
    expect(t('overlayEditor.engagement.mirroringBody')).toBe(
      'Mirror your native Twitch polls & predictions onto All-Chat overlays (read-only \u2014 ' +
        'viewers still vote in Twitch). Opt-in; it adds read-only Twitch scopes and takes ' +
        'effect after the next channel sync (a stream restart or re-adding the source).'
    )
    expect(t('overlayEditor.engagement.enableMirroring')).toBe('Enable Twitch mirroring')
  })
})

describe('add-source form copy', () => {
  it('keeps the intro and the four OAuth buttons', () => {
    expect(t('overlayEditor.addSource.intro')).toBe('Connect a platform to this overlay.')
    expect(t('overlayEditor.addSource.connectTwitch')).toBe('Connect Twitch')
    expect(t('overlayEditor.addSource.connectYoutube')).toBe('Connect YouTube')
    expect(t('overlayEditor.addSource.connectKick')).toBe('Connect Kick')
    expect(t('overlayEditor.addSource.connectTiktok')).toBe('Connect TikTok')
    expect(t('overlayEditor.addSource.connectDiscord')).toBe('Connect Discord')
  })

  it('keeps the no-Discord-server prompt as one sentence around its link', () => {
    // Was three JSX children with the link in the middle. The link text is the
    // {emphasis} run so the sentence survives a word-order change.
    expect(t('overlayEditor.addSource.discordNeedsServer', { emphasis: 'Settings' })).toBe(
      'Connect a Discord server in Settings first to add Discord sources.'
    )
    expect(t('overlayEditor.addSource.discordNeedsServerEmphasis')).toBe('Settings')
  })

  it('keeps the Discord channel picker', () => {
    expect(t('overlayEditor.addSource.channelLabel')).toBe('Channel')
    expect(t('overlayEditor.addSource.selectChannel')).toBe('Select a channel...')
    expect(t('overlayEditor.addSource.back')).toBe('Back')
    expect(t('overlayEditor.addSource.cancel')).toBe('Cancel')
    expect(t('overlayEditor.addSource.add')).toBe('Add')
    expect(t('overlayEditor.addSource.addingEllipsis')).toBe('Adding...')
  })

  it('keeps the TikTok dialog', () => {
    expect(t('overlayEditor.addSource.tiktokTitle')).toBe('Connect TikTok')
    expect(t('overlayEditor.addSource.tiktokBody')).toBe(
      // &apos; in the JSX is U+0027, a straight apostrophe, not the curly one
      // other copy in this file uses. Transcribed as it renders.
      "TikTok has no login step here. Enter the creator's username and we'll pull their live chat."
    )
    expect(t('overlayEditor.addSource.tiktokPlaceholder')).toBe('@username')
    // The TikTok dialog uses a real ellipsis where the Discord one uses three
    // dots. Both are transcribed as they were rather than normalised.
    expect(t('overlayEditor.addSource.adding')).toBe('Adding\u2026')
  })

  it('keeps the admin manual-entry form', () => {
    expect(t('overlayEditor.addSource.adminSummary')).toBe('Admin: manual channel ID')
    expect(t('overlayEditor.addSource.adminYoutubePlaceholder')).toBe(
      '@handle, channel URL, or UC\u2026'
    )
    expect(t('overlayEditor.addSource.adminChannelPlaceholder')).toBe('Channel ID or username')
    expect(t('overlayEditor.addSource.adminAdd')).toBe('Add manually')
    expect(t('overlayEditor.addSource.adminResolving')).toBe('Resolving\u2026')
  })
})

describe('editor page chrome copy', () => {
  it('keeps the loading and not-found states', () => {
    expect(t('overlayEditor.page.loadingEditor')).toBe('Loading editor...')
    expect(t('overlayEditor.page.notFound')).toBe('Overlay not found')
    expect(t('overlayEditor.page.returnToDashboard')).toBe('Return to Dashboard')
  })

  it('keeps the header actions', () => {
    expect(t('overlayEditor.page.back')).toBe('Back')
    expect(t('overlayEditor.page.monitorView')).toBe('Monitor View')
    expect(t('overlayEditor.page.monitorViewTitle')).toBe(
      'Open the readable chat & activity monitor in a new tab'
    )
    expect(t('overlayEditor.page.eventSettings')).toBe('Event Settings')
    expect(t('overlayEditor.page.credits')).toBe('Credits')
    expect(t('overlayEditor.page.clone')).toBe('Clone')
    expect(t('overlayEditor.page.cloning')).toBe('Cloning\u2026')
  })

  it('keeps the OBS URL controls', () => {
    expect(t('overlayEditor.page.copyObsUrl')).toBe('Copy OBS URL')
    expect(t('overlayEditor.page.copiedObsUrl')).toBe('Copied!')
    expect(t('overlayEditor.page.obsHelpTrigger')).toBe('How do I add this to OBS?')
    expect(t('overlayEditor.page.obsHelpTitle')).toBe('Add the overlay to OBS')
    expect(t('overlayEditor.page.shareOverlay')).toBe('Share Overlay')
    expect(t('overlayEditor.page.resetToThemeDefaults')).toBe('Reset to theme defaults')
  })

  it('keeps the dock URL controls distinct from the OBS URL ones', () => {
    // Two different destinations — the public overlay for a browser source, the
    // monitor for a browser dock — so the labels must not read alike.
    expect(t('overlayEditor.page.copyDockUrl')).toBe('Copy dock URL')
    expect(t('overlayEditor.page.copiedDockUrl')).toBe('Copied!')
    expect(t('overlayEditor.page.dockHelpTrigger')).toBe('How do I dock chat in OBS?')
    expect(t('overlayEditor.page.dockHelpTitle')).toBe('Add the chat monitor as an OBS dock')
  })

  it('keeps the browser-extension card', () => {
    expect(t('overlayEditor.page.extensionHeading')).toBe('Browser Extension Overlay')
    expect(t('overlayEditor.page.extensionActive')).toBe('Active')
    expect(t('overlayEditor.page.extensionActiveBody')).toBe(
      'This overlay is shown to viewers via the browser extension at allch.at/c/caesarlp.'
    )
    expect(t('overlayEditor.page.extensionInactiveBody')).toBe(
      'Set this as the overlay shown to viewers via the browser extension.'
    )
    expect(t('overlayEditor.page.extensionDeactivate')).toBe('Deactivate')
    expect(t('overlayEditor.page.extensionSetActive')).toBe('Set Active')
  })

  it('keeps the premium-required dialog as whole sentences around their links', () => {
    // Two element runs in one sentence (the upsell link) and one in the next
    // (the Discord invite), so both go through interpolateElements rather than
    // being split into JSX children either side of the anchor.
    expect(
      t('overlayEditor.page.premiumRequiredBody', { upgradeLink: 'Upgrade your account' })
    ).toBe(
      'Sharing your overlay is a premium feature. Upgrade your account to share your chat with other streamers.'
    )
    expect(t('overlayEditor.page.premiumRequiredTitle')).toBe('Premium Feature')
    expect(t('overlayEditor.page.premiumUpgradeLink')).toBe('Upgrade your account')
    expect(t('overlayEditor.page.questionsJoin', { discordLink: 'Discord community' })).toBe(
      'Questions? Join our Discord community.'
    )
    expect(t('overlayEditor.page.discordCommunityLink')).toBe('Discord community')
    expect(t('overlayEditor.page.close')).toBe('Close')
    expect(t('overlayEditor.page.upgrade')).toBe('Upgrade')
  })

  it('keeps the share dialog naming the overlay', () => {
    expect(t('overlayEditor.page.shareTitle')).toBe('Share Overlay')
    expect(t('overlayEditor.page.shareBody', { emphasis: 'My Overlay' })).toBe(
      "Enter the Twitch username of the person you want to share My Overlay with. They'll receive a request they can accept or decline."
    )
    expect(t('overlayEditor.page.shareRecipientLabel')).toBe('Twitch username')
    expect(t('overlayEditor.page.shareRecipientPlaceholder')).toBe('e.g. somestreamer')
    expect(t('overlayEditor.page.shareCancel')).toBe('Cancel')
    expect(t('overlayEditor.page.shareSend')).toBe('Send Request')
    expect(t('overlayEditor.page.shareSending')).toBe('Sending...')
  })
})

describe('message settings copy', () => {
  it('keeps the two sliders with their value in the sentence', () => {
    // The value sits in a coloured <span> mid-label, so the label stays whole
    // and the render site wraps the {value} run. 'Message Duration' appends its
    // unit inside the same span, which is why the unit is part of that key.
    expect(t('overlayEditor.messages.maxMessagesLabel', { value: '42' })).toBe('Max Messages: 42')
    expect(t('overlayEditor.messages.messageDurationLabel', { value: '12s' })).toBe(
      'Message Duration: 12s'
    )
    expect(t('overlayEditor.messages.durationSeconds', { seconds: 12 })).toBe('12s')
  })

  it('keeps the fade and ordering controls', () => {
    expect(t('overlayEditor.messages.disableFade')).toBe('Disable Message Fade Out')
    expect(t('overlayEditor.messages.disableFadeHint')).toBe(
      'Messages stay visible until max is reached'
    )
    expect(t('overlayEditor.messages.invertOrder')).toBe('Invert Message Order')
    expect(t('overlayEditor.messages.invertOrderHint')).toBe(
      'Reverses the reading order so the newest message is listed first. This is the order only \u2014 use Feed Anchor to move the feed to the other edge.'
    )
  })

  it('keeps the feed anchor control', () => {
    expect(t('overlayEditor.messages.feedAnchorLabel')).toBe('Feed Anchor')
    expect(t('overlayEditor.messages.feedAnchorTop')).toBe('Top edge \u2014 feed grows downward')
    expect(t('overlayEditor.messages.feedAnchorBottom')).toBe(
      'Bottom edge \u2014 feed grows upward'
    )
    expect(t('overlayEditor.messages.feedAnchorHint')).toBe(
      'Which edge of the overlay the feed sits on when it is not full. Anchor it to the bottom and each new message pushes the older ones up.'
    )
  })

  it('keeps every entry-animation option', () => {
    expect(t('overlayEditor.messages.entryAnimationLabel')).toBe('Entry Animation')
    expect(t('overlayEditor.messages.entryAnimationHint')).toBe(
      'How new messages appear on the overlay'
    )
    expect(t('overlayEditor.messages.animationDefault')).toBe('Fade + slide up (default)')
    expect(t('overlayEditor.messages.animationFlyLeft')).toBe('Fly in from left')
    expect(t('overlayEditor.messages.animationFlyRight')).toBe('Fly in from right')
    expect(t('overlayEditor.messages.animationFlySpring')).toBe('Fly in with overshoot')
    expect(t('overlayEditor.messages.animationPop')).toBe('Pop in')
    expect(t('overlayEditor.messages.animationBounce')).toBe('Bounce up')
    expect(t('overlayEditor.messages.animationFlip')).toBe('Flip in')
    expect(t('overlayEditor.messages.animationSwoosh')).toBe('Swoosh')
    expect(t('overlayEditor.messages.animationSoftFocus')).toBe('Soft focus')
  })

  it('keeps the emote provider toggles', () => {
    expect(t('overlayEditor.messages.emoteProvidersLabel')).toBe('Emote Providers')
    // The three provider names are third-party products, so they are locked as
    // names rather than translated. They live in the catalog anyway: the render
    // site must not carry literals, and a language may transliterate a name.
    expect(t('overlayEditor.messages.seventv')).toBe('7TV')
    expect(t('overlayEditor.messages.betterttv')).toBe('BetterTTV')
    expect(t('overlayEditor.messages.frankerfacez')).toBe('FrankerFaceZ')
  })

  it('keeps the 7TV emote-set override copy', () => {
    expect(t('overlayEditor.messages.seventvOverrideLabel')).toBe('7TV Emote Set')
    expect(t('overlayEditor.messages.seventvOverrideHint')).toBe(
      'Optional. Paste a 7TV emote-set ID, an emote-set URL, or your 7TV profile URL to attach those emotes to this overlay regardless of which platforms you stream on.'
    )
    expect(t('overlayEditor.messages.seventvCurrentlyActive')).toBe('Currently active: ')
    expect(t('overlayEditor.messages.seventvQuotedName', { name: 'Cool Set' })).toBe('"Cool Set"')
    expect(t('overlayEditor.messages.seventvEmoteCount', { count: 120 })).toBe(' (120 emotes)')
    expect(t('overlayEditor.messages.seventvRemove')).toBe('Remove')
    expect(t('overlayEditor.messages.seventvRemoving')).toBe('Removing\u2026')
    expect(t('overlayEditor.messages.seventvRemoved')).toBe('7TV emote set removed')
    expect(t('overlayEditor.messages.seventvRemoveFailed')).toBe('Failed to remove 7TV emote set')
    expect(t('overlayEditor.messages.seventvReplacePlaceholder')).toBe(
      'Paste a new ID/URL to replace\u2026'
    )
    expect(t('overlayEditor.messages.seventvUrlPlaceholder')).toBe('https://7tv.app/users/...')
    expect(t('overlayEditor.messages.seventvVerify')).toBe('Verify')
    expect(t('overlayEditor.messages.seventvChecking')).toBe('Checking\u2026')
    expect(t('overlayEditor.messages.seventvResolveFailed')).toBe('Could not resolve 7TV reference')
  })

  it('keeps the resolved-reference notice as one sentence', () => {
    // Was four concatenated fragments: 'Resolved', an optional ` to "name"`, an
    // optional ` (n emotes)` and a trailing clause. Word order is the first
    // thing a second language changes, so the four cases are four whole
    // sentences instead.
    expect(t('overlayEditor.messages.seventvResolved')).toBe(
      'Resolved \u2014 click Save Configuration to apply.'
    )
    expect(t('overlayEditor.messages.seventvResolvedNamed', { name: 'Cool Set' })).toBe(
      'Resolved to "Cool Set" \u2014 click Save Configuration to apply.'
    )
    expect(t('overlayEditor.messages.seventvResolvedCounted', { count: 120 })).toBe(
      'Resolved (120 emotes) \u2014 click Save Configuration to apply.'
    )
    expect(
      t('overlayEditor.messages.seventvResolvedNamedCounted', { name: 'Cool Set', count: 120 })
    ).toBe('Resolved to "Cool Set" (120 emotes) \u2014 click Save Configuration to apply.')
  })
})

describe('custom CSS section copy', () => {
  it('keeps the section heading and its theme-state pills', () => {
    expect(t('overlayEditor.customCss.heading')).toBe('Custom CSS')
    expect(t('overlayEditor.customCss.usingTheme', { theme: 'Neon' })).toBe(
      'Using \u201cNeon\u201d theme \u00b7 auto-updates'
    )
    expect(t('overlayEditor.customCss.noThemeApplied')).toBe('No theme applied')
    expect(t('overlayEditor.customCss.customPill')).toBe('Custom CSS')
    expect(t('overlayEditor.customCss.forkedPill')).toBe(
      'Full copy saved \u2014 theme updates paused'
    )
    expect(t('overlayEditor.customCss.layeredPill')).toBe(
      'Customized \u2014 untouched theme rules still auto-update'
    )
    expect(t('overlayEditor.customCss.resetToTheme')).toBe('Reset to theme')
    expect(t('overlayEditor.customCss.clear')).toBe('Clear')
  })

  it('keeps the editor explainer and placeholder', () => {
    expect(t('overlayEditor.customCss.explainer')).toBe(
      'Edit the CSS below \u2014 the preview updates as you type. Only your changes are saved, so fixes we ship to the theme still reach the rules you didn\u2019t touch. Deleting theme rules can\u2019t be layered, so it stores a full copy and pauses theme updates for this overlay; \u201cReset to theme\u201d re-links it.'
    )
    expect(t('overlayEditor.customCss.editorPlaceholder')).toBe('/* Enter your custom CSS here */')
  })

  it('keeps each CSS problem count as its own whole phrase', () => {
    // Was `${n} error${n > 1 ? 's' : ''}`, which builds a plural by appending a
    // letter — a rule that holds in English and almost nowhere else. Singular
    // and plural are separate keys so a language can spell each properly.
    expect(t('overlayEditor.customCss.noProblems')).toBe('\u2713 No CSS problems detected.')
    expect(t('overlayEditor.customCss.errorCountOne', { count: 1 })).toBe('1 error')
    expect(t('overlayEditor.customCss.errorCountMany', { count: 3 })).toBe('3 errors')
    expect(t('overlayEditor.customCss.warningCountOne', { count: 1 })).toBe('1 warning')
    expect(t('overlayEditor.customCss.warningCountMany', { count: 3 })).toBe('3 warnings')
    expect(t('overlayEditor.customCss.problemsSeparator')).toBe(' \u00b7 ')
    expect(t('overlayEditor.customCss.problemsAdvice', { counts: '3 errors' })).toBe(
      '3 errors \u2014 invalid rules are ignored by the browser, so fix these for your styles to take effect. Incomplete rules aren\u2019t previewed.'
    )
  })

  it('keeps the per-issue line reference and the overflow row', () => {
    expect(t('overlayEditor.customCss.issueLine', { line: 12 })).toBe('L12:')
    expect(t('overlayEditor.customCss.moreIssues', { count: 4 })).toBe('\u2026and 4 more')
  })

  it('keeps the theme-docs pointer as one sentence around its link', () => {
    expect(t('overlayEditor.customCss.inspiration', { docsLink: 'theme docs' })).toBe(
      'Need inspiration? Explore theme docs.'
    )
    expect(t('overlayEditor.customCss.themeDocsLink')).toBe('theme docs')
  })
})

describe('testing and danger zone copy', () => {
  it('keeps the mock message form fields', () => {
    expect(t('overlayEditor.testing.platformLabel')).toBe('Platform')
    expect(t('overlayEditor.testing.displayNameLabel')).toBe('Display Name')
    expect(t('overlayEditor.testing.usernameLabel')).toBe('Username')
    expect(t('overlayEditor.testing.avatarUrlLabel')).toBe('Avatar URL')
    expect(t('overlayEditor.testing.avatarUrlPlaceholder')).toBe('https://...')
    expect(t('overlayEditor.testing.nameColorLabel')).toBe('Name Color')
    expect(t('overlayEditor.testing.messageLabel')).toBe('Message')
    expect(t('overlayEditor.testing.messagePlaceholder')).toBe('Type something fun...')
    expect(t('overlayEditor.testing.injectMessage')).toBe('Inject Message')
    expect(t('overlayEditor.testing.sampleChat')).toBe('\u{1F4AC} Sample Chat')
    expect(t('overlayEditor.testing.sampleEvents')).toBe('\u2B50 Sample Events')
  })

  it('keeps the danger zone explainer and its confirm dialog', () => {
    expect(t('overlayEditor.dangerZone.explainer')).toBe(
      'Reset your overlay ID to revoke any leaked OBS URLs. A new overlay with the same configuration will be created and you will be redirected to it. The old overlay and its URL will be permanently deleted.'
    )
    expect(t('overlayEditor.dangerZone.resetOverlayId')).toBe('Reset Overlay ID')
    expect(t('overlayEditor.dangerZone.resetting')).toBe('Resetting\u2026')
    expect(t('overlayEditor.dangerZone.confirmTitle')).toBe('Reset Overlay ID?')
    expect(t('overlayEditor.dangerZone.confirmBody')).toBe(
      'This will create a new overlay with a fresh ID and permanently delete this one. Any existing OBS URLs will stop working \u2014 update your browser source after the reset.'
    )
    expect(t('overlayEditor.dangerZone.cancel')).toBe('Cancel')
    expect(t('overlayEditor.dangerZone.confirmReset')).toBe('Reset ID')
  })

  it('keeps the theme-apply confirm dialog and the save footer', () => {
    expect(t('overlayEditor.page.applyThemeTitle')).toBe('Apply theme?')
    expect(t('overlayEditor.page.applyThemeBody')).toBe(
      'Loading this theme will reset your visual customizations. Continue?'
    )
    expect(t('overlayEditor.page.applyThemeCancel')).toBe('Cancel')
    expect(t('overlayEditor.page.applyThemeContinue')).toBe('Continue')
    expect(t('overlayEditor.page.saveConfiguration')).toBe('Save Configuration')
    expect(t('overlayEditor.page.savingConfiguration')).toBe('Saving...')
  })
})

describe('credit roll settings copy', () => {
  it('keeps the load states and the page header', () => {
    expect(t('overlayEditor.credits.loadingEditor')).toBe('Loading editor...')
    expect(t('overlayEditor.credits.notFound')).toBe('Overlay not found')
    expect(t('overlayEditor.credits.returnToDashboard')).toBe('Return to Dashboard')
    expect(t('overlayEditor.credits.backToOverlay')).toBe('Back to Overlay')
    expect(t('overlayEditor.credits.heading')).toBe('Credit Roll Settings')
    expect(t('overlayEditor.credits.intro')).toBe(
      'Configure end-of-stream credits to showcase viewers who supported your stream with subs, donations, raids, and more.'
    )
  })

  it('keeps the OBS URL control', () => {
    expect(t('overlayEditor.credits.copyObsUrl')).toBe('Copy Credits OBS URL')
    expect(t('overlayEditor.credits.copiedObsUrl')).toBe('Copied!')
    expect(t('overlayEditor.credits.obsUrlHint')).toBe(
      'Add this URL as a Browser Source in OBS to display credits at end of stream'
    )
  })

  it('keeps the enable toggle and the event-type picker', () => {
    expect(t('overlayEditor.credits.enableHeading')).toBe('Enable Credit Roll')
    expect(t('overlayEditor.credits.enableHint')).toBe(
      'Show end-of-stream credits with leaderboards and highlights'
    )
    expect(t('overlayEditor.credits.eventTypesHeading')).toBe('Event Types to Include')
    expect(t('overlayEditor.credits.eventTypesHint')).toBe(
      'Select which types of events should appear in the credit roll leaderboards'
    )
  })

  it('keeps every event-type label', () => {
    expect(t('overlayEditor.credits.eventSubs')).toBe('Subscriptions')
    expect(t('overlayEditor.credits.eventResubs')).toBe('Resubscriptions')
    expect(t('overlayEditor.credits.eventGiftSubs')).toBe('Gift Subs')
    expect(t('overlayEditor.credits.eventBits')).toBe('Bits/Cheers')
    expect(t('overlayEditor.credits.eventRaids')).toBe('Raids')
    expect(t('overlayEditor.credits.eventSuperChats')).toBe('Super Chats')
    expect(t('overlayEditor.credits.eventMemberships')).toBe('Memberships')
    expect(t('overlayEditor.credits.eventFollows')).toBe('Follows')
  })

  it('keeps the leaderboard settings', () => {
    expect(t('overlayEditor.credits.leaderboardHeading')).toBe('Leaderboard Settings')
    expect(t('overlayEditor.credits.topNLabel')).toBe('Top N Users per Category')
    expect(t('overlayEditor.credits.topNHint')).toBe(
      'Show top 1-50 users in each leaderboard category'
    )
    expect(t('overlayEditor.credits.sortByLabel')).toBe('Sort By')
    expect(t('overlayEditor.credits.sortByTotalValue')).toBe('Total Value (monetary amount)')
    expect(t('overlayEditor.credits.sortByCount')).toBe('Count (number of events)')
  })

  it('keeps the display settings', () => {
    expect(t('overlayEditor.credits.displayHeading')).toBe('Display Settings')
    expect(t('overlayEditor.credits.themeLabel')).toBe('Theme')
    expect(t('overlayEditor.credits.themeClassic')).toBe('Classic')
    expect(t('overlayEditor.credits.themeCinematic')).toBe('Cinematic')
    expect(t('overlayEditor.credits.themeModern')).toBe('Modern')
    expect(t('overlayEditor.credits.scrollSpeedLabel')).toBe('Scroll Speed (1-100)')
    // One key with the value in it, shared by the two sliders below it, rather
    // than a bare 'Current:' label glued to a number at the render site.
    expect(t('overlayEditor.credits.currentValue', { value: 50 })).toBe('Current: 50')
    expect(t('overlayEditor.credits.durationLabel')).toBe('Display Duration (seconds)')
    expect(t('overlayEditor.credits.durationHint')).toBe(
      'How long to show the credit roll (10-300 seconds)'
    )
    expect(t('overlayEditor.credits.opacityLabel')).toBe('Background Opacity (0-1)')
  })

  it('keeps the Twitch clips settings', () => {
    expect(t('overlayEditor.credits.clipsHeading')).toBe('Twitch Clips')
    expect(t('overlayEditor.credits.clipsHint')).toBe('Show clips during credit roll')
    expect(t('overlayEditor.credits.maxClipsLabel')).toBe('Maximum Clips')
    expect(t('overlayEditor.credits.fallbackDaysLabel')).toBe('Fallback Days')
    expect(t('overlayEditor.credits.fallbackDaysHint')).toBe(
      'If no clips from this stream, show clips from last N days'
    )
    expect(t('overlayEditor.credits.muteClipsLabel')).toBe('Mute Clips Audio')
    expect(t('overlayEditor.credits.muteClipsHint')).toBe(
      'Required for browser autoplay. Unmuting may require viewer interaction.'
    )
  })

  it('keeps the custom CSS editor section', () => {
    expect(t('overlayEditor.credits.cssHeading')).toBe('Custom CSS Editor')
    expect(t('overlayEditor.credits.cssEnable')).toBe('Enable Custom CSS')
    expect(t('overlayEditor.credits.cssBrowseThemes')).toBe('Browse Themes')
    expect(t('overlayEditor.credits.cssReset')).toBe('Reset')
    expect(t('overlayEditor.credits.cssEditorPlaceholder')).toBe(
      '/* Enter your custom CSS for credit roll */'
    )
    expect(t('overlayEditor.credits.cssHint', { docsLink: 'credit roll theme docs' })).toBe(
      'Customize your credit roll appearance with CSS. Browse themes or write your own styles. See credit roll theme docs for examples and CSS selectors.'
    )
    expect(t('overlayEditor.credits.cssDocsLink')).toBe('credit roll theme docs')
  })

  it('keeps the save and cancel actions', () => {
    expect(t('overlayEditor.credits.save')).toBe('Save Settings')
    expect(t('overlayEditor.credits.saving')).toBe('Saving...')
    expect(t('overlayEditor.credits.cancel')).toBe('Cancel')
  })
})

describe('event display settings page copy', () => {
  it('keeps the page chrome and its tabs', () => {
    expect(t('overlayEditor.eventSettings.back')).toBe('Back to Overlay')
    expect(t('overlayEditor.eventSettings.heading')).toBe('Event Display Settings')
    expect(t('overlayEditor.eventSettings.subheading')).toBe(
      'Control which platform events appear on your overlay.'
    )
    expect(t('overlayEditor.eventSettings.loadFailed')).toBe('Failed to load event settings')
    expect(t('overlayEditor.eventSettings.save')).toBe('Save Settings')
    expect(t('overlayEditor.eventSettings.saving')).toBe('Saving…')
    expect(t('overlayEditor.eventSettings.cancel')).toBe('Cancel')
    expect(t('overlayEditor.eventSettings.tabGlobal')).toBe('Global')
  })

  it('keeps the global tab', () => {
    expect(t('overlayEditor.eventSettings.systemEventsHeading')).toBe('System Events')
    expect(t('overlayEditor.eventSettings.tokenWarningsLabel')).toBe('Token Warnings')
    expect(t('overlayEditor.eventSettings.tokenWarningsDescription')).toBe(
      'Display OAuth authentication errors on overlay (requires token-refresh-service)'
    )
    expect(t('overlayEditor.eventSettings.displaySettingsHeading')).toBe('Display Settings')
    expect(t('overlayEditor.eventSettings.durationMultiplierLabel')).toBe(
      'Event Duration Multiplier'
    )
    expect(t('overlayEditor.eventSettings.durationMultiplierDescription')).toBe(
      'Multiply all event display durations (0.5 = half time, 2.0 = double time)'
    )
  })

  it('keeps the event tier explainer, each tier a whole sentence', () => {
    expect(t('overlayEditor.eventSettings.tiersHeading')).toBe('About event tiers')
    // The leading bullet and the emphasised tier name stay in the sentence as
    // placeholders; splitting on the em dash would leave a translator with
    // fragments that no longer form a sentence.
    expect(t('overlayEditor.eventSettings.tierHigh', { tier: 'High-value' })).toBe(
      '• High-value — subs, large donations, raids: 30+ seconds'
    )
    expect(t('overlayEditor.eventSettings.tierHighName')).toBe('High-value')
    expect(t('overlayEditor.eventSettings.tierMedium', { tier: 'Medium-value' })).toBe(
      '• Medium-value — follows, small gifts: 15 seconds'
    )
    expect(t('overlayEditor.eventSettings.tierMediumName')).toBe('Medium-value')
    expect(t('overlayEditor.eventSettings.tierLow', { tier: 'Low-value' })).toBe(
      '• Low-value — likes, shares: 5–10 seconds'
    )
    expect(t('overlayEditor.eventSettings.tierLowName')).toBe('Low-value')
    // The two CSS class names are protocol, not copy, so they stay at the render
    // site; the sentence around them is a template with both as placeholders.
    expect(
      t('overlayEditor.eventSettings.tierStyling', {
        tierClass: '.event-tier-high',
        typeClass: '.event-type-raid',
      })
    ).toBe('• Style with CSS classes: .event-tier-high, .event-type-raid')
  })

  it('keeps the Twitch event toggles', () => {
    expect(t('overlayEditor.eventSettings.twitchSubsLabel')).toBe('Subscriptions')
    expect(t('overlayEditor.eventSettings.twitchSubsDescription')).toBe(
      'New subscriptions and resubscriptions'
    )
    expect(t('overlayEditor.eventSettings.twitchResubsLabel')).toBe('Resubscriptions')
    expect(t('overlayEditor.eventSettings.twitchResubsDescription')).toBe(
      'Monthly resubscription notices with streak information'
    )
    expect(t('overlayEditor.eventSettings.twitchGiftSubsLabel')).toBe('Gift Subscriptions')
    expect(t('overlayEditor.eventSettings.twitchGiftSubsDescription')).toBe(
      'Gift subs and mystery gift bombs'
    )
    expect(t('overlayEditor.eventSettings.twitchBitsLabel')).toBe('Bits / Cheers')
    expect(t('overlayEditor.eventSettings.twitchBitsDescription')).toBe('Bits cheered in chat')
    expect(t('overlayEditor.eventSettings.twitchRaidsLabel')).toBe('Raids')
    expect(t('overlayEditor.eventSettings.twitchRaidsDescription')).toBe(
      'Incoming raids from other channels'
    )
    expect(t('overlayEditor.eventSettings.twitchChannelPointsLabel')).toBe('Channel Points')
    expect(t('overlayEditor.eventSettings.twitchChannelPointsDescription')).toBe(
      'Channel point reward redemptions (requires EventSub service)'
    )
    expect(t('overlayEditor.eventSettings.twitchFollowsLabel')).toBe('Follows')
    expect(t('overlayEditor.eventSettings.twitchFollowsDescription')).toBe(
      'New channel followers (requires EventSub service)'
    )
    expect(t('overlayEditor.eventSettings.twitchWatchStreaksLabel')).toBe('Watch Streaks')
    expect(t('overlayEditor.eventSettings.twitchWatchStreaksDescription')).toBe(
      "Returning viewers' watch-streak milestones. Turning this off hides the milestone only — their chat message still shows"
    )
  })

  it('keeps the YouTube event toggles', () => {
    expect(t('overlayEditor.eventSettings.youtubeSuperChatLabel')).toBe('Super Chat')
    expect(t('overlayEditor.eventSettings.youtubeSuperChatDescription')).toBe(
      'Paid Super Chat messages'
    )
    expect(t('overlayEditor.eventSettings.youtubeSuperStickerLabel')).toBe('Super Stickers')
    expect(t('overlayEditor.eventSettings.youtubeSuperStickerDescription')).toBe(
      'Paid Super Sticker purchases'
    )
    expect(t('overlayEditor.eventSettings.youtubeMembersLabel')).toBe('New Members')
    expect(t('overlayEditor.eventSettings.youtubeMembersDescription')).toBe(
      'New channel memberships'
    )
    expect(t('overlayEditor.eventSettings.youtubeMemberMilestonesLabel')).toBe('Member Milestones')
    expect(t('overlayEditor.eventSettings.youtubeMemberMilestonesDescription')).toBe(
      'Membership anniversary celebrations'
    )
    expect(t('overlayEditor.eventSettings.youtubeMemberGiftsLabel')).toBe('Membership Gifts')
    expect(t('overlayEditor.eventSettings.youtubeMemberGiftsDescription')).toBe(
      'Gifted memberships'
    )
  })

  it('keeps the Kick event toggles and the reverse-engineering caveat', () => {
    expect(t('overlayEditor.eventSettings.kickSubsLabel')).toBe('Subscriptions')
    expect(t('overlayEditor.eventSettings.kickSubsDescription')).toBe('Kick channel subscriptions')
    // The render site spelled the ampersand &amp;; a catalog string is not HTML.
    expect(t('overlayEditor.eventSettings.kickGiftsLabel')).toBe('Gifts & Donations')
    expect(t('overlayEditor.eventSettings.kickGiftsDescription')).toBe(
      'Gift subscriptions and donations'
    )
    expect(t('overlayEditor.eventSettings.kickCaveat')).toBe(
      '⚠️ Kick events require reverse-engineering and may not be available yet.'
    )
  })

  it('keeps the TikTok event toggles and the coin chest caveat', () => {
    expect(t('overlayEditor.eventSettings.tiktokLikesLabel')).toBe('Likes')
    expect(t('overlayEditor.eventSettings.tiktokLikesDescription')).toBe(
      'Likes sent during stream (aggregated)'
    )
    expect(t('overlayEditor.eventSettings.tiktokGiftsLabel')).toBe('Gifts')
    expect(t('overlayEditor.eventSettings.tiktokGiftsDescription')).toBe(
      'Virtual gifts sent with diamond values'
    )
    expect(t('overlayEditor.eventSettings.tiktokFollowsLabel')).toBe('Follows')
    expect(t('overlayEditor.eventSettings.tiktokFollowsDescription')).toBe(
      'New followers during stream'
    )
    expect(t('overlayEditor.eventSettings.tiktokSharesLabel')).toBe('Shares')
    expect(t('overlayEditor.eventSettings.tiktokSharesDescription')).toBe(
      'Stream shares to other platforms'
    )
    expect(t('overlayEditor.eventSettings.tiktokTreasureChestsLabel')).toBe('Coin Chests')
    // The "best effort" caveat is load bearing — see the comment on the toggle.
    expect(t('overlayEditor.eventSettings.tiktokTreasureChestsDescription')).toBe(
      'Treasure boxes of coins dropped by viewers. Best effort: TikTok does not reliably send these to third-party tools, so they may not appear.'
    )
    expect(t('overlayEditor.eventSettings.advancedHeading')).toBe('Advanced')
    expect(t('overlayEditor.eventSettings.likeWindowLabel')).toBe(
      'Like Aggregation Window (seconds)'
    )
    expect(t('overlayEditor.eventSettings.likeWindowDescription')).toBe(
      'Likes are collected in this window to prevent spam'
    )
  })
})

describe('create overlay page copy', () => {
  it('keeps the form', () => {
    expect(t('overlayEditor.create.heading')).toBe('Create Overlay')
    expect(t('overlayEditor.create.body')).toBe(
      'Give your overlay a name. You can add chat sources after creation.'
    )
    expect(t('overlayEditor.create.nameLabel')).toBe('Overlay Name')
    expect(t('overlayEditor.create.namePlaceholder')).toBe('e.g. Main Stream, TikTok Only')
    expect(t('overlayEditor.create.nameRequired')).toBe('Overlay name is required')
    expect(t('overlayEditor.create.cancel')).toBe('Cancel')
    expect(t('overlayEditor.create.submit')).toBe('Create Overlay')
  })
})

describe('embed preview page copy', () => {
  it('keeps the waiting-for-messages empty state', () => {
    expect(t('overlayEditor.embedPreview.waitingHeading')).toBe('Waiting for messages...')
    expect(t('overlayEditor.embedPreview.waitingBody')).toBe(
      'Messages will appear here when chat is active'
    )
  })
})

describe('overlay editor toast copy', () => {
  it('keeps the create-overlay toasts', () => {
    // The name is quoted in the original, with U+0022 either side.
    expect(t('overlayEditor.toasts.created', { name: 'My Overlay' })).toBe('"My Overlay" created')
    expect(t('overlayEditor.toasts.createFailed')).toBe('Failed to create overlay')
  })

  it('keeps the event-settings toasts', () => {
    expect(t('overlayEditor.toasts.eventSettingsSaved')).toBe('Event settings saved')
    expect(t('overlayEditor.toasts.eventSettingsSaveFailed')).toBe('Failed to save event settings')
  })
})

describe('overlay editor source toast copy', () => {
  it('keeps the stream-selection and relay toasts', () => {
    expect(t('overlayEditor.toasts.streamSelectionSaved')).toBe('Stream selection saved')
    expect(t('overlayEditor.toasts.streamSelectionSaveFailed')).toBe(
      'Failed to save stream selection'
    )
    expect(t('overlayEditor.toasts.relaySaved')).toBe('Relay settings saved')
    expect(t('overlayEditor.toasts.relaySaveFailed')).toBe('Failed to save relay settings')
  })

  it('keeps the engagement toasts', () => {
    expect(t('overlayEditor.toasts.engagementSaved')).toBe('Engagement settings saved')
    expect(t('overlayEditor.toasts.engagementSaveFailed')).toBe(
      'Failed to save engagement settings'
    )
    expect(t('overlayEditor.toasts.twitchConsentFailed')).toBe(
      'Could not start Twitch consent. Please try again.'
    )
  })

  it('keeps the add-source toasts', () => {
    expect(t('overlayEditor.toasts.discordSourceAdded')).toBe('Discord source added')
    expect(t('overlayEditor.toasts.discordSourceAddFailed')).toBe('Failed to add Discord source')
    expect(t('overlayEditor.toasts.sourceAdded')).toBe('Source added')
    expect(t('overlayEditor.toasts.oauthConnectFailed')).toBe('Could not connect')
    expect(t('overlayEditor.toasts.tiktokSourceAdded')).toBe('TikTok source added')
    expect(t('overlayEditor.toasts.tiktokSourceAddFailed')).toBe('Failed to add TikTok source')
    expect(t('overlayEditor.toasts.tiktokSourceAddFailedBody')).toBe(
      'Check the username and try again.'
    )
    expect(t('overlayEditor.toasts.manualSourceAddFailed')).toBe('Failed to add source')
    expect(t('overlayEditor.toasts.manualSourceAddFailedBody')).toBe(
      'Verify the channel ID and try again.'
    )
  })

  it('keeps the OAuth-callback source toasts', () => {
    expect(t('overlayEditor.toasts.oauthSourceAddedBody', { platform: 'youtube' })).toBe(
      'Successfully added youtube source!'
    )
    expect(t('overlayEditor.toasts.youtubePermissionRequired')).toBe('YouTube permission required')
    expect(t('overlayEditor.toasts.youtubePermissionRequiredBody')).toBe(
      'To add your YouTube channel, you must allow All-Chat to see your YouTube account. Please try again and approve the YouTube permission on the Google screen.'
    )
    expect(t('overlayEditor.toasts.youtubeNoChannel')).toBe('No YouTube channel found')
    expect(t('overlayEditor.toasts.youtubeNoChannelBody')).toBe(
      'We could not find a YouTube channel on that Google account. Make sure the account has a YouTube channel, then try again.'
    )
    expect(t('overlayEditor.toasts.oauthSourceAddFailed')).toBe('Failed to add source')
  })

  it('keeps the remove-source and Twitch-reconnect toasts', () => {
    expect(t('overlayEditor.toasts.sourceRemoved')).toBe('Source removed')
    expect(t('overlayEditor.toasts.sourceRemoveFailed')).toBe('Failed to remove source')
    expect(t('overlayEditor.toasts.twitchChatConnected')).toBe('Twitch chat connected')
    expect(t('overlayEditor.toasts.twitchChatReconnectFailed')).toBe(
      'Could not reconnect Twitch chat'
    )
  })

  it('keeps the shared-overlay toasts', () => {
    expect(t('overlayEditor.toasts.sharedOverlayAdded')).toBe('Shared overlay added')
    expect(t('overlayEditor.toasts.sharedOverlayAddedBody', { sender: 'Ada' })).toBe(
      "Added Ada's overlay"
    )
    expect(t('overlayEditor.toasts.sharedOverlayAddFailed')).toBe('Failed to add shared overlay')
    // The share-revoked notification names the revoker; 'someone' stands in when
    // the socket envelope omits the username.
    expect(t('overlayEditor.toasts.shareRevoked')).toBe('Share revoked')
    expect(t('overlayEditor.toasts.shareRevokedBody', { revoker: 'Ada' })).toBe(
      'Your share with Ada was revoked'
    )
    expect(t('overlayEditor.toasts.shareRevokedUnknownRevoker')).toBe('someone')
    expect(t('overlayEditor.toasts.shareRequestSent', { username: 'ada' })).toBe(
      'Share request sent to ada'
    )
    expect(t('overlayEditor.toasts.shareRequestFailed')).toBe('Failed to send share request')
  })

  it('keeps the clone, reset and mock-message toasts', () => {
    expect(t('overlayEditor.toasts.cloned')).toBe('Overlay cloned')
    expect(t('overlayEditor.toasts.cloneFailed')).toBe('Failed to clone overlay')
    // U+2014 em dash and U+2026 ellipsis, both as in the original.
    expect(t('overlayEditor.toasts.overlayIdReset')).toBe(
      'Overlay ID reset \u2014 redirecting\u2026'
    )
    expect(t('overlayEditor.toasts.overlayIdResetFailed')).toBe('Failed to reset overlay ID')
    expect(t('overlayEditor.toasts.mockMessageFailed')).toBe('Failed to send mock message')
  })

  it('keeps the extension-overlay toasts', () => {
    expect(t('overlayEditor.toasts.extensionOverlaySet')).toBe('Extension overlay set')
    expect(t('overlayEditor.toasts.extensionOverlaySetBody')).toBe(
      'This overlay will be shown in the browser extension.'
    )
    expect(t('overlayEditor.toasts.extensionOverlayUnset')).toBe('Extension overlay deactivated')
    expect(t('overlayEditor.toasts.extensionOverlayUpdateFailed')).toBe('Failed to update overlay')
  })
})

describe('TTS group toast and inline error copy', () => {
  it('keeps the API key save errors', () => {
    expect(t('overlayEditor.tts.apiKeyEmptyError')).toBe('API key cannot be empty.')
    expect(t('overlayEditor.tts.pickVoiceError')).toBe('Pick a voice before saving.')
    expect(t('overlayEditor.tts.saveKeyError')).toBe('Could not save. Try again.')
  })

  it('keeps the API key toasts', () => {
    expect(t('overlayEditor.tts.apiKeySavedToast')).toBe('API key saved.')
    expect(t('overlayEditor.tts.apiKeySaveFailedToast')).toBe('Could not save key')
    // Stands in for the thrown error's message when the failure is not an Error.
    expect(t('overlayEditor.tts.networkErrorDetail')).toBe('network error')
    expect(t('overlayEditor.tts.apiKeyRemovedToast')).toBe('API key removed.')
    expect(t('overlayEditor.tts.apiKeyRemoveFailedToast')).toBe('Could not remove key. Try again.')
  })

  it('keeps the ElevenLabs key-test toasts, one per status code', () => {
    expect(t('overlayEditor.tts.testInvalidKeyToast')).toBe('Invalid API key')
    // U+2014 em dash.
    expect(t('overlayEditor.tts.testRateLimitedToast')).toBe(
      'Rate-limited \u2014 try again in a minute'
    )
    expect(t('overlayEditor.tts.testUnreachableToast')).toBe(
      'Could not reach ElevenLabs. Check your connection.'
    )
    expect(t('overlayEditor.tts.testUnavailableToast')).toBe('ElevenLabs service unavailable')
  })

  it('keeps the voice and OBS URL toasts', () => {
    expect(t('overlayEditor.tts.voiceUpdatedToast')).toBe('Voice updated.')
    expect(t('overlayEditor.tts.voiceSaveFailedToast')).toBe('Could not save voice')
    expect(t('overlayEditor.tts.obsUrlCopiedToast')).toBe('OBS URL copied.')
    expect(t('overlayEditor.tts.obsUrlCopyFailedToast')).toBe('Could not copy URL.')
    expect(t('overlayEditor.tts.obsUrlRegeneratedToast')).toBe('New OBS URL copied to clipboard.')
    expect(t('overlayEditor.tts.obsUrlRegenerateFailedToast')).toBe(
      'Could not regenerate URL. Try again.'
    )
  })
})
