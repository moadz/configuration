package clusters

import (
	"fmt"

	cfgobservatorium "github.com/rhobs/configuration/configuration/observatorium"

	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
)

// ClusterName represents a specific cluster identifier
type ClusterName string

// ClusterEnvironment represents the deployment environment
type ClusterEnvironment string

// Supported cluster environments
const (
	EnvironmentIntegration ClusterEnvironment = "integration"
	EnvironmentStaging     ClusterEnvironment = "staging"
	EnvironmentProduction  ClusterEnvironment = "production"
)

// ClusterConfig holds the configuration for a specific cluster deployment
type ClusterConfig struct {
	Name        ClusterName
	Environment ClusterEnvironment
	Namespace   string
	Templates   TemplateMaps
	RBAC        cfgobservatorium.ObservatoriumRBAC
	Tenants     observatoriumapi.Tenants
	AMSUrl      string
	BuildSteps  []string
}

// String returns the string representation of ClusterName
func (c ClusterName) String() string {
	return string(c)
}

// String returns the string representation of ClusterEnvironment
func (e ClusterEnvironment) String() string {
	return string(e)
}

// IsValid checks if the cluster environment is valid
func (e ClusterEnvironment) IsValid() bool {
	switch e {
	case EnvironmentIntegration, EnvironmentStaging, EnvironmentProduction:
		return true
	default:
		return false
	}
}

// Validate checks if the cluster configuration is valid
func (c ClusterConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if !c.Environment.IsValid() {
		return fmt.Errorf("invalid environment: %s", c.Environment)
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if len(c.BuildSteps) == 0 {
		return fmt.Errorf("cluster must have at least one build step")
	}
	return nil
}

// ClusterRegistry holds all registered clusters
var ClusterRegistry = make(map[ClusterName]ClusterConfig)

// BuildStep represents a string key for each template generation step
type BuildStep string

// Available build steps
const (
	StepThanosOperatorCRDS = "thanos-operator-crds"
	StepThanosOperator     = "thanos-operator"
	StepDefaultThanosStack = "default-thanos-stack"

	StepLokiOperatorCRDS = "loki-operator-crds"
	StepLokiOperator     = "loki-operator"
	StepDefaultLokiStack = "default-loki-stack"

	StepServiceMonitors = "servicemonitors"

	StepAlertmanager = "alertmanager"
	StepSecrets      = "secrets"
	StepGateway      = "gateway"
	StepMemcached    = "memcached"

	StepSyntheticsApi = "synthetics-api"

	StepThanosRules = "thanos-rules"
)

// DefaultBuildSteps returns the default build pipeline for clusters
func DefaultBuildSteps() []string {
	var steps []string
	steps = append(steps, DefaultMetricsBuildSteps()...)
	steps = append(steps, DefaultLoggingBuildSteps()...)
	steps = append(steps, DefaultSyntheticsBuildSteps()...)

	steps = append(steps,
		StepServiceMonitors, // Monitoring setup
		StepAlertmanager,    // Alerting configuration
		StepSecrets,         // Secrets last
		StepMemcached,       // Memcached configuration
		StepGateway,         // Gateway configuration
		StepThanosRules,     // Thanos metamonitoring Rules configuration last}
	)
	return steps
}

func DefaultMetricsBuildSteps() []string {
	return []string{
		StepThanosOperatorCRDS, // Core components first
		StepThanosOperator,     // Custom Resource Definitions
		StepDefaultThanosStack, // ThanosOperator deployment
	}
}

func DefaultLoggingBuildSteps() []string {
	return []string{
		StepLokiOperatorCRDS,
		StepLokiOperator,
		StepDefaultLokiStack,
	}
}

func DefaultSyntheticsBuildSteps() []string {
	return []string{
		StepSyntheticsApi,
	}
}

// Prune is a utility function to remove specified steps from a list
func Prune(from []string, prune ...[]string) []string {
	pruneMap := make(map[string]struct{})
	for _, p := range prune {
		for _, step := range p {
			pruneMap[step] = struct{}{}
		}
	}

	var result []string
	for _, step := range from {
		if _, shouldPrune := pruneMap[step]; !shouldPrune {
			result = append(result, step)
		}
	}
	return result
}

// RegisterCluster registers a cluster configuration with validation
func RegisterCluster(config ClusterConfig) {
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Invalid cluster %s: %v", config.Name, err))
	}
	ClusterRegistry[config.Name] = config
}

// GetClusters returns all registered clusters
func GetClusters() []ClusterConfig {
	var clusters []ClusterConfig
	for _, cluster := range ClusterRegistry {
		clusters = append(clusters, cluster)
	}
	return clusters
}

// GetClusterByName finds a cluster configuration by name
func GetClusterByName(name ClusterName) (*ClusterConfig, error) {
	if cluster, exists := ClusterRegistry[name]; exists {
		return &cluster, nil
	}
	return nil, fmt.Errorf("cluster not found: %s", name)
}

// GetClustersByEnvironment returns all clusters for a specific environment
func GetClustersByEnvironment(env ClusterEnvironment) []ClusterConfig {
	var clusters []ClusterConfig
	for _, cluster := range ClusterRegistry {
		if cluster.Environment == env {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}
