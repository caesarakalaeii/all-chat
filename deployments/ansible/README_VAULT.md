# Ansible Vault Setup Guide

This guide explains how to set up and use Ansible Vault for managing secrets in the All-Chat deployment.

## What is Ansible Vault?

Ansible Vault is an encrypted storage system for sensitive data like passwords, API keys, and tokens. It keeps your secrets secure in version control.

## Quick Setup

### 1. Create Your Secrets File

```bash
# Copy the template
cd deployments/ansible
cp secrets.vault.yml.template secrets.vault.yml
```

### 2. Fill In Your Values

Edit `secrets.vault.yml` and replace all `CHANGE_ME_` values with your actual credentials:

```bash
# Use your favorite editor
nano secrets.vault.yml
# or
vim secrets.vault.yml
```

**Required Credentials:**

- **Twitch**: Get from https://dev.twitch.tv/console/apps
  - Client ID & Secret for OAuth
  - Bot username & OAuth token from https://twitchapps.com/tmi/

- **YouTube**: Get from https://console.developers.google.com/
  - Client ID & Secret for OAuth
  - API Key (optional)

- **Database**: Use a strong password for PostgreSQL

- **JWT Secret**: Generate with `openssl rand -base64 32`

### 3. Encrypt the Vault

```bash
# Encrypt the secrets file
ansible-vault encrypt secrets.vault.yml

# You'll be prompted for a vault password - REMEMBER THIS!
New Vault password:
Confirm New Vault password:
Encryption successful
```

### 4. Update the Playbook

The playbook has been updated to load variables from the vault automatically:

```yaml
# In playbook.yml
vars_files:
  - secrets.vault.yml
```

### 5. Run the Playbook

```bash
# Run with vault password
ansible-playbook -i inventory.yml playbook.yml --ask-vault-pass

# Or use a password file
echo "your-vault-password" > .vault_pass
chmod 600 .vault_pass
ansible-playbook -i inventory.yml playbook.yml --vault-password-file .vault_pass
```

## Common Operations

### View Encrypted Vault

```bash
ansible-vault view secrets.vault.yml
```

### Edit Encrypted Vault

```bash
ansible-vault edit secrets.vault.yml
```

### Change Vault Password

```bash
ansible-vault rekey secrets.vault.yml
```

### Decrypt Vault (NOT RECOMMENDED for production)

```bash
ansible-vault decrypt secrets.vault.yml
```

### Encrypt Again

```bash
ansible-vault encrypt secrets.vault.yml
```

## Using Password Files

For automation or convenience, you can store the vault password in a file:

```bash
# Create password file
echo "your-strong-vault-password" > .vault_pass

# Secure the file (important!)
chmod 600 .vault_pass

# Add to .gitignore (already done)
echo ".vault_pass" >> ../../.gitignore

# Use with playbook
ansible-playbook -i inventory.yml playbook.yml --vault-password-file .vault_pass
```

## Security Best Practices

1. **Never commit unencrypted secrets** - Always encrypt before committing
2. **Use strong vault passwords** - Minimum 20 characters, random
3. **Don't commit password files** - Keep `.vault_pass` local only
4. **Rotate credentials regularly** - Change secrets periodically
5. **Use different secrets per environment** - Dev, staging, production
6. **Secure your workstation** - Vault password is only as secure as your machine

## Environment-Specific Vaults

For different environments, create separate vault files:

```bash
# Development
secrets.vault.dev.yml

# Staging
secrets.vault.staging.yml

# Production
secrets.vault.prod.yml
```

Then specify which to use:

```bash
ansible-playbook -i inventory.yml playbook.yml \
  -e @secrets.vault.prod.yml \
  --ask-vault-pass
```

## Troubleshooting

### "Decryption failed"

- You entered the wrong vault password
- The vault file is corrupted
- The vault was encrypted with a different password

### "Vault format unhexlify error"

- The vault file is not properly encrypted
- Try: `ansible-vault encrypt secrets.vault.yml`

### Variables not found in playbook

- Make sure `vars_files` points to your vault file
- Check that variable names match between vault and playbook
- Verify the vault is actually being loaded

## Integration with CI/CD

For GitHub Actions or other CI/CD:

```yaml
# In GitHub Actions
- name: Run Ansible Playbook
  env:
    VAULT_PASSWORD: ${{ secrets.ANSIBLE_VAULT_PASSWORD }}
  run: |
    echo "$VAULT_PASSWORD" > .vault_pass
    ansible-playbook -i inventory.yml playbook.yml --vault-password-file .vault_pass
    rm .vault_pass
```

Store `ANSIBLE_VAULT_PASSWORD` as a GitHub Secret.

## Example: Complete Workflow

```bash
# 1. Copy template
cp secrets.vault.yml.template secrets.vault.yml

# 2. Fill in real values
nano secrets.vault.yml

# 3. Encrypt
ansible-vault encrypt secrets.vault.yml

# 4. Run playbook
ansible-playbook -i inventory.yml playbook.yml --ask-vault-pass

# 5. Later: Edit secrets
ansible-vault edit secrets.vault.yml

# 6. Run playbook again with updated secrets
ansible-playbook -i inventory.yml playbook.yml --ask-vault-pass
```

## Getting Credentials

### Twitch

1. Go to https://dev.twitch.tv/console/apps
2. Click "Register Your Application"
3. Name: "All-Chat"
4. OAuth Redirect URLs: `http://localhost:8080/api/v1/auth/twitch/callback`
5. Category: "Application Integration"
6. Save Client ID and Secret

For bot OAuth token:
1. Go to https://twitchapps.com/tmi/
2. Connect with your bot account
3. Copy the OAuth token (starts with `oauth:`)

### YouTube

1. Go to https://console.developers.google.com/
2. Create a new project or select existing
3. Enable "YouTube Data API v3"
4. Create OAuth 2.0 credentials
5. Authorized redirect URIs: `http://localhost:8080/api/v1/auth/youtube/callback`
6. Save Client ID and Secret

### Generate JWT Secret

```bash
# Strong random secret (32 bytes base64)
openssl rand -base64 32

# Or use Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"
```

## Support

For issues with Ansible Vault:
- [Official Ansible Vault Docs](https://docs.ansible.com/ansible/latest/user_guide/vault.html)
- Project Issues: https://github.com/your-repo/issues

---

**Remember**: The security of your deployment depends on keeping these secrets safe!
