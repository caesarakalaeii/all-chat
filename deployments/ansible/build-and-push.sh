#!/bin/bash
set -e

# Build and push All-Chat services to local k3d registry
# Registry: localhost:5000

REGISTRY="localhost:5000"
TAG="latest"
ROOT_DIR="$(cd ../.. && pwd)"

echo "========================================="
echo "Building All-Chat Docker Images"
echo "========================================="
echo "Registry: $REGISTRY"
echo "Tag: $TAG"
echo "Root: $ROOT_DIR"
echo ""

cd "$ROOT_DIR"

# Array of services to build
services=(
  "auth-service"
  "overlay-manager"
  "emote-service"
  "api-gateway"
  "twitch-listener"
  "youtube-listener"
  "message-processor"
  "source-manager"
)

# Build and push each service
for service in "${services[@]}"; do
  echo "========================================="
  echo "Building: $service"
  echo "========================================="

  IMAGE_NAME="allchat-$service"
  FULL_IMAGE="$REGISTRY/$IMAGE_NAME:$TAG"

  # Build
  docker build \
    -f "services/$service/Dockerfile" \
    -t "$IMAGE_NAME:$TAG" \
    -t "$FULL_IMAGE" \
    .

  # Push to registry
  echo "Pushing $FULL_IMAGE..."
  docker push "$FULL_IMAGE"

  echo "✅ $service complete"
  echo ""
done

echo "========================================="
echo "All images built and pushed successfully!"
echo "========================================="
echo ""
echo "To verify:"
echo "  curl http://localhost:5000/v2/_catalog"
echo ""
echo "To deploy:"
echo "  kubectl apply -f deployments/k8s/base/ -n allchat --recursive"
