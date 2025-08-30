# CSI Driver Upgrade Tests

Tests to verify PVC mounts survive CSI driver upgrades without disruption.

## Overview

These tests ensure that:

1. Existing PVC mounts remain active during CSI driver upgrade
2. No remounting occurs (mounts are NOT disrupted)
3. Data remains accessible throughout the upgrade
4. Active I/O operations continue without interruption

## Test Scenarios

### Static Provisioning Test

- Pre-provisioned PV with PVC
- Pod writing continuous data
- Verifies mount survives upgrade

### Dynamic Provisioning Test  

- Dynamic PVC using StorageClass
- Pod with open file handles
- Verifies no remounting occurs

## Usage

### Run Complete Test Suite

```bash
# Setup before upgrade
mage setupUpgradeTests

# Perform CSI driver upgrade
mage up  # or your upgrade command

# Verify after upgrade
mage verifyUpgradeTests

# Cleanup
mage cleanupUpgradeTests
```

### Run Individual Tests

```bash
# Setup only static test
UPGRADE_TEST_TYPE=static mage setupUpgradeTests

# Verify only dynamic test
UPGRADE_TEST_TYPE=dynamic mage verifyUpgradeTests
```

## How It Works

### Setup Phase (Before Upgrade)

1. Creates test namespace `upgrade-test`
2. Deploys pods with PVC mounts (static and dynamic)
3. Starts continuous write process with open file handles
4. Captures mount details (PID, mount ID, inode numbers)
5. Begins writing timestamps every 100ms to detect gaps

### Verification Phase (After Upgrade)

1. Checks mount process PID unchanged
2. Verifies open file handles still valid
3. Confirms mount ID unchanged
4. Analyzes continuous write log for gaps
5. Tests read/write capabilities

### What Proves No Remount Occurred

| Check | What It Proves |
|-------|---------------|
| Same mountpoint-s3 PID | Process never restarted |
| Open file handle valid | Kernel didn't unmount |
| Same mount ID | Linux sees it as same mount |
| No gaps in writes | I/O never interrupted |
| Same inode numbers | Filesystem wasn't recreated |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `UPGRADE_TEST_NAMESPACE` | Test namespace | `upgrade-test` |
| `UPGRADE_TEST_TYPE` | Run specific test (static/dynamic/all) | `all` |
| `UPGRADE_TEST_TIMEOUT` | Timeout for operations | `120s` |
| `UPGRADE_TEST_BUCKET` | S3 bucket for static test | `upgrade-test-static-bucket` |

## Troubleshooting

### Test Fails: "Mount was REMOUNTED"

- Check CSI driver logs during upgrade
- Verify no node restarts occurred
- Check if systemd restarted mount services

### Test Fails: "Continuous write stopped"

- Pod may have been evicted
- Check node resources (CPU/memory)
- Verify S3 endpoint remained accessible

### Test Fails: "Open file handle broken"

- Mount was disrupted during upgrade
- This indicates the upgrade is NOT seamless

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Test Pod  │────▶│  PVC Mount   │────▶│  S3 Bucket  │
│             │     │              │     │             │
│ - Writer    │     │ /data mount  │     │             │
│ - Open FDs  │     │              │     │             │
└─────────────┘     └──────────────┘     └─────────────┘
       │                    │
       ▼                    ▼
┌─────────────┐     ┌──────────────┐
│  Continuity │     │ Mount Details │
│   Tracker   │     │   Captured    │
│             │     │               │
│ - PID       │     │ - Mount ID    │
│ - Writes    │     │ - Inodes      │
└─────────────┘     └──────────────┘
```

## Test Results

### Expected Success Output

```
✅ Mount PID unchanged: 12345
✅ Process start time unchanged
✅ Mount ID unchanged: 123
✅ Open file handle still valid
✅ No significant gaps (max: 0.15s)
✅ Read/write operations working
✅ VERIFIED: Mount survived upgrade without remounting!
```

### Expected Failure Output (if mount was remounted)

```
❌ Mount PID changed: 12345 → 67890 (REMOUNTED!)
❌ Open file handle broken (REMOUNTED!)
❌ Found 15.30 second gap in writes (DISRUPTED!)
❌ mount was disrupted during upgrade
```

## CI Integration

The tests are designed to run in GitHub Actions with KIND clusters and verify that CSI driver upgrades maintain mount continuity for production workloads.
