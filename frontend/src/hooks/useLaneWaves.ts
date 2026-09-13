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
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * useLaneWaves — the WebGL stacked-area hero.
 *
 * One fullscreen quad, evaluated per pixel in the fragment shader: every
 * lane's thickness is a smooth function of horizontal position and time
 * (a two-sine field, phase-shifted per lane), the five curves are stacked
 * and filled to the bottom, and the stack is re-normalized per column so
 * the fills always sum to exactly the stage height while every lane
 * keeps its 10% floor at every x. The result reads as a filled stacked
 * line graph that never stops moving — not as five straight bands.
 *
 * The same curve is evaluated on the CPU (laneWeight) for the DOM text
 * lanes, so flex lanes, canvas fills and the counter all trace one
 * chart.
 *
 * The render loop never calls setState — it mutates the returned
 * weightsRef, which the hero reads from its own rAF loop to size the flex
 * lanes and breathe the counter. Canvas and DOM share one clock, no
 * re-render churn.
 *
 * Degradation: without WebGL2 (or after a context loss) the canvas stays
 * blank and activeRef stays false — the hero keeps the CSS lane
 * backgrounds, so lanes lose their waves but never their color. Reduced
 * motion: one static frame at the exact base weights.
 */

'use client'

import { useEffect, useRef } from 'react'

/** Fixed platform count; the shaders hardcode this. */
export const LANE_COUNT = 5

/** Guaranteed minimum lane thickness as a fraction of the stage: with five
 * lanes the floors take half the hero and the remaining half is split by
 * real share. Raw weekly counts are huge, so a floor expressed in those
 * units is meaningless — normalization (laneBaseWeights) turns shares
 * into stage fractions first, and only then does this floor bite. */
export const LANE_FLOOR = 0.1

/** Share of the stage left over once every lane has its floor. */
const FLEX_BUDGET = 1 - LANE_FLOOR * LANE_COUNT

/**
 * Raw weekly counts → normalized lane weights: every lane gets its 10%
 * floor plus a slice of the remaining budget proportional to its share.
 * Output sums to 1, so the vector doubles directly as a flex-grow vector.
 * Missing entries default to share 1, and an all-zero input yields equal
 * weights — fallback (40) and live (400000) counts produce the same lane
 * sizes, which is what the zero-guard in LanesHero relies on.
 */
export function laneBaseWeights(shares: readonly number[]): number[] {
  const total = shares.reduce((sum, share) => sum + share, 0)
  const safeTotal = total > 0 ? total : LANE_COUNT
  return Array.from({ length: LANE_COUNT }, (_, i) => {
    const share = shares[i] ?? 1
    return LANE_FLOOR + FLEX_BUDGET * (share / safeTotal)
  })
}

/**
 * Wave amplitude per lane, as a fraction of that lane's FLEX mass (the
 * budget above the floor): small lanes get bigger relative swings so
 * their movement stays perceptible, but because the floor term is never
 * modulated they can never wave below their 10%.
 */
const WAVE_AMPLITUDES = [0.16, 0.2, 0.24, 0.28, 0.32] as const

// The wave field below is mirrored between GLSL and TypeScript: any change
// to the formula must land in both places — the DOM lanes must trace the
// same curves the canvas paints, or the text rides a different chart than
// its fill. The -1..1 x range spans the canvas width.

/**
 * Smooth pseudo time series around one lane's base weight at horizontal
 * position x (−1 left edge, +1 right edge): a slow primary wave plus a
 * faster secondary ripple, phase-shifted per lane so the lanes never
 * breathe in sync. Decorative — no weekly history exists server-side —
 * but the continuous rise-and-fall reads like a live chart.
 * Only the flex mass above LANE_FLOOR is modulated, so the floor never
 * moves and a lane cannot undulate out of view. Deterministic in
 * (lane, x, seconds); the phase term is subtracted out so the value at
 * seconds=0 is exactly the base — the reduced-motion frame and the first
 * animation frame match the server-rendered lane sizes (a plain
 * sin(s + phase) would start offset, disagreeing with the SSR markup).
 * The x terms vanish at x=0 (screen center), so the CPU mirror
 * there evaluates the same time-only wave the center column paints and
 * the DOM text lanes trace their fill; toward the edges the x terms pull
 * each lane's curve out of phase with the center, which is what makes
 * the boundaries undulate horizontally.
 */
