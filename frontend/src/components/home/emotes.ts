/**
 * This file is part of All-Chat.
 *
 * All-Chat is free software: you can redistribute it and/or modify it under
 * the terms of the GNU Affero General Public License as published by the
 * Free Software Foundation, version 3 of the License.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
 * FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
 * for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Emote registry for the public landing page.
 *
 * The marquee lanes and the demo feed promise native emote rendering, so they
 * show actual emote artwork instead of dimmed text tokens. The files under
 * /emotes are self-hosted so the logged-out homepage makes zero third-party
 * requests (the same download recipe as marketing/public/emotes).
 *
 * Every name here comes from a provider's own global emote set (7TV "Global
 * Emotes", BTTV global, or the top-usage 7TV mirrors for emotes no global set
 * carries), so the artwork is the canonical art — 7TV's fuzzy search returns
 * look-alike channel uploads that share the name, which is how a "GIGACHAD"
 * ends up being somebody's bird emote.
 *
 * .webp entries are the providers' animated files (feedback: "the animated
 * emotes don't play" — static first frames read as broken artwork). .png
 * entries are emotes with no animated upload anywhere; their static frame is
 * the canonical art.
 */
/** Emote names the landing page can render. Each has a /emotes/<slug>.<ext>. */
export type EmoteToken = keyof typeof EMOTE_SRC

/** Token → self-hosted file. `Record` keeps a missing file a compile error. */
export const EMOTE_SRC = {
  peepoHappy: '/emotes/peepohappy.webp',
  peepoSad: '/emotes/peeposad.webp',
  PepePls: '/emotes/pepepls.webp',
  peepoPls: '/emotes/peepopls.webp',
  SourPls: '/emotes/sourpls.webp',
  monkaS: '/emotes/monkas.webp',
  KEKW: '/emotes/kekw.webp',
  catJAM: '/emotes/catjam.webp',
  POGGERS: '/emotes/poggers.webp',
  BasedGod: '/emotes/basedgod.webp',
  ApuApustaja: '/emotes/apuapustaja.webp',
  EZ: '/emotes/ez.webp',
  Clap: '/emotes/clap.webp',
  GIGACHAD: '/emotes/gigachad.webp',
  BibleThump: '/emotes/biblethump.webp',
  FeelsGoodMan: '/emotes/feelsgoodman.png',
  FeelsBadMan: '/emotes/feelsbadman.png',
  KKona: '/emotes/kkona.webp',
  haHAA: '/emotes/hahaa.webp',
  AYAYA: '/emotes/ayaya.webp',
  OMEGALUL: '/emotes/omegalul.png',
  LuL: '/emotes/lul.webp',
  gachiBASS: '/emotes/gachibass.webp',
  WAYTOODANK: '/emotes/waytoodank.webp',
  PartyParrot: '/emotes/partyparrot.webp',
  PETPET: '/emotes/petpet.webp',
  peepoHey: '/emotes/peepohey.webp',
  Stare: '/emotes/stare.webp',
  xdx: '/emotes/xdx.webp',
  FeelsOkayMan: '/emotes/feelsokayman.webp',
  RebeccaBlack: '/emotes/rebeccablack.webp',
  forsenPls: '/emotes/forsenpls.webp',
} as const
