# SC Tenant Rules

PrometheusRule templates for ROSA HCP Service Cluster (SC) monitoring, evaluated on RHOBS cells.

## Directory Structure

```
sc/
├── README.md                  # This file
├── acm-managed-clusters.yaml  # ACM managed cluster condition alerts
├── acm-grc.yaml               # ACM GRC policy controller alerts
├── acm-manifestwork.yaml      # ACM ManifestWork failure alerts
├── cert-manager.yaml          # TLS certificate health alerts
└── observability.yaml         # DeadMansSnitch watchdog
```

## Alerts

| Alert | File | Severity | For |
|-------|------|----------|-----|
| ACMManagedClusterConditionUnknown | acm-managed-clusters.yaml | warning | 10m |
| ACMManagedClusterKubeAPIServerUnavailable | acm-managed-clusters.yaml | warning | 15m |
| ACMManagedClusterClientCertRotationFailed | acm-managed-clusters.yaml | warning | 12h |
| ACMPolicyControllerReconcileErrors | acm-grc.yaml | warning | 5m |
| ACMManifestWorkAppliedHighFailureRate | acm-manifestwork.yaml | warning | 5m |
| CertManagerCertExpirySoon | cert-manager.yaml | warning | 1h |
| CertManagerCertNotReady | cert-manager.yaml | warning | 10m |
| DeadMansSnitch | observability.yaml | critical | -- |

## Recording Rules

ManifestWork recording rules run on each SC via the monitoring stack template
(`resources/collection/metrics/hypershift-monitoring-stack-template.yaml`), not here.
Only the alert that consumes the recording rule results lives in these tenant rules.

## Adding New Rules

1. Add the PrometheusRule to the appropriate file (or create a new one)
2. Update this README
3. Follow the [promotion process](docs/sop/hcp_configuration_promotion.md) for deploying to production
