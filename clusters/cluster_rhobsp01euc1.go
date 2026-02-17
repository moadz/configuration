package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
	cfgobservatorium "github.com/rhobs/configuration/configuration/observatorium"
)

const (
	ClusterRHOBSEuropeCentralOneProduction ClusterName = "rhobsp01euc1"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSEuropeCentralOneProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithTracingEnabled(),
			WithTenants(rhobsp01euc1Tenants()),
			WithRBAC(rhobsp01euc1RBAC()),
			WithCustomRoute("eu-central-1-0.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp01euc1TemplateMaps(),
		BuildSteps: rhobsp01euc1BuildSteps(),
	})
}

func rhobsp01euc1Tenants() observatoriumapi.Tenants {
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
					UsernameClaim: "client_id",
				},
			},
		},
	}
}

func rhobsp01euc1RBAC() cfgobservatorium.ObservatoriumRBAC {
	opts := &cfgobservatorium.BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(cfgobservatorium.HcpTenant).
		WithSignals([]cfgobservatorium.Resource{cfgobservatorium.MetricsResource, cfgobservatorium.LogsResource, cfgobservatorium.ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write}).
		WithRawSubjectName()

	config := cfgobservatorium.GenerateClusterRBAC(opts)
	return *config
}

func rhobsp01euc1BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp01euc1TemplateMaps returns template mappings specific to the rhobsp01euc1 production cluster
func rhobsp01euc1TemplateMaps() TemplateMaps {
	return DefaultBaseTemplate().Override()
}
