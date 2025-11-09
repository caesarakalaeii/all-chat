import './app.css'
import App from './App.svelte'
import OverlayViewer from './components/OverlayViewer.svelte'

// Simple router: check if we're on an overlay page
const path = window.location.pathname
const overlayMatch = path.match(/^\/overlay\/([^/]+)/)

if (overlayMatch) {
  // Overlay viewer page
  const overlayId = overlayMatch[1]
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token') || localStorage.getItem('access_token') || ''

  const app = new OverlayViewer({
    target: document.getElementById('app')!,
    props: {
      overlayId,
      token
    }
  })
} else {
  // Main app (landing/dashboard)
  const app = new App({
    target: document.getElementById('app')!,
  })
}
