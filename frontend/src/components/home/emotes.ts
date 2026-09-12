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
 * show actual emote artwork instead of dimmed text tokens. The PNGs under
 * /emotes are self-hosted first frames so the logged-out homepage makes zero
 * third-party requests (the same download recipe as marketing/public/emotes).
 *
 * Every name here comes from a provider's own global emote set (7TV "Global
 * Emotes", BTTV global, or the top-usage FFZ/xqc mirrors for emotes no global
 * set carries), so the artwork is the canonical art — 7TV's fuzzy search
 * returns look-alike channel uploads that share the name, which is how a
 * "GIGACHAD" ends up being somebody's bird emote.
 */
/** Emote names the landing page can render. Each has a /emotes/<slug>.png. */
export type EmoteToken = keyof typeof EMOTE_SRC

/** Token → self-hosted PNG path. `Record` keeps a missing file a compile error. */
export const EMOTE_SRC = {
  peepoHappy: '/emotes/peepohappy.png',
  peepoSad: '/emotes/peeposad.png',
  PepePls: '/emotes/pepepls.png',
  peepoPls: '/emotes/peepopls.png',
  SourPls: '/emotes/sourpls.png',
  monkaS: '/emotes/monkas.png',
  KEKW: '/emotes/kekw.png',
  catJAM: '/emotes/catjam.png',
  POGGERS: '/emotes/poggers.png',
  BasedGod: '/emotes/basedgod.png',
  ApuApustaja: '/emotes/apuapustaja.png',
  EZ: '/emotes/ez.png',
  Clap: '/emotes/clap.png',
  GIGACHAD: '/emotes/gigachad.png',
  BibleThump: '/emotes/biblethump.png',
  FeelsGoodMan: '/emotes/feelsgoodman.png',
  FeelsBadMan: '/emotes/feelsbadman.png',
  KKona: '/emotes/kkona.png',
  haHAA: '/emotes/hahaa.png',
  AYAYA: '/emotes/ayaya.png',
  OMEGALUL: '/emotes/omegalul.png',
  LuL: '/emotes/lul.png',
  gachiBASS: '/emotes/gachibass.png',
  WAYTOODANK: '/emotes/waytoodank.png',
  PartyParrot: '/emotes/partyparrot.png',
  PETPET: '/emotes/petpet.png',
  peepoHey: '/emotes/peepohey.png',
  Stare: '/emotes/stare.png',
  xdx: '/emotes/xdx.png',
  FeelsOkayMan: '/emotes/feelsokayman.png',
  RebeccaBlack: '/emotes/rebeccablack.png',
  forsenPls: '/emotes/forsenpls.png',
}
