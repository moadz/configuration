# HCP Tenant Rules

This directory contains PrometheusRule templates for ROSA HCP (Hosted Control Plane) monitoring, organized by functional domain for improved maintainability.

## Directory Structure

```
hcp/
├── README.md              # This file
├── api-server.yaml        # kube-apiserver, openshift-apiserver, error-budget-burn
├── audit.yaml             # Audit webhook CloudWatch alerts
├── billing.yaml           # Billing metric alerts
├── cert-manager.yaml      # TLS certificate health
├── cluster-operators.yaml # ClusterOperator health alerts
├── kube-api-error-budget.yaml # KubeAPI SLO error budget burn
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
- `api-SLOs-probe` - Probe-based SLO alerts (api-ErrorBudgetBurn)
- `api-rapid-burn` - Rapid error budget burn (api-RapidErrorBudgetBurn)
- `kube-apiserver-restarts` - Recording rule + KubeAPIServerRestartingFrequently
- `osd-kube-apiserver-rules` - KubeAPIServer/OpenshiftAPIServer Down/Degraded

### control-plane.yaml
Core control plane component monitoring:
- `sre-etcd-rules` - etcd leader, quota alerts
- `kube-controller-manager` - Controller manager availability
- `kube-scheduler` - Scheduler availability

### cert-manager.yaml
TLS certificate health:
- `cert-manager` - CertManagerCertExpirySoon, CertManagerCertNotReady

### kube-api-error-budget.yaml
KubeAPI SLO error budget burn (99.99% availability target):
- `KubeAPIErrorBudgetBurn_1m_eval` - Base counters, short-window error rates (5m-6h), fast/medium burn alerts
- `KubeAPIErrorBudgetBurn_15m_eval` - Long-window error rates (1d, 3d), slow burn alerts
- 4 alert variants: 5m/1h (fast), 30m/6h (medium), 2h/1d (slow), 6h/3d (very slow)

### cluster-operators.yaml
OpenShift ClusterOperator health:
- `cluster-operators` - ClusterOperatorDegraded, ClusterOperatorDown
- `core-cluster-operators` - Recording rules (hcp_worker_nodes:available_count, core_cluster_operator:down:filtered) + CoreClusterOperatorDown, DefaultIngressControllerDegraded

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
- `SAEDeploymentErrors` - SAEDeploymentMissing, SAEDeploymentDown, SAEDeploymentDoesNotHaveExpectedReplicas

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

1. Identify the appropriate functional domain (or create a new file)
2. Add the PrometheusRule to the corresponding file
3. If creating a new file, add it to the `FILES` array in `scripts/generate-hcp-rules.sh`
4. Run `make hcp-rules` to regenerate `../hcp.yaml`
5. Update this README with the new alert/recording rule
6. Commit both the source file and the generated `hcp.yaml`
7. Include `unless on (_id) hypershift_cluster_alerts_disabled` in alert expressions for limited support suppression
8. Start new alerts with `severity: soaking` -- see the [alert graduation process](https://gitlab.cee.redhat.com/service/hypershift-pagerduty-config/-/blob/main/docs/ALERT-GRADUATION.md)