export function laneWeight(base: number, laneIndex: number, x: number, seconds: number): number {
  const flex = Math.max(base - LANE_FLOOR, 0)
  const phase = laneIndex * 1.3
  const wave =
    Math.sin(seconds * 0.45 + phase + x * 1.6) +
    0.5 * Math.sin(seconds * 1.15 + phase * 2 - x * 2.4) -
    (Math.sin(phase) + 0.5 * Math.sin(phase * 2))
  return base + flex * wave * WAVE_AMPLITUDES[laneIndex % WAVE_AMPLITUDES.length]
}

// Lane colors duplicated from the .lanes-home palette (globals.css): the
// GL context cannot resolve CSS custom properties, and a literal pins the
// values the pre-hydration CSS background already painted, so canvas and
// fallback agree exactly.
const LANE_COLORS = ['#8464d6', '#d95c50', '#62aeb4', '#56b847', '#6a72c9'] as const

// Per-lane tint opacity. 0.24 matches the 24% color-mix the CSS fallback
// paints, but over near-black every color is worth a different fraction:
// green and indigo darken to a murmur at the same alpha that leaves red
// loud. Each lane gets the alpha that paints an equal perceptual wash, and
// the two curves below are calibrated to that page background.
const LANE_ALPHAS = [0.24, 0.24, 0.26, 0.3, 0.32] as const

// The fragment shader evaluates the whole stacked chart per pixel: lane
// weights as a smooth function of x and time, stacked cumulatively, and
// filled to the bottom of the stage. A single full-screen triangle strip
// covers the quad; v_x interpolates −1..1 left to right.
const VERT_SRC = `#version 300 es
out float v_x;
out float v_y;
void main() {
  // One full-screen triangle strip of 4 vertices (two triangles).
  vec2 corners[4] = vec2[4](
    vec2(-1.0, -1.0), vec2(1.0, -1.0), vec2(-1.0, 1.0), vec2(1.0, 1.0)
  );
  v_x = corners[gl_VertexID].x;
  v_y = corners[gl_VertexID].y;
  gl_Position = vec4(corners[gl_VertexID], 0.0, 1.0);
}`

