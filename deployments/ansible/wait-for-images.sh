#!/bin/bash
#
# Wait for GitHub Actions to build all images and make them available
#

set -e

REGISTRY="ghcr.io"
IMAGE_PREFIX="caesarakalaeii/allchat"
TAG="main"

SERVICES=(
  "auth-service"
  "overlay-manager"
  "emote-service"
  "api-gateway"
  "twitch-listener"
  "youtube-listener"
  "message-processor"
  "source-manager"
)

echo "================================================"
echo "Waiting for GitHub Container Registry Images"
echo "================================================"
echo ""
echo "This script checks if all service images are available"
echo "on GitHub Container Registry."
echo ""
echo "Images will be built by GitHub Actions after pushing to main."
echo "Typical build time: 5-10 minutes for all services."
echo ""
echo "Check build status at:"
echo "  https://github.com/caesarakalaeii/all-chat/actions"
echo ""

check_image() {
  local service=$1
  local image="$REGISTRY/$IMAGE_PREFIX-$service:$TAG"

  # Try to pull image manifest (doesn't download the image)
  if docker manifest inspect "$image" &> /dev/null; then
    return 0
  else
    return 1
  fi
}

echo "Checking for images..."
echo ""

MAX_WAIT=600  # 10 minutes
ELAPSED=0
CHECK_INTERVAL=30

while [ $ELAPSED -lt $MAX_WAIT ]; do
  AVAILABLE=0
  MISSING=()

  for service in "${SERVICES[@]}"; do
    if check_image "$service"; then
      echo "✅ $service"
      ((AVAILABLE++))
    else
      echo "⏳ $service (not ready)"
      MISSING+=("$service")
    fi
  done

  echo ""
  echo "Available: $AVAILABLE / ${#SERVICES[@]}"

  if [ $AVAILABLE -eq ${#SERVICES[@]} ]; then
    echo ""
    echo "================================================"
    echo "✅ All images are available!"
    echo "================================================"
    echo ""
    echo "You can now deploy to Kubernetes:"
    echo "  kubectl apply -k ../k8s/base/"
    echo ""
    echo "Or restart the deployments:"
    echo "  kubectl rollout restart deployment --all -n allchat"
    echo ""
    exit 0
  fi

  echo "Waiting $CHECK_INTERVAL seconds before next check..."
  echo "Elapsed: ${ELAPSED}s / ${MAX_WAIT}s"
  echo ""

  sleep $CHECK_INTERVAL
  ((ELAPSED += CHECK_INTERVAL))
done

echo "================================================"
echo "⚠️  Timeout: Not all images are ready yet"
echo "================================================"
echo ""
echo "Still missing:"
for service in "${MISSING[@]}"; do
  echo "  • $service"
done
echo ""
echo "Check GitHub Actions for build status:"
echo "  https://github.com/caesarakalaeii/all-chat/actions"
echo ""
echo "Once builds complete, run:"
echo "  kubectl rollout restart deployment --all -n allchat"
echo ""
exit 1
