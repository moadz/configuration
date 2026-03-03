package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	tempov1alpha1 "github.com/grafana/tempo-operator/api/tempo/v1alpha1"
	"github.com/observatorium/observatorium/configuration_go/kubegen/openshift"
	templatev1 "github.com/openshift/api/template/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/configuration/clusters"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func (b Build) DefaultTempoStack(config clusters.ClusterConfig) {
	// For migrated clusters, generate traces bundle with individual resources
	if isMigratedCluster(config) {
		if err := generateTracesBundle(config); err != nil {
			log.Printf("Error generating traces bundle: %v", err)
		}
		return
	}

	gen := b.generator(config, "tempo-operator-default-cr")
	objs := []runtime.Object{
		NewTempoStack(config.Namespace, config.Templates),
	}

	gen.Add("tempo-operator-default-cr.yaml", encoding.GhodssYAML(
		openshift.WrapInTemplate(
			objs,
			metav1.ObjectMeta{Name: "tempo-rhobs"},
			[]templatev1.Parameter{
				{
					Name:  "TEMPO_STORAGE_SECRET_NAME",
					Value: "tempo-default-bucket",
				},
				{
					Name:  "TEMPO_STORAGE_CLASS",
					Value: "gp3-csi",
				},
			},
		),
	))

	gen.Generate()
}

// NewTempoStack creates a TempoStack custom resource
func NewTempoStack(namespace string, overrides clusters.TemplateMaps) *tempov1alpha1.TempoStack {
	tempoConfig := overrides.TempoOverrides[clusters.TempoConfig]
	retentionHours := fmt.Sprintf("%dh", tempoConfig.RetentionDays*24)

	return &tempov1alpha1.TempoStack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tempo.grafana.com/v1alpha1",
			Kind:       "TempoStack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "observatorium-tempostack",
			Namespace: namespace,
		},
		Spec: tempov1alpha1.TempoStackSpec{
			ManagementState:   tempov1alpha1.ManagementStateManaged,
			ReplicationFactor: tempoConfig.ReplicationFactor,
			LimitSpec: tempov1alpha1.LimitSpec{
				PerTenant: map[string]tempov1alpha1.RateLimitSpec{
					"dev": {
						Ingestion: tempov1alpha1.IngestionLimitSpec{
							IngestionBurstSizeBytes: ptr.To(tempoConfig.IngestionBurstSizeBytes),
							IngestionRateLimitBytes: ptr.To(tempoConfig.IngestionRateLimitBytes),
							MaxTracesPerUser:        ptr.To(tempoConfig.MaxTracesPerUser),
							MaxBytesPerTrace:        ptr.To(tempoConfig.MaxBytesPerTrace),
						},
						Query: tempov1alpha1.QueryLimit{
							MaxBytesPerTagValues:   ptr.To(tempoConfig.MaxBytesPerTagValues),
							MaxSearchBytesPerTrace: ptr.To(tempoConfig.MaxSearchBytesPerTrace),
						},
					},
				},
			},
			StorageSize: resource.MustParse(tempoConfig.StorageSize),
			Storage: tempov1alpha1.ObjectStorageSpec{
				Secret: tempov1alpha1.ObjectStorageSecretSpec{
					Type: tempov1alpha1.ObjectStorageSecretS3,
					Name: "${TEMPO_STORAGE_SECRET_NAME}",
				},
			},
			StorageClassName: ptr.To("${TEMPO_STORAGE_CLASS}"),
			Retention: tempov1alpha1.RetentionSpec{
				Global: tempov1alpha1.RetentionConfig{
					Traces: metav1.Duration{Duration: parseDuration(retentionHours)},
				},
			},
			Template: tempov1alpha1.TempoTemplateSpec{
				Distributor: tempov1alpha1.TempoDistributorSpec{
					TempoComponentSpec: tempov1alpha1.TempoComponentSpec{
						Replicas: &tempoConfig.Distributor.Replicas,
					},
				},
				Ingester: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Ingester.Replicas,
				},
				Compactor: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Compactor.Replicas,
				},
				Querier: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Querier.Replicas,
				},
				QueryFrontend: tempov1alpha1.TempoQueryFrontendSpec{
					TempoComponentSpec: tempov1alpha1.TempoComponentSpec{
						Replicas: &tempoConfig.QueryFrontend.Replicas,
					},
					JaegerQuery: tempov1alpha1.JaegerQuerySpec{
						Enabled: false,
					},
				},
			},
			Observability: tempov1alpha1.ObservabilitySpec{
				Metrics: tempov1alpha1.MetricsConfigSpec{
					CreateServiceMonitors: false,
				},
			},
		},
	}
}