// Mirrors laneWeight() in TypeScript above (same sines, same phases, same
// amplitudes): the CPU evaluates the identical field at x=0 to size the
// DOM lanes, so text and fill trace one chart. Amplitudes and base
// weights arrive as uniforms so a stats refetch never rebuilds the GL
// program — the field shape stays compiled, only the numbers move.
const FRAG_SRC = `#version 300 es
precision mediump float;
in float v_x;
in float v_y;
uniform float u_time;
uniform float u_bases[${LANE_COUNT}];
uniform float u_amps[${LANE_COUNT}];
uniform vec3 u_colors[${LANE_COUNT}];
uniform float u_alphas[${LANE_COUNT}];
out vec4 o_color;

const float FLOOR = ${LANE_FLOOR.toFixed(2)};
// Flex budget: stage fraction left once every lane has its floor.
const float FLEX = ${(1 - LANE_FLOOR * LANE_COUNT).toFixed(2)};

float laneWeight(float base, int lane, float x, float s) {
  float flex = max(base - FLOOR, 0.0);
  float phase = float(lane) * 1.3;
  float wave =
    sin(s * 0.45 + phase + x * 1.6) +
    0.5 * sin(s * 1.15 + phase * 2.0 - x * 2.4) -
    (sin(phase) + 0.5 * sin(phase * 2.0));
  return base + flex * wave * u_amps[lane];
}

void main() {
  // Raw modulated weights at this column. At s=0, x=0 every lane is
  // exactly its base (phase subtracted), which pins the first frame to
  // the SSR markup.
  float w[${LANE_COUNT}];
  for (int i = 0; i < ${LANE_COUNT}; i++) {
    w[i] = laneWeight(u_bases[i], i, v_x, u_time);
  }
  // Per-column re-normalization of the flex mass: after the wave moves
  // mass between lanes, the part above each floor is rescaled so the
  // five floors plus the shared flex budget tile the stage exactly.
  // Because only mass above the floor is redistributed, every lane
  // keeps its full floor — the 10% minimum holds at every x and t.
  // Σflex is always > 0: the bases sum to 1 and the floors to 0.5.
  float fm[${LANE_COUNT}];
  float fmTotal = 0.0;
  for (int i = 0; i < ${LANE_COUNT}; i++) {
    fm[i] = max(w[i] - FLOOR, 0.0);
    fmTotal += fm[i];
  }
  for (int i = 0; i < ${LANE_COUNT}; i++) {
    w[i] = FLOOR + FLEX * fm[i] / fmTotal;
  }
  // Stacked lookup: clip-space v_y runs +1 (stage top) to -1 (bottom);
  // lane i fills [cum, cum + w[i]] of the unit stack, top-down.
  float y = (1.0 - v_y) / 2.0;
  float cum = 0.0;
  int lane = -1;
  for (int i = 0; i < ${LANE_COUNT}; i++) {
    // Last lane inclusive at the stage bottom: cum+w sums to exactly 1.0,
    // and the 'y <' test would otherwise discard the bottom pixel row.
    if (y >= cum && (y < cum + w[i] || i == ${LANE_COUNT} - 1)) lane = i;
    cum += w[i];
  }
  if (lane < 0) discard;
  o_color = vec4(u_colors[lane], u_alphas[lane]);
}`

export interface LaneWavesOptions {
  /** Normalized lane weights (laneBaseWeights output), palette order. */
  bases: readonly number[]
  /** Current prefers-reduced-motion resolution. */
  reducedMotion: boolean
}

/**
 * Render the waving lanes and expose their live weights. The returned
 * weights are in bases order, mutated in place every frame; callers that
 * need them (the hero's flex lanes and counter) read the ref from their
 * own rAF loop. activeRef flips true once a frame has actually drawn, so
 * the caller can drop its CSS fallback backgrounds behind the canvas.
 */
