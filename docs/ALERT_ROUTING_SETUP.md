# All-Chat Alert Routing Configuration Guide

**Status**: ✅ Alert Rules Deployed
**Date**: 2025-11-20

---

## Alert Rules Deployed

### Critical Alerts (4)
- ✅ `AllChatServiceDown` - Service unavailable >2min
- ✅ `YouTubeQuotaCritical` - Quota >90% (URGENT!)
- ✅ `AllChatListenerDisconnected` - Platform offline >2min
- ✅ `AllChatHighMessageDropRate` - >5% message loss

### Warning Alerts (7)
- ✅ `YouTubeQuotaWarning` - Quota >80%
- ✅ `AllChatHighProcessingLatency` - p95 >500ms
- ✅ `AllChatMessageBacklog` - >5000 pending messages
- ✅ `AllChatListenerReconnecting` - Frequent reconnects
- ✅ `AllChatHighStreamErrors` - Redis errors
- ✅ `AllChatPodCrashLoop` - Pods restarting
- ✅ `AllChatLowEmoteCacheHitRate` - Cache <80%

---

## Contact Points Available

- 💬 **Discord**: "Salad Cluster Channel" (uid: `bf4pu1h7x0t1cb`)
- 📧 **Email**: "email receiver"

---

## Configure Notification Policies in Grafana

Since you're using **Grafana Unified Alerting** (not native Prometheus Alertmanager), configure routing through Grafana UI:

### Step 1: Access Grafana Alerting

1. Open Grafana: http://localhost:3000 (or your Grafana URL)
2. Navigate to: **Alerting** → **Notification policies**

### Step 2: Create Routing Policy for Critical Alerts

Click **"New nested policy"** and configure:

**Match labels:**
```
severity = critical
team = allchat
```

**Contact point:**
```
Salad Cluster Channel (Discord)
```

**Additional settings:**
- Group by: `alertname, severity`
- Group wait: `30s`
- Group interval: `5m`
- Repeat interval: `4h`

**Continue matching subsequent sibling nodes**: ✅ (to also send to email)

### Step 3: Add Email Notification for Critical

Create another nested policy under the root:

**Match labels:**
```
severity = critical
team = allchat
```

**Contact point:**
```
email receiver
```

**Additional settings:**
- Group by: `alertname, severity`
- Group wait: `30s`
- Group interval: `5m`
- Repeat interval: `12h`

### Step 4: Create Routing Policy for Warning Alerts

Click **"New nested policy"**:

**Match labels:**
```
severity = warning
team = allchat
```

**Contact point:**
```
Salad Cluster Channel (Discord)
```

**Additional settings:**
- Group by: `alertname`
- Group wait: `1m`
- Group interval: `10m`
- Repeat interval: `12h`

### Step 5: Save Configuration

Click **"Save policy"** at the top.

---

## Alternative: Configure via AlertmanagerConfig CRD

If you prefer declarative configuration, create an AlertmanagerConfig resource:

```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: allchat-routing
  namespace: monitoring
spec:
  route:
    receiver: 'null'
    routes:
      # Critical alerts to Discord + Email
      - match:
          severity: critical
          team: allchat
        receiver: allchat-critical
        groupBy: ['alertname', 'severity']
        groupWait: 30s
        groupInterval: 5m
        repeatInterval: 4h

      # Warning alerts to Discord only
      - match:
          severity: warning
          team: allchat
        receiver: allchat-warnings
        groupBy: ['alertname']
        groupWait: 1m
        groupInterval: 10m
        repeatInterval: 12h

  receivers:
    - name: 'null'

    - name: allchat-critical
      discordConfigs:
        - sendResolved: true
          webhookUrl:
            name: discord-webhook-secret
            key: salad-cluster-url
      emailConfigs:
        - sendResolved: true
          to: 'your-team@example.com'

    - name: allchat-warnings
      discordConfigs:
        - sendResolved: true
          webhookUrl:
            name: discord-webhook-secret
            key: salad-cluster-url
```

**Note**: This requires the Discord webhook URL to be stored in a Secret.

---

## Test Alert Firing

### Test 1: Service Down Alert

```bash
# Scale down a service to trigger alert
kubectl scale -n allchat deployment/emote-service --replicas=0

# Wait 2 minutes for alert to fire
# Check alert in Grafana: Alerting → Alert rules

# Restore service
kubectl scale -n allchat deployment/emote-service --replicas=2
```

### Test 2: View Active Alerts

```bash
# Check Prometheus alerts
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-prometheus 9090:9090 &
# Open: http://localhost:9090/alerts

# Check Alertmanager
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-alertmanager 9093:9093 &
# Open: http://localhost:9093
```

### Test 3: Check Discord Notifications

Once an alert fires, you should receive a Discord message in your "Salad Cluster Channel" with:
- Alert name and severity
- Summary and description
- Labels (platform, team, severity)
- Runbook actions

---

## Alert Summary by Severity

### Critical (Page Immediately)
| Alert | Threshold | Duration | Impact |
|-------|-----------|----------|--------|
| ServiceDown | up=0 | 2m | Platform unavailable |
| YouTubeQuotaCritical | >90% quota | 1m | API shutdown risk |
| ListenerDisconnected | status=0 | 2m | Platform offline |
| HighMessageDropRate | >5% loss | 5m | Data loss |

### Warning (Investigate Soon)
| Alert | Threshold | Duration | Impact |
|-------|-----------|----------|--------|
| YouTubeQuotaWarning | >80% quota | 5m | Approaching limit |
| HighProcessingLatency | p95 >500ms | 10m | Slow delivery |
| MessageBacklog | >5000 msgs | 5m | Delays building |
| ListenerReconnecting | >0.1 fails/s | 10m | Unstable connection |
| HighStreamErrors | >1 error/s | 5m | Processing issues |
| PodCrashLoop | Restarts >0 | 5m | Service instability |
| LowEmoteCacheHitRate | <80% | 15m | Increased latency |

---

## Alert Routing Decision Tree

```
Alert Fires
    ↓
Is severity=critical?
    ├─ YES → Send to Discord + Email (repeat every 4h)
    └─ NO  → Send to Discord only (repeat every 12h)
```

---

## Monitoring Alert Health

**View alert rules in Grafana:**
- Navigate to: **Alerting** → **Alert rules**
- Filter by: `team=allchat`

**View active alerts:**
- **Alerting** → **Alert groups**

**View notification history:**
- **Alerting** → **Contact points** → Select contact point → **View notification history**

---

## Next Steps

1. **Configure notification policies** in Grafana UI (see Step 2-4 above)
2. **Test an alert** by scaling down a service
3. **Verify Discord notifications** are received
4. **Fine-tune alert thresholds** based on real traffic patterns
5. **Create runbook wiki** with detailed remediation steps

---

## Files

**Alert Manifests:**
- `/deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml`
- `/deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml`

**Applied to cluster:**
```bash
kubectl get prometheusrule -n monitoring | grep allchat
```

---

**Status**: 🚀 Alert Rules Deployed and Ready!

Configure routing policies in Grafana UI to start receiving notifications.
