#!/bin/bash
# Script to grant admin privileges to a user in Kubernetes
# Usage: ./scripts/k8s-make-user-admin.sh <username>

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <username>"
    echo "Example: $0 caesarlp"
    exit 1
fi

USERNAME=$1
NAMESPACE=${NAMESPACE:-allchat}

echo "Making user '$USERNAME' an admin in Kubernetes..."

# Find the first postgres pod
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POSTGRES_POD" ]; then
    # Try cluster pods
    POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l cnpg.io/cluster=allchat-cluster -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
fi

if [ -z "$POSTGRES_POD" ]; then
    echo "Error: Could not find PostgreSQL pod in namespace '$NAMESPACE'"
    echo "Available pods:"
    kubectl get pods -n $NAMESPACE
    exit 1
fi

echo "Using PostgreSQL pod: $POSTGRES_POD"

# Execute SQL update
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d allchat << EOF
-- Update user to admin
UPDATE users SET is_admin = TRUE WHERE username = '$USERNAME';

-- Verify the update
SELECT id, username, display_name, auth_provider, is_admin
FROM users
WHERE username = '$USERNAME';
EOF

echo ""
echo "Done! User '$USERNAME' is now an admin."
echo "Note: The user will need to log out and log back in for the changes to take effect."
