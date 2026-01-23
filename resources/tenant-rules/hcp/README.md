# HCP Tenant Rules

This directory contains PrometheusRule templates for ROSA HCP (Hosted Control Plane) monitoring, organized by functional domain for improved maintainability.

## Directory Structure

```
hcp/
├── README.md              # This file
├── api-server.yaml        # kube-apiserver, openshift-apiserver, error-budget-burn
├── audit.yaml             # Audit webhook CloudWatch alerts
├── billing.yaml           # Billing metric alerts
├── cluster-operators.yaml # ClusterOperator health alerts
├── control-plane.yaml     # etcd, kube-controller-manager, kube-scheduler
├── nodes.yaml             # Node health, nodepool, autoscaler
├── oauth.yaml             # OAuth service health
├── observability.yaml     # Watchdog, prometheus targets
└── splunk.yaml            # SAE (Splunk Audit Exporter) deployment alerts
```

## Files by Functional Domain

### api-server.yaml
API server availability and SLO monitoring:
- `kube-api-error-budget-burn` - Recording rules for error budget calculations
- `api` - Probe-based SLO alerts (api-ErrorBudgetBurn)
- `sre-kube-apiserver-rules` - KubeAPIServer/OpenshiftAPIServer Down/Degraded

### control-plane.yaml
Core control plane component monitoring:
- `sre-etcd-rules` - etcd leader, quota alerts
- `kube-controller-manager` - Controller manager availability
- `kube-scheduler` - Scheduler availability

### cluster-operators.yaml
OpenShift ClusterOperator health:
- `cluster-operators` - ClusterOperatorDegraded, ClusterOperatorDown

### nodes.yaml
Worker node and nodepool health:
- `nodepool-failure` - NodePoolFailing
- `nodes-need-upscale` - NodesNeedUpscale
- `cluster-autoscaler-rules` - ClusterAutoscalerDown
- `nodes-rules` - NodeHighResourceUsage, NodeNotReady, NodeInBadCondition
- `sre-node-not-joining-nodepool-sre-actionable-rules` - NodepoolFailureSRE
- `sre-nodes-need-upscale-rules` - RequestServingNodesNeedUpscale

### observability.yaml
Monitoring infrastructure health:
- `watchdog` - DeadMansSnitch heartbeat
- `sre-prometheus-target-alerting` - PodMonitor/ServiceMonitor health

### oauth.yaml
OAuth service health:
- `oauth-service-health` - OauthServiceDeploymentDegraded, OauthServiceDeploymentDown

### billing.yaml
Billing metric availability:
- `billing-rules` - BillingMetricMissing

### audit.yaml
Audit log forwarding to CloudWatch:
- `audit-webhook-error` - AuditWebhookIncorrectCloudwatchConfiguration, AuditWebhookCloudWatchErrors

### splunk.yaml
Splunk Audit Exporter health:
- `sae-deployment` - SAEDeploymentMissing, SAEDeploymentDown

## Usage

Each file is a standalone OpenShift Template that can be processed with `oc process`:

```bash
# Process a single domain
oc process -f api-server.yaml \
  -p NAMESPACE=rhobs-hcp \
  -p TENANT=hcp | oc apply -f -

# Process all domains
for f in *.yaml; do
  oc process -f "$f" \
    -p NAMESPACE=rhobs-hcp \
    -p TENANT=hcp | oc apply -f -
done
```

## Parameters

All templates accept the same parameters:

| Parameter | Required | Description |
|-----------|----------|-------------|
| NAMESPACE | Yes | Namespace to deploy the rules to |
| TENANT | Yes | Tenant identifier for Thanos operator |

## Migration from hcp.yaml

This directory structure replaces the monolithic `../hcp.yaml` file. The split provides:

1. **Easier code review** - Changes to node alerts don't require reviewing API server rules
2. **Domain ownership** - Teams can own specific functional domains
3. **Selective deployment** - Deploy only the domains needed for testing
4. **Reduced merge conflicts** - Parallel work on different domains

## Adding New Rules

1. Identify the appropriate functional domain
2. Add the PrometheusRule to the corresponding file
3. Update this README if adding a new PrometheusRule object
4. Test with `oc process -f <file>.yaml -p NAMESPACE=test -p TENANT=test`