// NewBundleTempoStack creates a TempoStack with concrete values for bundle deployment (no template parameters)
func NewBundleTempoStack(namespace string, overrides clusters.TemplateMaps) *tempov1alpha1.TempoStack {
	tempoConfig := overrides.TempoOverrides[clusters.TempoConfig]
	retentionHours := fmt.Sprintf("%dh", tempoConfig.RetentionDays*24)

	// ptrIfNonZero returns a pointer to the value if it's non-zero, otherwise nil
	ptrIfNonZero := func(v int) *int {
		if v == 0 {
			return nil
		}
		return &v
	}

	return &tempov1alpha1.TempoStack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tempo.grafana.com/v1alpha1",
			Kind:       "TempoStack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "observatorium-tempostack",
			Namespace: namespace,
		},
		Spec: tempov1alpha1.TempoStackSpec{
			ManagementState:   tempov1alpha1.ManagementStateManaged,
			ReplicationFactor: tempoConfig.ReplicationFactor,
			LimitSpec: tempov1alpha1.LimitSpec{
				PerTenant: map[string]tempov1alpha1.RateLimitSpec{
					"dev": {
						Ingestion: tempov1alpha1.IngestionLimitSpec{
							IngestionBurstSizeBytes: ptr.To(tempoConfig.IngestionBurstSizeBytes),
							IngestionRateLimitBytes: ptr.To(tempoConfig.IngestionRateLimitBytes),
							MaxTracesPerUser:        ptrIfNonZero(tempoConfig.MaxTracesPerUser),
							MaxBytesPerTrace:        ptrIfNonZero(tempoConfig.MaxBytesPerTrace),
						},
						Query: tempov1alpha1.QueryLimit{
							MaxBytesPerTagValues:   ptrIfNonZero(tempoConfig.MaxBytesPerTagValues),
							MaxSearchBytesPerTrace: ptrIfNonZero(tempoConfig.MaxSearchBytesPerTrace),
						},
					},
				},
			},
			StorageSize: resource.MustParse(tempoConfig.StorageSize),
			Storage: tempov1alpha1.ObjectStorageSpec{
				Secret: tempov1alpha1.ObjectStorageSecretSpec{
					Type: tempov1alpha1.ObjectStorageSecretS3,
					Name: "tempo-default-bucket",
				},
			},
			StorageClassName: ptr.To("gp3-csi"),
			Retention: tempov1alpha1.RetentionSpec{
				Global: tempov1alpha1.RetentionConfig{
					Traces: metav1.Duration{Duration: parseDuration(retentionHours)},
				},
			},
			Template: tempov1alpha1.TempoTemplateSpec{
				Distributor: tempov1alpha1.TempoDistributorSpec{
					TempoComponentSpec: tempov1alpha1.TempoComponentSpec{
						Replicas: &tempoConfig.Distributor.Replicas,
					},
				},
				Ingester: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Ingester.Replicas,
				},
				Compactor: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Compactor.Replicas,
				},
				Querier: tempov1alpha1.TempoComponentSpec{
					Replicas: &tempoConfig.Querier.Replicas,
				},
				QueryFrontend: tempov1alpha1.TempoQueryFrontendSpec{
					TempoComponentSpec: tempov1alpha1.TempoComponentSpec{
						Replicas: &tempoConfig.QueryFrontend.Replicas,
					},
					JaegerQuery: tempov1alpha1.JaegerQuerySpec{
						Enabled: false,
					},
				},
			},
			Observability: tempov1alpha1.ObservabilitySpec{
				Metrics: tempov1alpha1.MetricsConfigSpec{
					CreateServiceMonitors: false,
				},
			},
		},
	}
}

// parseDuration parses a duration string and returns a time.Duration
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Error parsing duration %s: %v, using default", s, err)
		return 720 * time.Hour
	}
	return d
}

