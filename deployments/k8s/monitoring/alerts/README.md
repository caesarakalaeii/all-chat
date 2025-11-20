# All-Chat Alerting Configuration

This directory contains Prometheus alert rules and Alertmanager routing configuration for the All-Chat platform.

## Files

- **`allchat-critical-alerts.yaml`** - PrometheusRule with 4 critical alerts
- **`allchat-warning-alerts.yaml`** - PrometheusRule with 7 warning alerts
- **`alertmanager-config.yaml`** - Alertmanager routing configuration (template)

## Alert Rules (Applied ✅)

The alert rules are already applied to the cluster:

```bash
kubectl get prometheusrule -n monitoring | grep allchat
# allchat-critical-alerts
# allchat-warning-alerts
```

## Configure Alertmanager Routing (Manual Step Required)

Since you're using Prometheus Operator's Alertmanager (not Grafana Unified Alerting), you need to configure routing in Alertmanager.

### Option 1: Configure via Grafana Contact Points (Recommended)

Your Grafana has a Discord contact point configured. To use it:

1. The PrometheusRule alerts will fire to **Prometheus Alertmanager**
2. You need to configure **Grafana to receive alerts from Alertmanager**
3. Then Grafana will route them to Discord using your contact point

**Steps:**
1. In Grafana: **Alerting** → **Admin** → **Alertmanager**
2. Add Alertmanager data source pointing to: `http://kube-prometheus-kube-prome-alertmanager.monitoring.svc:9093`
3. Configure notification policies to route `team=allchat` alerts to "Salad Cluster Channel"

### Option 2: Configure Alertmanager Directly (Native Discord Integration)

Update the Alertmanager configuration to send directly to Discord:

#### Step 1: Get Discord Webhook URL

1. Open Discord Server Settings
2. Go to: **Integrations** → **Webhooks**
3. Find "Salad Cluster Channel" webhook
4. Copy the Webhook URL

#### Step 2: Create Secret with Discord Webhook

```bash
# Replace with your actual webhook URL
DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"

kubectl create secret generic allchat-discord-webhook \
  -n monitoring \
  --from-literal=url="$DISCORD_WEBHOOK_URL"
```

#### Step 3: Update alertmanager-config.yaml

Edit `alertmanager-config.yaml` and replace `YOUR_DISCORD_WEBHOOK_URL_HERE` with the webhook URL.

Also update `YOUR_EMAIL_HERE` with your actual email address for critical alerts.

#### Step 4: Apply Configuration

**WARNING**: This will replace the entire Alertmanager configuration. Make sure you've backed up the current config.

```bash
# Backup current config (already saved to /tmp/alertmanager-current.yaml)

# Apply new configuration
kubectl create secret generic alertmanager-kube-prometheus-kube-prome-alertmanager \
  -n monitoring \
  --from-file=alertmanager.yaml=deployments/k8s/monitoring/alerts/alertmanager-config.yaml \
  --dry-run=client -o yaml | kubectl apply -f -

# Reload Alertmanager (Prometheus Operator handles this automatically)
```

### Option 3: Use AlertmanagerConfig CRD (Declarative)

Create a separate AlertmanagerConfig that gets merged with the main config:

```bash
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1beta1
kind: AlertmanagerConfig
metadata:
  name: allchat-routing
  namespace: monitoring
  labels:
    alertmanagerConfig: allchat
spec:
  route:
    receiver: allchat-default
    matchers:
      - name: team
        value: allchat
    routes:
      - matchers:
          - name: severity
            value: critical
        receiver: allchat-critical
        groupBy: ['alertname', 'platform']
        groupWait: 10s
        groupInterval: 2m
        repeatInterval: 4h

      - matchers:
          - name: severity
            value: warning
        receiver: allchat-warnings
        groupBy: ['alertname']
        groupWait: 1m
        groupInterval: 10m
        repeatInterval: 12h

  receivers:
    - name: allchat-default
      discordConfigs:
        - webhookURL:
            name: allchat-discord-webhook
            key: url

    - name: allchat-critical
      discordConfigs:
        - webhookURL:
            name: allchat-discord-webhook
            key: url
          sendResolved: true

    - name: allchat-warnings
      discordConfigs:
        - webhookURL:
            name: allchat-discord-webhook
            key: url
          sendResolved: true
EOF
```

## Verification

### Check Alert Rules Loaded

```bash
# View in Prometheus UI
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-prometheus 9090:9090 &
# Open: http://localhost:9090/alerts
# Should see all AllChat* alerts
```

### Check Alertmanager Config

```bash
# View in Alertmanager UI
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-alertmanager 9093:9093 &
# Open: http://localhost:9093/#/status
# Check "Config" tab to verify routing
```

### Test Alert Firing

```bash
# Scale down a service to trigger ServiceDown alert
kubectl scale -n allchat deployment/emote-service --replicas=0

# Wait 2 minutes, then check:
# 1. Prometheus: http://localhost:9090/alerts (should show firing)
# 2. Alertmanager: http://localhost:9093/#/alerts (should show active)
# 3. Discord: Should receive notification in Salad Cluster Channel

# Restore service
kubectl scale -n allchat deployment/emote-service --replicas=2
```

## Current Status

- ✅ PrometheusRule resources created and applied
- ✅ Alert rules loaded into Prometheus
- ⏳ Alertmanager routing needs configuration (see options above)
- ⏳ Discord webhook URL needs to be added

## Recommended Next Steps

1. Choose Option 3 (AlertmanagerConfig CRD) for declarative configuration
2. Get Discord webhook URL from your "Salad Cluster Channel"
3. Create the secret with webhook URL
4. Apply the AlertmanagerConfig
5. Test with a service scale-down

This keeps configuration in Git and doesn't require manual UI changes.
