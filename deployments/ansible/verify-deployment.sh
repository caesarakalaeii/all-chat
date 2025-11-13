#!/bin/bash
set -e

# Verification script for All-Chat Kubernetes deployment
# Tests health checks, connectivity, and basic functionality

NAMESPACE="allchat"
FAILED=0

echo "========================================="
echo "All-Chat Deployment Verification"
echo "========================================="
echo "Namespace: $NAMESPACE"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check_command() {
  local cmd=$1
  local name=$2

  if command -v "$cmd" &> /dev/null; then
    echo -e "${GREEN}✓${NC} $name is installed"
  else
    echo -e "${RED}✗${NC} $name is NOT installed"
    FAILED=1
  fi
}

check_pod() {
  local app=$1
  local name=$2

  if kubectl get pod -n "$NAMESPACE" -l "app=$app" &> /dev/null; then
    local status=$(kubectl get pod -n "$NAMESPACE" -l "app=$app" -o jsonpath='{.items[0].status.phase}')
    if [ "$status" == "Running" ]; then
      echo -e "${GREEN}✓${NC} $name is running"
    else
      echo -e "${YELLOW}⚠${NC} $name status: $status"
      FAILED=1
    fi
  else
    echo -e "${RED}✗${NC} $name pod not found"
    FAILED=1
  fi
}

check_service() {
  local svc=$1
  local name=$2

  if kubectl get service -n "$NAMESPACE" "$svc" &> /dev/null; then
    echo -e "${GREEN}✓${NC} $name service exists"
  else
    echo -e "${RED}✗${NC} $name service not found"
    FAILED=1
  fi
}

# Check prerequisites
echo "Checking prerequisites..."
check_command "kubectl" "kubectl"
check_command "docker" "docker"
check_command "k3d" "k3d"
echo ""

# Check cluster exists
echo "Checking k3d cluster..."
if k3d cluster list | grep -q "allchat"; then
  echo -e "${GREEN}✓${NC} k3d cluster 'allchat' exists"
else
  echo -e "${RED}✗${NC} k3d cluster 'allchat' not found"
  echo "Run: ansible-playbook -i inventory.yml playbook.yml"
  exit 1
fi
echo ""

# Check namespace
echo "Checking namespace..."
if kubectl get namespace "$NAMESPACE" &> /dev/null; then
  echo -e "${GREEN}✓${NC} Namespace '$NAMESPACE' exists"
else
  echo -e "${RED}✗${NC} Namespace '$NAMESPACE' not found"
  FAILED=1
fi
echo ""

# Check ConfigMap and Secret
echo "Checking configuration..."
if kubectl get configmap allchat-config -n "$NAMESPACE" &> /dev/null; then
  echo -e "${GREEN}✓${NC} ConfigMap 'allchat-config' exists"
else
  echo -e "${RED}✗${NC} ConfigMap not found"
  FAILED=1
fi

if kubectl get secret allchat-secrets -n "$NAMESPACE" &> /dev/null; then
  echo -e "${GREEN}✓${NC} Secret 'allchat-secrets' exists"
else
  echo -e "${RED}✗${NC} Secret not found"
  FAILED=1
fi
echo ""

# Check infrastructure pods
echo "Checking infrastructure..."
check_pod "postgres" "PostgreSQL"
check_pod "redis" "Redis"
echo ""

# Check Phase 1-3 services
echo "Checking Phase 1-3 services..."
check_pod "auth-service" "Auth Service"
check_pod "overlay-manager" "Overlay Manager"
check_pod "emote-service" "Emote Service"
check_pod "api-gateway" "API Gateway"
check_pod "twitch-listener" "Twitch Listener"
check_pod "message-processor" "Message Processor"
echo ""

# Check Phase 4 services
echo "Checking Phase 4 services..."
check_pod "youtube-listener" "YouTube Listener"
check_pod "source-manager" "Source Manager"
echo ""

# Check services
echo "Checking Kubernetes services..."
check_service "postgres" "PostgreSQL"
check_service "redis" "Redis"
check_service "api-gateway" "API Gateway"
check_service "youtube-listener" "YouTube Listener"
check_service "source-manager" "Source Manager"
echo ""

# Check deployments are ready
echo "Checking deployment readiness..."
READY_DEPLOYMENTS=$(kubectl get deployments -n "$NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"="}{.status.readyReplicas}{"/"}{.spec.replicas}{"\n"}{end}')

echo "$READY_DEPLOYMENTS" | while IFS= read -r line; do
  if [ -n "$line" ]; then
    NAME=$(echo "$line" | cut -d'=' -f1)
    STATUS=$(echo "$line" | cut -d'=' -f2)
    READY=$(echo "$STATUS" | cut -d'/' -f1)
    DESIRED=$(echo "$STATUS" | cut -d'/' -f2)

    if [ "$READY" == "$DESIRED" ] && [ "$READY" != "" ]; then
      echo -e "${GREEN}✓${NC} $NAME: $STATUS ready"
    else
      echo -e "${YELLOW}⚠${NC} $NAME: $STATUS (waiting...)"
    fi
  fi
done
echo ""

# Test health endpoints (requires port-forward)
echo "========================================="
echo "Health Check Tests"
echo "========================================="
echo "Note: Run './port-forward.sh' in another terminal first"
echo ""

# Check if port-forward is active
if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "Port 8080 is forwarded, testing endpoints..."

  # Test health endpoints
  for port in 8080 8086 8088; do
    if curl -s -f "http://localhost:$port/health/live" > /dev/null 2>&1; then
      echo -e "${GREEN}✓${NC} localhost:$port/health/live is responding"
    else
      echo -e "${YELLOW}⚠${NC} localhost:$port/health/live not accessible (port-forward needed)"
    fi
  done
else
  echo -e "${YELLOW}⚠${NC} No port-forward detected. Run './port-forward.sh' to test endpoints"
fi
echo ""

# Summary
echo "========================================="
echo "Verification Summary"
echo "========================================="
if [ $FAILED -eq 0 ]; then
  echo -e "${GREEN}✓ All checks passed!${NC}"
  echo ""
  echo "Next steps:"
  echo "  1. Port forward: ./port-forward.sh"
  echo "  2. Test health: curl http://localhost:8080/health"
  echo "  3. Check status: curl http://localhost:8086/status | jq"
  echo "  4. View logs: kubectl logs -n allchat -l app=youtube-listener --tail=50"
else
  echo -e "${RED}✗ Some checks failed${NC}"
  echo ""
  echo "Debugging commands:"
  echo "  kubectl get pods -n $NAMESPACE"
  echo "  kubectl describe pod -n $NAMESPACE <pod-name>"
  echo "  kubectl logs -n $NAMESPACE <pod-name>"
  echo "  kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
  exit 1
fi
