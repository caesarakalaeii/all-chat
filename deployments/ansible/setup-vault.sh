#!/bin/bash
#
# Ansible Vault Setup Helper Script
# This script helps you set up your encrypted secrets vault for All-Chat deployment
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="$SCRIPT_DIR/secrets.vault.yml.template"
VAULT_FILE="$SCRIPT_DIR/secrets.vault.yml"

echo "================================================"
echo "All-Chat Ansible Vault Setup"
echo "================================================"
echo ""

# Check if vault already exists
if [ -f "$VAULT_FILE" ]; then
    echo "⚠️  Warning: secrets.vault.yml already exists!"
    echo ""
    echo "Choose an option:"
    echo "  1) Edit existing vault"
    echo "  2) Create new vault (backup existing)"
    echo "  3) View existing vault"
    echo "  4) Exit"
    echo ""
    read -p "Enter choice [1-4]: " choice

    case $choice in
        1)
            echo "Opening vault for editing..."
            ansible-vault edit "$VAULT_FILE"
            exit 0
            ;;
        2)
            BACKUP_FILE="$VAULT_FILE.backup.$(date +%Y%m%d_%H%M%S)"
            echo "Backing up existing vault to: $BACKUP_FILE"
            mv "$VAULT_FILE" "$BACKUP_FILE"
            ;;
        3)
            echo "Viewing vault contents..."
            ansible-vault view "$VAULT_FILE"
            exit 0
            ;;
        4)
            echo "Exiting..."
            exit 0
            ;;
        *)
            echo "Invalid choice. Exiting."
            exit 1
            ;;
    esac
fi

# Check if template exists
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "❌ Error: Template file not found: $TEMPLATE_FILE"
    exit 1
fi

echo "✓ Found template file"
echo ""

# Step 1: Copy template
echo "Step 1: Copying template to secrets.vault.yml..."
cp "$TEMPLATE_FILE" "$VAULT_FILE"
echo "✓ Template copied"
echo ""

# Step 2: Prompt for interactive editing
echo "Step 2: Edit secrets file"
echo ""
echo "The vault file needs to be filled with your actual credentials."
echo "All values marked with 'CHANGE_ME_' must be replaced."
echo ""
echo "Required credentials:"
echo "  • Twitch Client ID & Secret (from https://dev.twitch.tv/console/apps)"
echo "  • Twitch Bot Username & OAuth (from https://twitchapps.com/tmi/)"
echo "  • YouTube Client ID & Secret (from https://console.developers.google.com/)"
echo "  • PostgreSQL password (use a strong password)"
echo "  • JWT secret (generate with: openssl rand -base64 32)"
echo ""
read -p "Open editor now? (y/n): " open_editor

if [[ "$open_editor" =~ ^[Yy]$ ]]; then
    # Detect editor
    if [ -n "$EDITOR" ]; then
        EDITOR_CMD="$EDITOR"
    elif command -v nano &> /dev/null; then
        EDITOR_CMD="nano"
    elif command -v vim &> /dev/null; then
        EDITOR_CMD="vim"
    elif command -v vi &> /dev/null; then
        EDITOR_CMD="vi"
    else
        echo "❌ No editor found. Please edit $VAULT_FILE manually."
        exit 1
    fi

    echo "Opening $VAULT_FILE in $EDITOR_CMD..."
    echo ""
    echo "📝 Remember to replace ALL 'CHANGE_ME_' values!"
    echo ""
    read -p "Press Enter to continue..."

    $EDITOR_CMD "$VAULT_FILE"
else
    echo "⚠️  You'll need to edit $VAULT_FILE manually before encrypting."
    echo "Run: $EDITOR_CMD $VAULT_FILE"
    echo ""
    read -p "Press Enter when you've finished editing..."
fi

# Step 3: Validate that user changed the values
echo ""
echo "Step 3: Validating secrets file..."

if grep -q "CHANGE_ME_" "$VAULT_FILE"; then
    echo "⚠️  Warning: Found unchanged 'CHANGE_ME_' values in vault file!"
    echo ""
    echo "The following values still need to be changed:"
    grep "CHANGE_ME_" "$VAULT_FILE" | sed 's/^/  • /'
    echo ""
    read -p "Continue anyway? (y/n): " continue_anyway
    if [[ ! "$continue_anyway" =~ ^[Yy]$ ]]; then
        echo "Please edit the file and run this script again."
        exit 1
    fi
fi

# Step 4: Encrypt the vault
echo ""
echo "Step 4: Encrypt the vault"
echo ""
echo "You'll need to create a vault password. This password will be required"
echo "every time you run the Ansible playbook or edit the vault."
echo ""
echo "💡 Tips for vault passwords:"
echo "  • Use at least 20 characters"
echo "  • Use a mix of letters, numbers, and symbols"
echo "  • Store it securely (password manager)"
echo "  • Never commit it to git"
echo ""
read -p "Ready to encrypt? (y/n): " ready_to_encrypt

if [[ "$ready_to_encrypt" =~ ^[Yy]$ ]]; then
    echo ""
    echo "Encrypting $VAULT_FILE..."
    ansible-vault encrypt "$VAULT_FILE"

    if [ $? -eq 0 ]; then
        echo ""
        echo "✅ Vault encrypted successfully!"
        echo ""
        echo "================================================"
        echo "Setup Complete!"
        echo "================================================"
        echo ""
        echo "Next steps:"
        echo ""
        echo "1. Run the Ansible playbook:"
        echo "   ansible-playbook -i inventory.yml playbook.yml --ask-vault-pass"
        echo ""
        echo "2. To edit the vault later:"
        echo "   ansible-vault edit secrets.vault.yml"
        echo ""
        echo "3. To view the vault:"
        echo "   ansible-vault view secrets.vault.yml"
        echo ""
        echo "4. For more info:"
        echo "   cat README_VAULT.md"
        echo ""
    else
        echo "❌ Encryption failed. Please try again."
        exit 1
    fi
else
    echo ""
    echo "⚠️  Vault not encrypted. To encrypt later, run:"
    echo "   ansible-vault encrypt secrets.vault.yml"
    exit 0
fi
