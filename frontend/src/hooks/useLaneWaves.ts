/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * useLaneWaves — the WebGL lane renderer behind the homepage hero.
 *
 * One fullscreen canvas draws the five platform bands. Their boundaries
 * undulate continuously: each lane's height walks a smooth two-sine time
 * series around its real weekly share (laneWeight), so the stack reads as
 * a live chart instead of stepped ticks. The canvas owns the fills; the
 * DOM keeps every piece of text (marquee, wordmarks), so i18n, selection
 * and crisp text rendering stay untouched.
 *
 * Geometry: one TRIANGLE_STRIP with two vertices per lane boundary
 * (boundaries 0..5, left/right corners = 12 vertices). Each vertex's
 * clip-space y is its boundary's cumulative share, so the strip tiles all
 * five bands in a single draw call. Band color is flat-shaded from the
 * provoking vertex (last of each triangle = the strip's upper boundary),
 * hence `boundary - 1` as the lane index.
 *
 * The render loop never calls setState — it mutates the returned
 * weightsRef, which the hero reads from its own rAF loop to size the flex
 * lanes and breathe the counter. Canvas and DOM share one clock, no
 * re-render churn.
 *
 * Degradation: without WebGL2 (or after a context loss) the canvas stays
 * blank and activeRef stays false — the hero keeps the CSS lane
 * backgrounds, so lanes lose their waves but never their color. Reduced
 * motion: one static frame at the exact base shares.
 */

'use client'

import { useEffect, useRef } from 'react'

/** Fixed platform count; the shaders hardcode this. */
export const LANE_COUNT = 5

/** Wave amplitude per lane, relative to its base share. */
const WAVE_AMPLITUDE = 0.22

/**
 * Smooth pseudo time series around one lane's base share: a slow primary
 * wave plus a faster secondary ripple, phase-shifted per lane so the lanes
 * never breathe in sync. Decorative — no weekly history exists
 * server-side — but the continuous rise-and-fall reads like a live chart.
 * Deterministic in (lane, seconds); sin(0) === 0 keeps the reduced-motion
 * frame and the first animation frame at the exact base shares, so the
 * canvas never disagrees with the server-rendered lane sizes.
 */
export function laneWeight(base: number, laneIndex: number, seconds: number): number {
  const phase = laneIndex * 1.3
  const wave = Math.sin(seconds * 0.45 + phase) + 0.5 * Math.sin(seconds * 1.15 + phase * 2)
  return Math.max(base * (1 + wave * WAVE_AMPLITUDE), 2)
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

const VERT_SRC = `#version 300 es
uniform float u_weights[${LANE_COUNT}];
flat out float v_lane;
void main() {
  int boundary = gl_VertexID / 2;
  int corner = gl_VertexID % 2;
  float total = 0.0;
  float below = 0.0;
  for (int i = 0; i < ${LANE_COUNT}; i++) {
    total += u_weights[i];
    if (i < boundary) below += u_weights[i];
  }
  // Band 0 sits at the top: clip y runs +1 (boundary 0) to -1 (bottom).
  float y = 1.0 - 2.0 * (below / total);
  // Provoking vertex of every strip triangle is an upper-boundary corner,
  // so boundary - 1 names the band the triangle fills.
  v_lane = clamp(float(boundary) - 1.0, 0.0, float(${LANE_COUNT}) - 1.0);
  gl_Position = vec4(float(corner) * 2.0 - 1.0, y, 0.0, 1.0);
}`

const FRAG_SRC = `#version 300 es
precision mediump float;
flat in float v_lane;
uniform vec3 u_colors[${LANE_COUNT}];
uniform float u_alphas[${LANE_COUNT}];
out vec4 o_color;
void main() {
  o_color = vec4(u_colors[int(v_lane)], u_alphas[int(v_lane)]);
}`

export interface LaneWavesOptions {
  /** Real (or fallback) weekly share per lane, palette order. */
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

    const uWeights = gl.getUniformLocation(prog, 'u_weights')
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

    const weights = weightsRef.current
    const draw = (seconds: number) => {
      for (let i = 0; i < LANE_COUNT; i++) {
        weights[i] = laneWeight(basesRef.current[i] ?? 1, i, seconds)
      }
      const w = canvas.clientWidth
      const h = canvas.clientHeight
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w
        canvas.height = h
        gl.viewport(0, 0, w, h)
      }
      gl.uniform1fv(uWeights, weights)
      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)
      // 12 vertices: boundary pairs 0..5, left/right corners.
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, LANE_COUNT * 2 + 2)
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
