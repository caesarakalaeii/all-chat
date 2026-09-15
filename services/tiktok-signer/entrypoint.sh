#!/bin/sh
# tiktok-signer container entrypoint: start a display, then the service.
#
# Page-viewer mode (SIGNER_VIEWER_MODE=page) runs Chromium non-headless on a
# display. Measured 2026-09-15: Xvfb + Mesa llvmpipe passes TikTok's session
# gating (the earlier failures were --disable-gpu/SwiftShader, not Xvfb);
# /dev/dri passthrough (Xorg on the node GPU) stays wired as the stronger
# option for nodes that have one.
#
# The display is supervised: if it dies, viewer captures degrade to 502s
# until the pod restarts, so a small watchdog restarts it and the service
# keeps running.
#
# SIGNER_VIEWER_MODE=signature skips the display entirely.

set -e

if [ "${SIGNER_VIEWER_MODE:-signature}" = "page" ]; then
  export DISPLAY="${SIGNER_DISPLAY:-:99}"

  start_display() {
    if [ -e /dev/dri/renderD128 ]; then
      echo '{"level":"info","message":"starting Xorg on node GPU"}'
      Xorg -noreset -logfile /tmp/xorg.log -config /dev/null \
        "${DISPLAY#:}" 2>/tmp/xorg-stderr.log &
    else
      echo '{"level":"info","message":"no /dev/dri; starting Xvfb (llvmpipe GL)"}'
      Xvfb "${DISPLAY}" -screen 0 1920x1080x24 2>/tmp/xvfb-stderr.log &
    fi
  }

  wait_display() {
    i=0
    while [ "$i" -lt 15 ]; do
      if xdpyinfo -display "${DISPLAY}" >/dev/null 2>&1; then
        return 0
      fi
      sleep 1
      i=$((i + 1))
    done
    return 1
  }

  start_display
  if ! wait_display; then
    echo '{"level":"error","message":"display did not come up within 15s; page mode will fail until restart"}'
  fi

  # Watchdog: Xvfb/Xorg has no supervisor in the pod, and a dead display
  # silently breaks every viewer capture. Restart it; the cost of a
  # momentary gap is one failed capture, not a wedged signer.
  (
    while true; do
      sleep 10
      if ! xdpyinfo -display "${DISPLAY}" >/dev/null 2>&1; then
        echo '{"level":"warn","message":"display died; restarting"}'
        start_display
        wait_display || true
      fi
    done
  ) &
fi

exec node dist/index.js
