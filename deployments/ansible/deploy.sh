#!/bin/bash
#
# All-Chat Kubernetes Deployment Script
# Automatically uses vault password from ~/.ssh/ansible_vault_pass
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAULT_PASS_FILE="${HOME}/.ssh/ansible_vault_pass"
VAULT_FILE="$SCRIPT_DIR/secrets.vault.yml"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

echo "================================================"
echo "All-Chat Kubernetes Deployment"
echo "================================================"
echo ""

# Check for vault password file
if [ ! -f "$VAULT_PASS_FILE" ]; then
    echo "❌ Error: Vault password file not found at: $VAULT_PASS_FILE"
    echo ""
    echo "Please create this file with your vault password:"
    echo "  echo 'your-vault-password' > $VAULT_PASS_FILE"
    echo "  chmod 600 $VAULT_PASS_FILE"
    exit 1
fi

echo "✓ Found vault password file"

# Check if vault exists
if [ ! -f "$VAULT_FILE" ]; then
    echo "⚠️  Vault file not found: $VAULT_FILE"
    echo ""
    read -p "Generate vault from .env file? (y/n): " generate

    if [[ "$generate" =~ ^[Yy]$ ]]; then
        echo ""
        echo "Running vault generator..."
        "$SCRIPT_DIR/generate-vault-from-env.sh"
        echo ""
    else
        echo "❌ Cannot deploy without vault file."
        echo ""
        echo "Options:"
        echo "  1. Generate from .env: $SCRIPT_DIR/generate-vault-from-env.sh"
        echo "  2. Create manually from template: cp secrets.vault.yml.template secrets.vault.yml"
        exit 1
    fi
fi

echo "✓ Found vault file"
echo ""

# Check if prerequisites are installed
echo "Checking prerequisites..."

if ! command -v ansible-playbook &> /dev/null; then
    echo "❌ ansible-playbook not found. Please install Ansible."
    exit 1
fi
echo "✓ Ansible installed"

if ! command -v kubectl &> /dev/null; then
    echo "⚠️  kubectl not found (will be installed by playbook)"
fi

if ! command -v k3d &> /dev/null; then
    echo "⚠️  k3d not found (will be installed by playbook)"
fi

echo ""
echo "================================================"
echo "Starting Deployment"
echo "================================================"
echo ""
echo "This will:"
echo "  • Create/update k3d cluster 'allchat'"
echo "  • Deploy PostgreSQL and Redis"
echo "  • Deploy all 8 microservices"
echo "  • Set up networking and ingress"
echo ""
echo "Using vault password from: $VAULT_PASS_FILE"
echo ""
read -p "Continue? (y/n): " continue_deploy

if [[ ! "$continue_deploy" =~ ^[Yy]$ ]]; then
    echo "Deployment cancelled."
    exit 0
fi

echo ""
echo "Running Ansible playbook..."
echo ""

# Run the playbook
ansible-playbook \
    -i inventory.yml \
    playbook.yml \
    --vault-password-file "$VAULT_PASS_FILE" \
    "$@"

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo "================================================"
    echo "✅ Deployment Complete!"
    echo "================================================"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Check cluster status:"
    echo "   kubectl get nodes"
    echo "   kubectl get pods -n allchat"
    echo "   kubectl get services -n allchat"
    echo ""
    echo "2. Set up port forwarding:"
    echo "   ./port-forward.sh"
    echo ""
    echo "3. Or manually forward API Gateway:"
    echo "   kubectl port-forward -n allchat svc/api-gateway 8080:8080"
    echo ""
    echo "4. Access services:"
    echo "   API Gateway: http://localhost:8080"
    echo "   Frontend: http://localhost:3000 (if deployed)"
    echo ""
    echo "5. View logs:"
    echo "   kubectl logs -n allchat -l app=api-gateway --tail=100 -f"
    echo ""
    echo "6. Delete cluster:"
    echo "   k3d cluster delete allchat"
    echo ""
else
    echo ""
    echo "❌ Deployment failed with exit code: $EXIT_CODE"
    echo ""
    echo "Check the output above for errors."
    echo ""
    echo "Common issues:"
    echo "  • Wrong vault password: Check $VAULT_PASS_FILE"
    echo "  • Missing credentials: Update .env and regenerate vault"
    echo "  • Docker not running: Start Docker service"
    echo "  • Port conflicts: Check if ports 8080, 6443 are in use"
    echo ""
    echo "To view full logs:"
    echo "  cat /tmp/ansible-deploy.log"
    echo ""
    exit $EXIT_CODE
fi
