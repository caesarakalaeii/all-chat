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
 * useLaneWaves — the WebGL stacked-area graph behind the homepage hero.
 *
 * One fullscreen canvas draws the five platform bands as a filled,
 * stacked time-series chart: the x axis is time (the last
 * WINDOW_SECONDS slide slowly from right to left — the right edge is
 * "now"), the y axis is each platform's share of all traffic. Shares
 * change as traffic changes: HomeClient polls /api/v1/stats and turns
 * consecutive samples into per-platform rate deltas (modulations), and
 * each eased share enters the graph at the right edge, then scrolls
 * left with everything else. A small two-sine shimmer rides on the
 * targets so the curves never freeze between polls.
 *
 * Smoothness: per-lane shares are sampled into a ring at a fixed grid
 * cadence, and the per-frame columns are interpolated with Catmull-Rom
 * splines over that grid (plus the live value as the terminal point).
 * The interpolation runs on the CPU — a few dozen kilobytes of vertex
 * data per frame — because a spline over a ring buffer is
 * trivially wrong-or-right in JS, while the same indexing in GLSL is a
 * debugging session. Each band is one TRIANGLE_STRIP zigzagging top and
 * bottom vertices per column, five draw calls with a per-call lane
 * uniform for the color.
 *
 * The render loop never calls setState — it mutates the returned
 * weightsRef with the live (right-edge) shares, which the hero reads
 * from its own rAF loop to size the flex lanes, so the DOM text rows
 * track the graph exactly where they sit (the right edge, under the
 * wordmarks). Canvas and DOM share one clock, no re-render churn.
 *
 * Fills are solid brand colors, deliberately: a stacked chart reads by
 * its block colors, and the 24% tints the first version aimed for were
 * never what the canvas actually painted (un-premultiplied rgba with
 * blending disabled composites at full saturation). Solid fills also
 * carry the dark marquee ink at AA contrast on every band. If
 * translucent tints ever come back, premultiply the shader output or
 * create the context with premultipliedAlpha: false — do not just
 * lower the alpha again.
 *
 * Degradation: without WebGL2 (or after a context loss) the canvas
 * stays blank and activeRef stays false — the hero keeps the CSS lane
 * backgrounds (solid brand colors, same vocabulary), so lanes lose
 * their graph but never their color. Reduced motion: one static frame
 * at the base shares — no scrolling, no rAF loop.
 */

'use client'

import { useEffect, useRef } from 'react'

/** Fixed platform count; the shader and buffers hardcode this. */
export const LANE_COUNT = 5
/** Wave amplitude per lane, relative to its base share. */
const WAVE_AMPLITUDE = 0.06

/**
 * The decorative shimmer only: a slow primary wave plus a faster secondary
 * ripple, phase-shifted per lane so the lanes never breathe in sync. Kept
 * small (feedback: the ±22% version swamped the real traffic steps and the
 * graph read as idle oscillation, not a time series). Deterministic in
 * (lane, seconds) so the ring can be prefilled with synthetic history at
 * negative clock values on mount — the graph is full-width immediately.
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

/** Seconds of history visible across the canvas width. */
const WINDOW_SECONDS = 30
/** Grid cadence the ring samples shares at; the spline smooths between. */
const SAMPLE_INTERVAL = 1
/** Ring depth: enough grid slots that the visible window never reaches
 * past the oldest spline control point (see the draw loop's column math). */
const SAMPLES = WINDOW_SECONDS / SAMPLE_INTERVAL + 4
/** Columns drawn across the canvas; one interpolated column per slice. */
const COLUMNS = 160

const VERT_SRC = `#version 300 es
in vec2 a_pos;
void main() {
  gl_Position = vec4(a_pos, 0.0, 1.0);
}`

const FRAG_SRC = `#version 300 es
precision mediump float;
uniform vec3 u_colors[${LANE_COUNT}];
uniform int u_lane;
out vec4 o_color;
void main() {
  o_color = vec4(u_colors[u_lane], 1.0);
}`

export interface LaneWavesOptions {
  /** Real (or fallback) weekly share per lane, palette order. */
  bases: readonly number[]
  /**
   * Per-lane live-rate modulation, palette order: 1 = platform moving at
   * its weekly-average pace, >1 hotter, <1 quieter. Derived by HomeClient
   * from consecutive /api/v1/stats samples.
   */
  modulations: readonly number[]
  /** Current prefers-reduced-motion resolution. */
  reducedMotion: boolean
}

