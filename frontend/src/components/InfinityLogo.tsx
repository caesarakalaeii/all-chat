'use client'

import { useEffect, useRef } from 'react'

/**
 * InfinityLogo — animated 4-colour infinity snake inside a chat bubble.
 * Used in AppNav, AdminNav, landing page, and loading screens.
 */
export function InfinityLogo({ size = 36 }: { size?: number }) {
  const svgRef = useRef<SVGSVGElement>(null)
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    const segs = svg.querySelectorAll<SVGPathElement>('.inf-seg')
    const segBs = svg.querySelectorAll<SVGPathElement>('.inf-seg-b')
    if (!segs.length) return

    const total = segs[0].getTotalLength()
    const SEG_FRAC = 0.55
    const LOOP_MS = 6000
    const seg = total * SEG_FRAC
    const piece = seg / 4

    function tick(now: number) {
      const head = ((now / LOOP_MS) * total) % total
      segs.forEach((path, ci) => {
        const colourOffset = ci * piece
        const t = ((head - colourOffset) % total + total) % total
        path.style.strokeDasharray = `${piece} ${total * 2}`
        path.style.strokeDashoffset = `${-t}`
        const b = segBs[ci]
        if (b) {
          b.style.strokeDasharray = `${piece} ${total * 2}`
          b.style.strokeDashoffset = `${-(t - total)}`
        }
      })
      rafRef.current = requestAnimationFrame(tick)
    }

    rafRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafRef.current)
  }, [])

  const inf = 'M6 10c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8'

  return (
    <div
      className="relative flex items-center justify-center shrink-0"
      style={{ width: size, height: size }}
      aria-hidden="true"
    >
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className="absolute inset-0">
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M4.84836 2.771C7.18302 2.42773 9.57113 2.25 12.0003 2.25C14.4292 2.25 16.8171 2.4277 19.1516 2.77091C21.1299 3.06177 22.5 4.79445 22.5 6.74056V12.7595C22.5 14.7056 21.1299 16.4382 19.1516 16.7291C17.2123 17.0142 15.2361 17.1851 13.2302 17.2348C13.1266 17.2374 13.0318 17.2788 12.9638 17.3468L8.78033 21.5303C8.56583 21.7448 8.24324 21.809 7.96299 21.6929C7.68273 21.5768 7.5 21.3033 7.5 21V17.045C6.60901 16.9634 5.72491 16.8579 4.84836 16.729C2.87004 16.4381 1.5 14.7054 1.5 12.7593V6.74064C1.5 4.79455 2.87004 3.06188 4.84836 2.771Z"
          fill="rgba(255,255,255,0.07)"
          stroke="rgba(255,255,255,0.10)"
          strokeWidth="0.5"
        />
      </svg>
      <svg
        ref={svgRef}
        style={{
          position: 'absolute',
          width: size * 0.67,
          height: size * 0.39,
          transform: 'translateY(-10%)',
          filter: 'drop-shadow(0 0 3px rgba(0,0,0,0.9))',
        }}
        viewBox="0 0 24 14"
        fill="none"
      >
        <path d={inf} stroke="rgba(255,255,255,0.08)" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#9146FF" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#9146FF" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#FF0000" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#FF0000" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#53FC18" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#53FC18" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#69C9D0" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#69C9D0" strokeWidth="2.5" strokeLinecap="round" />
      </svg>
    </div>
  )
}
