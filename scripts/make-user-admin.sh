#!/bin/bash
# Script to grant admin privileges to a user
# Usage: ./scripts/make-user-admin.sh <username>
# Or with kubectl: kubectl exec -n allchat <postgres-pod> -- psql -U allchat -d allchat -c "UPDATE users SET is_admin = TRUE WHERE username = '<username>';"

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <username>"
    echo "Example: $0 caesarlp"
    exit 1
fi

USERNAME=$1

# Database connection details from environment or defaults
DB_HOST=${DATABASE_HOST:-localhost}
DB_PORT=${DATABASE_PORT:-5432}
DB_USER=${DATABASE_USER:-allchat}
DB_PASSWORD=${DATABASE_PASSWORD:-allchat_dev_password}
DB_NAME=${DATABASE_NAME:-allchat}

echo "Making user '$USERNAME' an admin..."

# Execute SQL update
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << EOF
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