export function useLaneWaves(
  canvasRef: React.RefObject<HTMLCanvasElement | null>,
  { bases, reducedMotion }: LaneWavesOptions
): {
  weightsRef: React.RefObject<number[]>
  activeRef: React.RefObject<boolean>
} {
  const weightsRef = useRef<number[]>([...bases])
  const activeRef = useRef(false)
  // bases arrive as a prop; mirror into a ref so a stats refetch never
  // re-creates the GL context (effect deps stay [canvasRef, reducedMotion]).
  const basesRef = useRef(bases)
  basesRef.current = bases

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const gl = canvas.getContext('webgl2', { alpha: true, antialias: true })
    // No WebGL2: leave the canvas transparent and activeRef false — the
    // hero keeps the CSS lane backgrounds.
    if (!gl) return

    const compile = (type: number, src: string) => {
      const shader = gl.createShader(type)
      if (!shader) return null
      gl.shaderSource(shader, src)
      gl.compileShader(shader)
      return gl.getShaderParameter(shader, gl.COMPILE_STATUS) ? shader : null
    }
    const vert = compile(gl.VERTEX_SHADER, VERT_SRC)
    const frag = compile(gl.FRAGMENT_SHADER, FRAG_SRC)
    if (!vert || !frag) return
    const prog = gl.createProgram()
    if (!prog) return
    gl.attachShader(prog, vert)
    gl.attachShader(prog, frag)
    gl.linkProgram(prog)
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return
    gl.useProgram(prog)

    const vao = gl.createVertexArray()
    gl.bindVertexArray(vao)
    // gl_VertexID carries all geometry, so no attributes are enabled; some
    // drivers want a buffer bound anyway, hence one empty placeholder.
    const buf = gl.createBuffer()
    gl.bindBuffer(gl.ARRAY_BUFFER, buf)
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(12), gl.STATIC_DRAW)

    const uTime = gl.getUniformLocation(prog, 'u_time')
    const uBases = gl.getUniformLocation(prog, 'u_bases')
    const uAmps = gl.getUniformLocation(prog, 'u_amps')
    const uColors = gl.getUniformLocation(prog, 'u_colors')
    const uAlphas = gl.getUniformLocation(prog, 'u_alphas')
    const colors = new Float32Array(LANE_COLORS.length * 3)
    LANE_COLORS.forEach((hex, i) => {
      colors[i * 3] = parseInt(hex.slice(1, 3), 16) / 255
      colors[i * 3 + 1] = parseInt(hex.slice(3, 5), 16) / 255
      colors[i * 3 + 2] = parseInt(hex.slice(5, 7), 16) / 255
    })
    gl.uniform3fv(uColors, colors)
    gl.uniform1fv(uAlphas, LANE_ALPHAS)
    gl.uniform1fv(uAmps, WAVE_AMPLITUDES)

    const weights = weightsRef.current
    // Bases arrive in two hops: fallback weights on first paint, live
    // stats a moment later. The draw loop eases its own copy toward
    // basesRef so a late /stats glides the boundaries in instead of
    // snapping a single frame. Time-constant form (not per-frame blend)
    // keeps the ease frame-rate independent; reduced motion draws only at
    // t=0, so it copies once and stays put.
    const displayedBases = [...basesRef.current]
    const BASE_EASE_TAU = 0.8
    let lastTs = 0
    const draw = (seconds: number) => {
      const ts = performance.now()
      const dt = lastTs === 0 ? 0 : (ts - lastTs) / 1000
      lastTs = ts
      if (dt > 0) {
        const blend = 1 - Math.exp(-dt / BASE_EASE_TAU)
        for (let i = 0; i < LANE_COUNT; i++) {
          const target = basesRef.current[i] ?? 1
          displayedBases[i] += (target - displayedBases[i]) * blend
        }
      }
      // CPU mirror of the shader's center column: raw wave, then the
      // same flex renormalization (LANE_FLOOR + FLEX_BUDGET * fm/Σfm), so
      // the DOM lanes trace the same curves the canvas paints at x=0 —
      // text lanes sit centered on their fills.
      let fmTotal = 0
      for (let i = 0; i < LANE_COUNT; i++) {
        const raw = laneWeight(displayedBases[i], i, 0, seconds)
        const fm = Math.max(raw - LANE_FLOOR, 0)
        weights[i] = fm
        fmTotal += fm
      }
      for (let i = 0; i < LANE_COUNT; i++) {
        weights[i] = LANE_FLOOR + FLEX_BUDGET * (weights[i] / fmTotal)
      }
      const w = canvas.clientWidth
      const h = canvas.clientHeight
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w
        canvas.height = h
        gl.viewport(0, 0, w, h)
      }
      gl.uniform1f(uTime, seconds)
      gl.uniform1fv(uBases, displayedBases)
      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)
      // 4 vertices: one full-screen strip.
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4)
      activeRef.current = true
    }

    let raf = 0
    if (reducedMotion) {
      draw(0)
    } else {
      const start = performance.now()
      const loop = () => {
        draw((performance.now() - start) / 1000)
        raf = window.requestAnimationFrame(loop)
      }
      raf = window.requestAnimationFrame(loop)
    }
    return () => {
      if (raf !== 0) window.cancelAnimationFrame(raf)
      gl.deleteProgram(prog)
      gl.deleteShader(vert)
      gl.deleteShader(frag)
      gl.deleteBuffer(buf)
      gl.deleteVertexArray(vao)
    }
  }, [canvasRef, reducedMotion])

  return { weightsRef, activeRef }
}
