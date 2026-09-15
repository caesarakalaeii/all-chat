#!/bin/sh
# tiktok-signer container entrypoint: start a display, then the service.
#
# Page-viewer mode (SIGNER_VIEWER_MODE=page) runs Chromium non-headless, and
# TikTok judges the rendering stack (measured 2026-09-15: software-only Xvfb
# gets im/fetch 403, GPU-backed X gets 200 with full payloads). So:
#
#  - /dev/dri present (GPU passed through from the node): start Xorg on the
#    GPU directly — a hardware-backed display.
#  - no GPU: fall back to Xvfb (signature mode still works headless; page mode
#    is expected to fail against TikTok's bot detection in this shape).
#
# SIGNER_VIEWER_MODE=signature skips the display entirely.

set -e

if [ "${SIGNER_VIEWER_MODE:-signature}" = "page" ]; then
  export DISPLAY="${SIGNER_DISPLAY:-:99}"
  if [ -e /dev/dri/renderD128 ]; then
    echo '{"level":"info","message":"starting Xorg on node GPU"}'
    # Direct rendering on the passthrough device; no xorg.conf needed for
    # kmsro-mode drivers, but X wants a config that does not fight the
    # read-only root filesystem — keep everything in /tmp.
    Xorg -noreset -logfile /tmp/xorg.log -config /dev/null \
      "${DISPLAY#:}" 2>/tmp/xorg-stderr.log &
  else
    echo '{"level":"info","message":"no /dev/dri; falling back to Xvfb (page mode may fail)"}'
    Xvfb "${DISPLAY}" -screen 0 1920x1080x24 2>/tmp/xvfb-stderr.log &
  fi
  # Give the display a moment; Chromium fails fast without one.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if xdpyinfo -display "${DISPLAY}" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

exec node dist/index.js
