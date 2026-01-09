# Discord Bot Examples

Visual representation of Discord embeds the bot will post.

## 1. Quota Status Embed (Healthy State)

```
╔═══════════════════════════════════════════════════════╗
║ 📊 YouTube API Quota Status                          ║
╠═══════════════════════════════════════════════════════╣
║ Current State: 🟢 HEALTHY                            ║
║                                                       ║
║ 📈 Usage                                             ║
║ 2,450 / 10,000 units (24.50%)                        ║
║                                                       ║
║ Progress                                              ║
║ [████░░░░░░░░░░░░░░░░] 24.5%                        ║
║                                                       ║
║ ⏱️ Remaining        🔄 Resets        ⚡ Polling Speed║
║ 7,550 units        in 8 hours       1.00x            ║
║                                                       ║
║ 🔝 Top Consuming Channels                            ║
║ 1. UCxxxxx... - 450 units (18.4%)                    ║
║ 2. UCyyyyy... - 380 units (15.5%)                    ║
║ 3. UCzzzzz... - 290 units (11.8%)                    ║
║ 4. UCwwwww... - 210 units (8.6%)                     ║
║ 5. UCvvvvv... - 180 units (7.3%)                     ║
╠═══════════════════════════════════════════════════════╣
║ All-Chat Quota Monitor • Jan 9, 2025 12:00 PM       ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🟢 Green

---

## 2. Degraded State Alert (Warning)

```
╔═══════════════════════════════════════════════════════╗
║ 🔄 Quota Alert: State Changed                        ║
╠═══════════════════════════════════════════════════════╣
║ Quota state changed from HEALTHY to DEGRADED         ║
║ (72.50% used)                                         ║
║                                                       ║
║ State              Usage           Remaining          ║
║ 🟡 DEGRADED       72.50%          2,750 units        ║
╠═══════════════════════════════════════════════════════╣
║ Severity: WARNING • Jan 9, 2025 2:30 PM             ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🟡 Yellow

---

## 3. Critical State Alert (Error)

```
╔═══════════════════════════════════════════════════════╗
║ 🔄 Quota Alert: State Changed                        ║
╠═══════════════════════════════════════════════════════╣
║ Quota state changed from DEGRADED to CRITICAL        ║
║ (87.50% used)                                         ║
║                                                       ║
║ State              Usage           Remaining          ║
║ 🟠 CRITICAL       87.50%          1,250 units        ║
╠═══════════════════════════════════════════════════════╣
║ Severity: ERROR • Jan 9, 2025 4:45 PM               ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🟠 Orange

---

## 4. Exhausted State Alert (Error)

```
╔═══════════════════════════════════════════════════════╗
║ 🚨 Quota Alert: Quota Exhausted                      ║
╠═══════════════════════════════════════════════════════╣
║ Quota exhausted: 97.80% used, 15 channels affected   ║
║                                                       ║
║ State              Usage           Remaining          ║
║ 🔴 EXHAUSTED      97.80%          220 units          ║
║                                                       ║
║ 📺 Affected Channels                                 ║
║ • UCxxxxxxxxxxxxx                                     ║
║ • UCyyyyyyyyyyyyy                                     ║
║ • UCzzzzzzzzzzzzz                                     ║
║ ... and 12 more                                       ║
╠═══════════════════════════════════════════════════════╣
║ Severity: ERROR • Jan 9, 2025 6:15 PM               ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🔴 Red

---

## 5. Depleted State Alert (Critical)

