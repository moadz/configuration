package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
	cfgobservatorium "github.com/rhobs/configuration/configuration/observatorium"
)

const (
	ClusterRHOBSUSWestTwoStaging ClusterName = "rhobss01uw2"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSUSWestTwoStaging,
		Environment: EnvironmentStaging,
		Namespace:   "rhobs-stage",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithTracingEnabled(),
			WithTenants(rhobss01uw2Tenants()),
			WithRBAC(rhobss01uw2RBAC()),
			WithCustomRoute("rhobs.us-west-2-0.api.stage.openshift.com"),
		),
		Templates:  rhobss01uwTemplateMaps(),
		BuildSteps: rhobss01uw2BuildSteps(),
	})
}

func rhobss01uw2Tenants() observatoriumapi.Tenants {
	return observatoriumapi.Tenants{
		Tenants: []observatoriumapi.Tenant{
			{
				Name: "hcp",
				ID:   "EFD08939-FE1D-41A1-A28A-BE9A9BC68003",
				OIDC: &observatoriumapi.TenantOIDC{
					ClientID:      "${CLIENT_ID}",
					ClientSecret:  "${CLIENT_SECRET}",
					IssuerURL:     "https://sso.redhat.com/auth/realms/redhat-external",
					RedirectURL:   "https://observatorium-mst.api.stage.openshift.com/oidc/odfms/callback",
					UsernameClaim: "preferred_username",
				},
			},
		},
	}
}

func rhobss01uw2RBAC() cfgobservatorium.ObservatoriumRBAC {
	opts := &cfgobservatorium.BindingOpts{}
	opts.WithServiceAccountName("45b1e1f4-6e17-4858-8f66-158320f6ac71").
		WithTenant(cfgobservatorium.HcpTenant).
		WithSignals([]cfgobservatorium.Resource{cfgobservatorium.MetricsResource, cfgobservatorium.LogsResource, cfgobservatorium.ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := cfgobservatorium.GenerateClusterRBAC(opts)
	return *config
}

func rhobss01uw2BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobss01uwTemplateMaps returns template mappings specific to ClusterRHOBSUSWestTwoStaging
func rhobss01uwTemplateMaps() TemplateMaps {
	// Start with integration base template and override only what's different
	lokiOverrides := LokiOverridesMap{
		LokiConfig: LokiOverrides{
			LokiLimitOverrides: LokiLimitOverrides{
				IngestionRateLimitMB: 20,
				PerStreamRateLimitMB: 15,
				PerStreamBurstSizeMB: 30,
				QueryTimeout:         "5m",
			},
			Ingest: LokiComponentSpec{
				Replicas: 3,
			},
		},
	}
	return DefaultBaseTemplate().Override(lokiOverrides)
}
