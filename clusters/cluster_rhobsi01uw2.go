package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
	cfgobservatorium "github.com/rhobs/configuration/configuration/observatorium"
)

const (
	ClusterRHOBSUSWestIntegration ClusterName = "rhobsi01uw2"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSUSWestIntegration,
		Environment: EnvironmentIntegration,
		Namespace:   "rhobs-int",
		AMSUrl:      "https://api.openshift.com",
		RBAC:        rhobsi01uw2RBAC(),
		Tenants:     rhobsi01uw2Tenants(),
		Templates:   rhobsi01uw2TemplateMaps(),
		BuildSteps:  rhobsi01uw2BuildSteps(),
	})
}

func rhobsi01uw2Tenants() observatoriumapi.Tenants {
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

func rhobsi01uw2RBAC() cfgobservatorium.ObservatoriumRBAC {
	opts := &cfgobservatorium.BindingOpts{}
	opts.WithServiceAccountName("d4045e4b-7b9c-46fc-8af0-5d483d9d205b").
		WithTenant(cfgobservatorium.HcpTenant).
		WithSignals([]cfgobservatorium.Signal{cfgobservatorium.MetricsSignal, cfgobservatorium.LogsSignal}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := cfgobservatorium.GenerateClusterRBAC(opts)
	return *config
}

func rhobsi01uw2BuildSteps() []string {
	return DefaultBuildSteps()
}

// rhobsi01uw2TemplateMaps returns template mappings specific to the rhobsi01uw2 integration cluster
func rhobsi01uw2TemplateMaps() TemplateMaps {
	// Start with integration base template and override only what's different
	return DefaultBaseTemplate().Override()
}
