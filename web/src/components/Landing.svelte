<script lang="ts">
  import { auth } from '../lib/api';

  function handleLogin() {
    auth.login();
  }
</script>

<div class="landing">
  <div class="container">
    <h1>All-Chat</h1>
    <p class="subtitle">Universal Twitch Chat Overlay Service</p>

    <div class="features">
      <div class="feature">
        <h3>🎨 Multi-Platform Emotes</h3>
        <p>Support for 7TV, BetterTTV, and FrankerFaceZ</p>
      </div>

      <div class="feature">
        <h3>⚡ Real-Time Updates</h3>
        <p>WebSocket-powered instant message delivery</p>
      </div>

      <div class="feature">
        <h3>🎯 Easy Integration</h3>
        <p>Simple browser source for OBS and other streaming software</p>
      </div>
    </div>

    <button class="login-button" onclick={handleLogin}>
      <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
        <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
      </svg>
      Login with Twitch
    </button>
  </div>
</div>

<style>
  .landing {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  }

  .container {
    text-align: center;
    max-width: 800px;
    padding: 2rem;
  }

  h1 {
    font-size: 4rem;
    margin: 0;
    background: linear-gradient(to right, #fff, #9147ff);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .subtitle {
    font-size: 1.5rem;
    margin-top: 0.5rem;
    opacity: 0.9;
  }

  .features {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 2rem;
    margin: 3rem 0;
  }

  .feature {
    background: rgba(255, 255, 255, 0.1);
    padding: 1.5rem;
    border-radius: 12px;
    backdrop-filter: blur(10px);
  }

  .feature h3 {
    margin: 0 0 0.5rem 0;
  }

  .feature p {
    margin: 0;
    opacity: 0.8;
    font-size: 0.9rem;
  }

  .login-button {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    background: #9147ff;
    color: white;
    border: none;
    padding: 1rem 2rem;
    font-size: 1.2rem;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    font-weight: 600;
  }

  .login-button:hover {
    background: #772ce8;
    transform: translateY(-2px);
    box-shadow: 0 8px 16px rgba(145, 71, 255, 0.4);
  }

  .login-button svg {
    width: 24px;
    height: 24px;
  }
</style>