/**
 * Render the scrolling stacked lane graph and expose its live weights.
 * The returned weights are in bases order, mutated in place every frame
 * with the right-edge ("now") shares; callers that need them (the hero's
 * flex lanes) read the ref from their own rAF loop. activeRef flips true
 * once a frame has actually drawn, so the caller can drop its CSS
 * fallback backgrounds behind the canvas.
 */
export function useLaneWaves(
  canvasRef: React.RefObject<HTMLCanvasElement | null>,
  { bases, modulations, reducedMotion }: LaneWavesOptions
): {
  weightsRef: React.RefObject<number[]>
  activeRef: React.RefObject<boolean>
} {
  const weightsRef = useRef<number[]>([...bases])
  const activeRef = useRef(false)
  // bases/modulations arrive as props; mirror them into refs so a stats
  // poll never re-creates the GL context (effect deps stay [canvasRef,
  // reducedMotion]).
  const basesRef = useRef(bases)
  basesRef.current = bases
  const modulationsRef = useRef(modulations)
  modulationsRef.current = modulations

  // Live shares ease toward their targets instead of snapping, so a
  // stats-poll step enters the graph as a glide, not a jump at the right
  // edge. Per-frame lerp factor; ~2 s to close most of the gap at 60fps.
  const eased = useRef<number[]>([...bases])
  const EASE = 0.015

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
    const buf = gl.createBuffer()
    gl.bindBuffer(gl.ARRAY_BUFFER, buf)
    const aPos = gl.getAttribLocation(prog, 'a_pos')
    gl.enableVertexAttribArray(aPos)
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0)

    const uLane = gl.getUniformLocation(prog, 'u_lane')
    const uColors = gl.getUniformLocation(prog, 'u_colors')
    const colors = new Float32Array(LANE_COLORS.length * 3)
    LANE_COLORS.forEach((hex, i) => {
      colors[i * 3] = parseInt(hex.slice(1, 3), 16) / 255
      colors[i * 3 + 1] = parseInt(hex.slice(3, 5), 16) / 255
      colors[i * 3 + 2] = parseInt(hex.slice(5, 7), 16) / 255
    })
    gl.uniform3fv(uColors, colors)

    // Target share for a lane at a time: weekly base × live modulation ×
    // the decorative shimmer. The live edge eases toward this target.
    const targetAt = (lane: number, seconds: number) =>
      laneWeight(
        (basesRef.current[lane] ?? 1) * (modulationsRef.current[lane] ?? 1),
        lane,
        seconds
      )

    // --- Time-series state -------------------------------------------------
    // ring[k] holds the shares sampled at time lastSample -
    // (ring.length - 1 - k) * SAMPLE_INTERVAL; the live value is the chain's
    // terminal point. Prefilled with synthetic history so the graph is
    // full-width from the first frame instead of wiping in from the right.
    const prefill = (atClock: number) => {
      const rows: number[][] = []
      for (let k = 0; k < SAMPLES; k++) {
        const seconds = atClock - (SAMPLES - 1 - k) * SAMPLE_INTERVAL
        rows.push(Array.from({ length: LANE_COUNT }, (_, lane) => targetAt(lane, seconds)))
      }
      return rows
    }
    const ring = prefill(0)
    let lastSample = 0

    // Per-frame scratch, allocated once. chain = ring grid + live edge;
    // column holds one column's interpolated per-lane shares.
    const chain: number[][] = Array.from({ length: SAMPLES + 1 }, () => new Array<number>(LANE_COUNT))
    const column = new Array<number>(LANE_COUNT)
    // One TRIANGLE_STRIP per band, zigzagging top/bottom vertex per
    // column: [x,top, x,bottom] repeated COLUMNS times per lane.
    const vertices = new Float32Array(LANE_COUNT * COLUMNS * 2 * 2)

    /**
     * Catmull-Rom interpolation of all lanes at chain position c in
     * [0, SAMPLES], written into `out`. Integers hit grid/live points,
     * end control points are clamped, tangents are the standard
     * half-difference of neighbors (0.5 uniform tension).
     */
    const splineAt = (c: number, out: number[]) => {
      const i = Math.max(Math.min(Math.floor(c), SAMPLES - 1), 0)
      const t = c - i
      const t2 = t * t
      const t3 = t2 * t
      const p0 = chain[i - 1 < 0 ? 0 : i - 1]
      const p1 = chain[i]
      const p2 = chain[i + 1]
      const p3 = chain[i + 2 > SAMPLES ? SAMPLES : i + 2]
      for (let lane = 0; lane < LANE_COUNT; lane++) {
        const a = p1[lane]
        const b = p2[lane]
        const m1 = 0.5 * (b - p0[lane])
        const m2 = 0.5 * (p3[lane] - a)
        out[lane] =
          (2 * a + m1 - 2 * b - m2) * t3 + (-3 * a - 2 * m1 + 3 * b + m2) * t2 + m1 * t + a
      }
    }

    const draw = (clock: number) => {
      // 1. Ease the live shares toward their targets — the graph must
      //    glide, never step, so a stats-poll jump enters as a curve.
      //    Both the right edge and the ring snapshots come from the eased
      //    signal, keeping the whole series one continuous curve.
      for (let lane = 0; lane < LANE_COUNT; lane++) {
        const target = targetAt(lane, clock)
        eased.current[lane] += (target - eased.current[lane]) * EASE
      }

      // 2. Commit due grid samples from the eased signal. If the tab slept
      //    past the whole window (rAF pauses while hidden), drop the stale
      //    history and restart the ring at now instead of synthesizing
      //    dozens of samples.
      if (clock - lastSample > WINDOW_SECONDS) {
        ring.length = 0
        ring.push([...eased.current])
        lastSample = clock
      }
      while (clock - lastSample >= SAMPLE_INTERVAL) {
        lastSample += SAMPLE_INTERVAL
        ring.push([...eased.current])
        ring.shift()
      }

      // 3. Build the interpolation chain: grid history plus the live
      //    eased value as the terminal point. The right edge of the
      //    graph is "now", so the flex lanes and wordmarks track it.
      for (let lane = 0; lane < LANE_COUNT; lane++) {
        chain[SAMPLES][lane] = eased.current[lane]
        weightsRef.current[lane] = eased.current[lane]
      }
      for (let k = 0; k < SAMPLES; k++) chain[k] = ring[k]

      // Fraction of the current grid interval already elapsed. The
      // column→chain mapping below adds it, so the graph scrolls by the
      // exact passage of time each frame instead of jumping one slot
      // whenever a grid sample commits.
      const frac = (clock - lastSample) / SAMPLE_INTERVAL

      const w = canvas.clientWidth
      const h = canvas.clientHeight
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w
        canvas.height = h
        gl.viewport(0, 0, w, h)
      }
      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)

      // 4. Columns: one spline evaluation per column, then stack each
      //    lane's share cumulatively into clip-space y. Each band's
      //    region in the vertex buffer is a TRIANGLE_STRIP that zigzags
      //    top/bottom per column — that ordering is what fills the band's
      //    interior (a polygon outline walked as one loop is NOT a strip).
      //    Band 0 is the top band; clip y is +1 at the top.
      //    Column col covers (col / COLUMNS) of the last WINDOW_SECONDS,
      //    mapped onto the chain at time-exact position with +frac; the
      //    two-slot slack keeps every visible spline segment's control
      //    points inside the ring, including at frac close to 1.
      const span = WINDOW_SECONDS / SAMPLE_INTERVAL
      for (let col = 0; col < COLUMNS; col++) {
        const c = SAMPLES - 2 - span + (col / (COLUMNS - 1)) * span + frac
        splineAt(c, column)
        let total = 0
        for (let lane = 0; lane < LANE_COUNT; lane++) total += column[lane]
        const x = (col / (COLUMNS - 1)) * 2 - 1
        let cumulative = 0
        for (let lane = 0; lane < LANE_COUNT; lane++) {
          cumulative += column[lane]
          const top = 1 - (2 * cumulative) / total
          // Bottom boundary of a band = top boundary of the band above
          // (clip 1 for the topmost band).
          const bottom =
            lane === 0 ? 1 : vertices[(lane - 1) * COLUMNS * 4 + col * 4 + 1]
          const v = lane * COLUMNS * 4 + col * 4
          vertices[v] = x
          vertices[v + 1] = top
          vertices[v + 2] = x
          vertices[v + 3] = bottom
        }
      }

      // 4. Upload once, draw each band's zigzag strip from its region.
      gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.DYNAMIC_DRAW)
      for (let lane = 0; lane < LANE_COUNT; lane++) {
        gl.uniform1i(uLane, lane)
        gl.drawArrays(gl.TRIANGLE_STRIP, lane * COLUMNS * 2, COLUMNS * 2)
      }

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
