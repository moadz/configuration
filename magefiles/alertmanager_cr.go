package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rhobs/configuration/clusters"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	alertmanagerName = "rhobs-alertmanager"
	alertmanagerPort = 9093
	defaultReplicas  = 1
)

// alertmanagerConfig holds the configuration for Alertmanager deployment
type alertmanagerConfig struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Replicas  *int32
}

func (b Build) AlertmanagerCR(config clusters.ClusterConfig) {
	// For rhobss01ue1 cluster, generate alertmanager bundle with individual resources
	if config.Name == "rhobss01ue1" {
		if err := generateAlertmanagerBundle(config); err != nil {
			log.Printf("Error generating alertmanager bundle: %v", err)
		}
		return
	}

	// For other clusters, generate templates (if needed in the future)
	log.Printf("Alertmanager CR generation not yet implemented for cluster: %s", config.Name)
}

// generateAlertmanagerBundle generates individual alertmanager component resources for bundle deployment
func generateAlertmanagerBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With("resources", "clusters", string(config.Environment), string(config.Name), "alertmanager", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// Create alertmanager resources with concrete values
	alertmanagerConfig := newBundleAlertmanagerConfig(ns)

	// Generate individual alertmanager resource files
	alertmanagerObjs := []runtime.Object{
		createAlertmanager(alertmanagerConfig),
	}

	for i, obj := range alertmanagerObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getKubernetesResourceName(obj)
		filename := fmt.Sprintf("%02d-%s-%s.yaml", i+1, resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	alertmanagerServiceMonitors := createAlertmanagerServiceMonitors(ns)

	for _, sm := range alertmanagerServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	return nil
}

// newBundleAlertmanagerConfig creates an alertmanager config with concrete values for bundle deployment
func newBundleAlertmanagerConfig(namespace string) *alertmanagerConfig {
	replicas := int32(defaultReplicas)
	return &alertmanagerConfig{
		Name:      alertmanagerName,
		Namespace: namespace,
		Replicas:  &replicas,
		Labels: map[string]string{
			"app.kubernetes.io/component":  "alertmanager",
			"app.kubernetes.io/instance":   "rhobs",
			"app.kubernetes.io/name":       "alertmanager",
			"app.kubernetes.io/part-of":    "rhobs",
			"app.kubernetes.io/managed-by": "observability-operator",
		},
	}
}

// createAlertmanager creates an Alertmanager CR using observability-operator
func createAlertmanager(config *alertmanagerConfig) *monitoringv1.Alertmanager {
	//TODO(simonpasquier): make it consistent with the ServiceMonitor's API Group.
	return &monitoringv1.Alertmanager{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Alertmanager",
			APIVersion: "monitoring.rhobs/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		Spec: monitoringv1.AlertmanagerSpec{
			Replicas: config.Replicas,
		},
	}
}

// createAlertmanagerServiceMonitors creates ServiceMonitors for alertmanager components
func createAlertmanagerServiceMonitors(namespace string) []runtime.Object {
	return []runtime.Object{
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "rhobs-alertmanager",
				Labels: map[string]string{
					"app.kubernetes.io/component": "alertmanager",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						HonorLabels: true,
						Interval:    "30s",
						Path:        "/metrics",
						Port:        "web",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "alertmanager",
					},
				},
			},
		},
	}
}
