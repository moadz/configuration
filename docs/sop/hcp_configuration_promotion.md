# HCP Configuration Promotion

How to promote RHOBS HCP configuration changes (alert rules, recording rules, monitoring stack templates) from non-production to production RHOBS cells.

## Context

Changes to this repository deploy to RHOBS cells via app-interface saas files. To avoid breaking production alerting, changes should be validated in non-production environments before promoting to production.

Currently, all saas file targets use `ref: main`, which means changes deploy to all environments simultaneously. To gate production deployment, production targets should use a pinned SHA that is manually promoted after validation.

## RHOBS Cell Environments

| Cluster | Environment | Region | Saas Target |
|---------|-------------|--------|-------------|
| rhobsi01uw2 | Integration | us-west-2 | `ref: main` (auto-deploy) |
| rhobss01uw2 | Stage | us-west-2 | `ref: main` (auto-deploy) |
| rhobss01ue1 | Stage | us-east-1 | `ref: main` (auto-deploy) |
| rhobsp01ue1 | Production | us-east-1 | `ref: <pinned-sha>` |

## Prerequisites

1. Access to [app-interface](https://gitlab.cee.redhat.com/service/app-interface)
2. OCM access (production) for cluster validation
3. PagerDuty API access for alert comparison

## How to promote

### Step 1: Merge changes to main

Submit and merge PRs to this repository. Changes automatically deploy to integration and stage RHOBS cells via `ref: main`.

### Step 2: Validate in non-production

Wait for app-interface reconciliation to deploy the changes to non-production cells (typically 15-30 minutes).

Verify PrometheusRules are deployed:

```bash
ocm login --use-auth-code --url production
ocm backplane login rhobsi01uw2 --multi
oc get prometheusrules -n rhobs-int --no-headers
```

For monitoring stack template changes (recording rules, allowlist), verify on a stage MC:

```bash
# Check the SelectorSyncSet on a hive cluster
ocm backplane login hive-stage-01 --multi
ocm backplane elevate "checking SSS" -- \
  get selectorsyncset rhobs-hcp-monitoring-stackus-west-2 -o yaml | \
  grep -E 'kind:|name:.*recording|name:.*rhobs-hcp'
```

Check PagerDuty for alerts firing correctly:

```bash
PD_TOKEN="<your-token>"
SINCE=$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ)

# RHOBS stage incidents
curl -sS "https://api.pagerduty.com/incidents?service_ids[]=P80K90K&service_ids[]=P4RZ8OX&since=$SINCE&limit=100" \
  -H "Authorization: Token token=$PD_TOKEN" \
  -H "Accept: application/vnd.pagerduty+json;version=2" | \
  jq '.incidents[] | {title: .title[:80], status, created_at: .created_at[:16]}'
```

Compare with Dynatrace stage to confirm parity (see the [alerting validation runbook](https://gitlab.cee.redhat.com/service/hypershift-pagerduty-config/-/blob/main/docs/ALERTING-VALIDATION.md) for detailed steps).

### Step 3: Promote to production

Once validated, submit an MR to app-interface updating the production target's `ref` to the validated SHA.

File: `data/services/rhobs/rhobs/cicd/saas-metric-collection.yaml`

```yaml
# Before
- name: hypershift-rhobs-monitoring-stack-production-us-east-1-canary
  ref: <old-sha>

# After
- name: hypershift-rhobs-monitoring-stack-production-us-east-1-canary
  ref: <validated-sha>  # validated in stage on YYYY-MM-DD
```

Get the SHA to promote:

```bash
# Use the commit that's currently deployed to stage
git log --oneline -1 main
```

The MR is self-serviceable for `rhobs-dev` role members via the `saas-file-self-service` changetype.

### Step 4: Verify production

After the app-interface MR merges, verify the production RHOBS cell:

```bash
ocm backplane login rhobsp01ue1 --multi
oc get prometheusrules -n rhobs-production --no-headers
```

Check PagerDuty production services for expected alert behavior.

## What to promote

The same saas file ref controls both:

- **Tenant rules** (alert rules in `resources/tenant-rules/hcp/`) -- deployed to RHOBS cell as PrometheusRules for Thanos ruler
- **Monitoring stack template** (`resources/collection/metrics/hypershift-monitoring-stack-template.yaml`) -- deployed to MCs via SelectorSyncSet, contains recording rules and metric allowlists

Both are promoted together since they're in the same repository and referenced by the same saas file target.

## Timing considerations

After promoting to production, allow time for changes to propagate:

| Change Type | Propagation Time | Notes |
|-------------|-----------------|-------|
| Tenant rules (alert rules) | ~15 min | Deployed directly to RHOBS cell namespace |
| Monitoring stack template (recording rules) | ~30-60 min | Deployed via Hive SelectorSyncSet to each MC |
| Recording rule data accumulation | Varies | `rate()` needs 2-5 min, `increase()` needs full window (e.g. 30 min) |
| Alert `for` duration | Varies | Must wait full duration (e.g. 60 min for ClusterOperatorDown) |

## Rollback

To rollback production, submit an MR reverting the production target's `ref` to the previous SHA:

```yaml
- name: hypershift-rhobs-monitoring-stack-production-us-east-1-canary
  ref: <previous-sha>  # rollback from <bad-sha>
```

## References

- [Alerting validation runbook](https://gitlab.cee.redhat.com/service/hypershift-pagerduty-config/-/blob/main/docs/ALERTING-VALIDATION.md)
- [Alert graduation process](https://gitlab.cee.redhat.com/service/hypershift-pagerduty-config/-/blob/main/docs/ALERT-GRADUATION.md)
- [HCP alert rules](resources/tenant-rules/hcp/README.md)
- Jira: [SREP-3431](https://issues.redhat.com/browse/SREP-3431)