// generateTracesBundle generates individual Tempo component resources for bundle deployment
func generateTracesBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With("resources", "clusters", string(config.Environment), string(config.Name), "traces", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// 1. CRDs (prefix: 01-*)
	crdObjs := getTempoCRDObjects()
	crdNames := []string{"tempostacks", "tempomonolithics"}
	for i, crd := range crdObjs {
		crdName := "unknown"
		if i < len(crdNames) {
			crdName = crdNames[i]
		}
		filename := fmt.Sprintf("01-crd-%s.yaml", crdName)
		bundleGen.Add(filename, encoding.GhodssYAML(crd))
	}

	// 2. OPERATOR (prefix: 02-*)
	operatorObjs := tempoOperatorResources(ns)
	for i, obj := range operatorObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getTempoResourceName(obj)
		filename := fmt.Sprintf("02-operator-%02d-%s-%s.yaml", i+1, resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// 3. TEMPOSTACK RESOURCES (prefix: 03-*)
	tempoStackObjs := make([]runtime.Object, 0, 1)
	tempoStackObjs = append(tempoStackObjs, NewBundleTempoStack(ns, config.Templates))

	for _, obj := range tempoStackObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getTempoResourceName(obj)
		// Clean up names and remove redundant prefixes
		resourceName = strings.TrimPrefix(resourceName, "observatorium-")
		filename := fmt.Sprintf("03-%s-%s.yaml", resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add consolidated ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	tempoServiceMonitors := createConsolidatedTempoServiceMonitors(ns)

	for _, sm := range tempoServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	return nil
}

// getTempoCRDObjects retrieves Tempo operator CRDs
func getTempoCRDObjects() []runtime.Object {
	const (
		tempostacks     = "tempo.grafana.com_tempostacks.yaml"
		tempomonolithic = "tempo.grafana.com_tempomonolithics.yaml"
		base            = "https://raw.githubusercontent.com/grafana/tempo-operator/" + tempoOperatorCRDRef + "/bundle/openshift/manifests/"
	)

	var objs []runtime.Object
	for _, component := range []string{tempostacks, tempomonolithic} {
		crd, err := getCustomResourceDefinition(base + component)
		if err != nil {
			log.Printf("Error fetching CRD %s: %v", component, err)
			continue
		}
		objs = append(objs, crd)
	}
	return objs
}

// getTempoResourceName extracts a meaningful name from a Tempo Kubernetes object
func getTempoResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	switch o := obj.(type) {
	case metav1.Object:
		name := o.GetName()
		if name != "" {
			// Remove redundant tempo prefix since it's implied by being in the traces bundle
			name = strings.TrimPrefix(name, "tempo-")
			return name
		}
	}

	// Fallback to the object type
	return "unnamed"
}

// createConsolidatedTempoServiceMonitors creates ServiceMonitors for Tempo components
func createConsolidatedTempoServiceMonitors(namespace string) []runtime.Object {
	return []runtime.Object{
		// Tempo Operator Controller Manager
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-operator-controller-manager-metrics",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "monitoring",
					"app.kubernetes.io/created-by": "tempo-operator",
					"app.kubernetes.io/instance":   "controller-manager-metrics",
					"app.kubernetes.io/managed-by": "rhobs",
					"app.kubernetes.io/name":       "servicemonitor",
					"app.kubernetes.io/part-of":    "tempo-operator",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Port: "https",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name":      "tempo-operator",
						"app.kubernetes.io/component": "controller",
					},
				},
			},
		},
		// Tempo Compactor
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-compactor",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "compactor",
					"app.kubernetes.io/instance":   "observatorium-tempostack",
					"app.kubernetes.io/managed-by": "tempo-operator",
					"app.kubernetes.io/name":       "tempo",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "compactor",
						"app.kubernetes.io/instance":  "observatorium-tempostack",
						"app.kubernetes.io/name":      "tempo",
					},
				},
			},
		},
		// Tempo Distributor
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-distributor",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "distributor",
					"app.kubernetes.io/instance":   "observatorium-tempostack",
					"app.kubernetes.io/managed-by": "tempo-operator",
					"app.kubernetes.io/name":       "tempo",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "distributor",
						"app.kubernetes.io/instance":  "observatorium-tempostack",
						"app.kubernetes.io/name":      "tempo",
					},
				},
			},
		},
		// Tempo Ingester
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-ingester",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "ingester",
					"app.kubernetes.io/instance":   "observatorium-tempostack",
					"app.kubernetes.io/managed-by": "tempo-operator",
					"app.kubernetes.io/name":       "tempo",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "ingester",
						"app.kubernetes.io/instance":  "observatorium-tempostack",
						"app.kubernetes.io/name":      "tempo",
					},
				},
			},
		},
		// Tempo Querier
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-querier",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "querier",
					"app.kubernetes.io/instance":   "observatorium-tempostack",
					"app.kubernetes.io/managed-by": "tempo-operator",
					"app.kubernetes.io/name":       "tempo",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "querier",
						"app.kubernetes.io/instance":  "observatorium-tempostack",
						"app.kubernetes.io/name":      "tempo",
					},
				},
			},
		},
		// Tempo Query Frontend
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "tempo-query-frontend",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "query-frontend",
					"app.kubernetes.io/instance":   "observatorium-tempostack",
					"app.kubernetes.io/managed-by": "tempo-operator",
					"app.kubernetes.io/name":       "tempo",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "query-frontend",
						"app.kubernetes.io/instance":  "observatorium-tempostack",
						"app.kubernetes.io/name":      "tempo",
					},
				},
			},
		},
	}
}
