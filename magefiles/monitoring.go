package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	"github.com/go-kit/log"
	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/configuration/clusters"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// MonitoringBundle collects ServiceMonitors from all build steps
type MonitoringBundle struct {
	config          clusters.ClusterConfig
	serviceMonitors []runtime.Object
}

// NewMonitoringBundle creates a new monitoring bundle for a cluster
func NewMonitoringBundle(config clusters.ClusterConfig) *MonitoringBundle {
	return &MonitoringBundle{
		config:          config,
		serviceMonitors: make([]runtime.Object, 0),
	}
}

// AddServiceMonitor adds a ServiceMonitor to the monitoring bundle
func (mb *MonitoringBundle) AddServiceMonitor(sm *monv1.ServiceMonitor) {
	if sm == nil {
		return
	}

	mb.serviceMonitors = append(mb.serviceMonitors, sm)
}

// Generate creates the monitoring bundle files.
func (mb *MonitoringBundle) Generate() error {
	if len(mb.serviceMonitors) == 0 {
		return nil // No ServiceMonitors to generate
	}

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	gen := &mimic.Generator{
		FilePool: mimic.FilePool{
			Logger: logger,
		},
	}
	gen = gen.With("resources", "clusters", string(mb.config.Environment), string(mb.config.Name), "monitoring")

	// Generate individual ServiceMonitor files as pure Kubernetes resources.
	for _, sm := range mb.serviceMonitors {
		if sm == nil {
			_ = logger.Log("msg", "found nil ServiceMonitor")
			continue
		}

		smObj := sm.(*monv1.ServiceMonitor)
		if smObj.Name == "" {
			panic("missing name for ServiceMonitor")
		}
		fileName := fmt.Sprintf("%s-ServiceMonitor.yaml", smObj.Name)

		// Process ServiceMonitor to ensure it has proper values (no template variables).
		processedSM := mb.processServiceMonitorForBundle(smObj)

		// Generate pure Kubernetes resource (no template wrapping).
		gen.Add(fileName, encoding.GhodssYAML(processedSM))
	}

	gen.Generate()
	return nil
}

// processServiceMonitorForBundle processes a ServiceMonitor to ensure it's a pure Kubernetes resource.
func (mb *MonitoringBundle) processServiceMonitorForBundle(sm *monv1.ServiceMonitor) runtime.Object {
	// Create a copy to avoid modifying the original
	processed := sm.DeepCopy()

	// Ensure that the namespace is set to the monitoring namespace.
	processed.Namespace = openshiftCustomerMonitoringNamespace

	// Ensure NamespaceSelector points to the actual cluster namespace (and not a template variable).
	if processed.Spec.NamespaceSelector.MatchNames != nil {
		for i, ns := range processed.Spec.NamespaceSelector.MatchNames {
			// Replace template variables with actual namespace
			if ns == "${NAMESPACE}" || ns == "" {
				processed.Spec.NamespaceSelector.MatchNames[i] = mb.config.Namespace
			}
		}
	} else {
		// Set namespace selector if not present
		processed.Spec.NamespaceSelector.MatchNames = []string{mb.config.Namespace}
	}

	// Add required prometheus label if not present.
	if processed.Labels == nil {
		processed.Labels = make(map[string]string)
	}
	processed.Labels[openshiftCustomerMonitoringLabel] = openShiftClusterMonitoringLabelValue

	return updateAPIGroup(processed, mb.config.MonitoringAPIGroup)
}

func updateAPIGroup(o runtime.Object, apiGroup clusters.MonitoringAPIGroup) runtime.Object {
	if apiGroup == "" {
		return o
	}

	// Update the API Group if required.
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		// It should never happen.
		panic(err)
	}

	unstructuredPtr := &unstructured.Unstructured{Object: unstructuredObj}
	apiGroupVersion := strings.Split(unstructuredPtr.GetAPIVersion(), "/")
	unstructuredPtr.SetAPIVersion(fmt.Sprintf("%s/%s", apiGroup, apiGroupVersion[1]))
	return unstructuredPtr
}

// Global monitoring bundle registry per cluster
var monitoringBundles = make(map[string]*MonitoringBundle)

// GetMonitoringBundle gets or creates a monitoring bundle for a cluster
func GetMonitoringBundle(config clusters.ClusterConfig) *MonitoringBundle {
	key := fmt.Sprintf("%s-%s", config.Environment, config.Name)
	if bundle, exists := monitoringBundles[key]; exists {
		return bundle
	}

	bundle := NewMonitoringBundle(config)
	monitoringBundles[key] = bundle
	return bundle
}

// GenerateAllMonitoringBundles generates all monitoring bundles that have been collected
func GenerateAllMonitoringBundles() error {
	for key, bundle := range monitoringBundles {
		if bundle == nil {
			continue
		}
		if err := bundle.Generate(); err != nil {
			return fmt.Errorf("failed to generate monitoring bundle for %s (%s): %w",
				key, bundle.config.Name, err)
		}
	}
	// Clear the registry after generation
	monitoringBundles = make(map[string]*MonitoringBundle)
	return nil
}