```
╔═══════════════════════════════════════════════════════╗
║ ❌ Quota Alert: Quota Depleted                       ║
╠═══════════════════════════════════════════════════════╣
║ Quota depleted: 10,000/10,000 units used,            ║
║ all API requests blocked                              ║
║                                                       ║
║ State              Usage           Remaining          ║
║ ⛔ DEPLETED       100.00%         0 units            ║
╠═══════════════════════════════════════════════════════╣
║ Severity: CRITICAL • Jan 9, 2025 8:00 PM            ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: ⛔ Dark Red

---

## 6. Quota Recovered (Info)

```
╔═══════════════════════════════════════════════════════╗
║ ✅ Quota Alert: Quota Recovered                      ║
╠═══════════════════════════════════════════════════════╣
║ Quota recovered to healthy state: 0.00% used         ║
║                                                       ║
║ State              Usage           Remaining          ║
║ 🟢 HEALTHY        0.00%           10,000 units       ║
╠═══════════════════════════════════════════════════════╣
║ Severity: INFO • Jan 10, 2025 12:00 AM (PST)        ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🔵 Blue

---

## 7. Threshold Crossed Alert (Warning - 70%)

```
╔═══════════════════════════════════════════════════════╗
║ ⚠️ Quota Alert: Threshold Crossed                    ║
╠═══════════════════════════════════════════════════════╣
║ Quota crossed 70% threshold, now at 71.25%           ║
║                                                       ║
║ State              Usage           Remaining          ║
║ 🟡 DEGRADED       71.25%          2,875 units        ║
╠═══════════════════════════════════════════════════════╣
║ Severity: WARNING • Jan 9, 2025 2:15 PM             ║
╚═══════════════════════════════════════════════════════╝
```
**Color**: 🟡 Yellow

---

## 8. Periodic Status Update (All States)

### Healthy (24%)
- **Color**: 🟢 Green
- **Progress Bar**: `[████░░░░░░░░░░░░░░░░] 24.5%`
- **Polling Speed**: 1.00x (normal)

### Degraded (75%)
- **Color**: 🟡 Yellow
- **Progress Bar**: `[███████████████░░░░░] 75.0%`
- **Polling Speed**: 1.20x (slightly slower)

### Critical (89%)
- **Color**: 🟠 Orange
- **Progress Bar**: `[█████████████████░░░] 89.0%`
- **Polling Speed**: 1.50x (moderately slower)

### Exhausted (97%)
- **Color**: 🔴 Red
- **Progress Bar**: `[███████████████████░] 97.0%`
- **Polling Speed**: 2.00x (significantly slower)

### Depleted (100%)
- **Color**: ⛔ Dark Red
- **Progress Bar**: `[████████████████████] 100.0%`
- **Polling Speed**: 4.00x (maximum slowdown)

---

## Event Timeline Example

Here's what a typical day might look like:

```
12:00 AM - 🟢 Quota Reset
           📊 Status: 0% used, 10,000 units remaining

8:30 AM  - 📊 Periodic Update
           Status: 18.5% used (1,850 / 10,000 units)

2:15 PM  - ⚠️ Threshold Alert
           Crossed 70% threshold → DEGRADED state

3:30 PM  - 📊 Periodic Update
           Status: 76.2% used (7,620 / 10,000 units)

5:45 PM  - 🔄 State Change
           DEGRADED → CRITICAL (87.5% used)

7:20 PM  - 🚨 Quota Exhausted
           97.8% used, polling slowed down

9:15 PM  - ❌ Quota Depleted
           100% used, all requests blocked

11:59 PM - ⏰ Reset in 1 minute
```

---

## Bot Behavior Summary

| State | Color | Usage | Polling Speed | Actions |
|-------|-------|-------|---------------|---------|
| 🟢 HEALTHY | Green | 0-70% | 1.00x | Normal operation |
| 🟡 DEGRADED | Yellow | 70-85% | 1.20x | Reduce low-priority ops |
| 🟠 CRITICAL | Orange | 85-95% | 1.50x | Stop discovery, active only |
| 🔴 EXHAUSTED | Red | 95-100% | 2.00x | Slow down polling |
| ⛔ DEPLETED | Dark Red | 100%+ | Blocked | All requests blocked |

---

## Notification Frequency

- **State Changes**: Immediate (whenever state transitions)
- **Threshold Crossings**: Once per threshold (70%, 85%, 95%, 100%)
- **Periodic Updates**: Configurable (default: every 1 hour)
- **Recovery Alerts**: When quota resets at midnight PST
