package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rhobs/configuration/clusters"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	"github.com/observatorium/observatorium/configuration_go/kubegen/openshift"
	templatev1 "github.com/openshift/api/template/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	syntheticsApiTemplate = "synthetics-api-template.yaml"
	syntheticsApiName     = "synthetics-api"

	syntheticsApiPort = 8080

	defaultGatewaySyntheticsApiReplicas = 1

	// Synthetics Agent constants
	syntheticsAgentName            = "synthetics-agent"
	syntheticsAgentPort            = 8080
	defaultSyntheticsAgentReplicas = 1
)

// syntheticsApiConfig holds the configuration for synthetics API deployment
type syntheticsApiConfig struct {
	Name               string
	Namespace          string
	Flags              *syntheticsApiFlags
	Labels             map[string]string
	Replicas           int32
	SyntheticsApiImage string
}

type syntheticsApiFlags struct {
	// TODO: Define any additonal command line flags
}

// syntheticsAgentConfig holds the configuration for synthetics Agent deployment
type syntheticsAgentConfig struct {
	Name                 string
	Namespace            string
	Flags                *syntheticsAgentFlags
	Labels               map[string]string
	Replicas             int32
	SyntheticsAgentImage string
}

type syntheticsAgentFlags struct {
	// TODO: Define any additional command line flags
}

func (f *syntheticsApiFlags) ToArgs() []string {
	var args []string
	return args
}

func (f *syntheticsAgentFlags) ToArgs() []string {
	var args []string
	return args
}

// SyntheticsApi creates the syntheticsApi resources for the stage environment
func (s Stage) SyntheticsApi() {
	gen := func() *mimic.Generator {
		return s.generator(syntheticsApiName)
	}
	syntheticsApis := []*syntheticsApiConfig{
		newSyntheticsApiConfig(clusters.StageMaps, s.namespace()),
	}
	syntheticsApi(gen, clusters.StageMaps, syntheticsApis)
}

// SyntheticsApi creates the syntheticsApi resources for the production environment
func (p Production) SyntheticsApi() {
	gen := func() *mimic.Generator {
		return p.generator(syntheticsApiName)
	}
	syntheticsApis := []*syntheticsApiConfig{
		newSyntheticsApiConfig(clusters.ProductionMaps, p.namespace()),
	}
	syntheticsApi(gen, clusters.ProductionMaps, syntheticsApis)
}

func syntheticsApi(g func() *mimic.Generator, m clusters.TemplateMaps, confs []*syntheticsApiConfig) {
	var sms []runtime.Object
	var objs []runtime.Object

	for _, c := range confs {
		objs = append(objs, createBundleSyntheticsApiDeployment(c, m))
		objs = append(objs, createSyntheticsApiServiceAccount(c))
		objs = append(objs, createSyntheticsApiRole(c))
		objs = append(objs, createSyntheticsApiRoleBinding(c))
		objs = append(objs, createSyntheticsApiService(c))
		sms = append(sms, createSyntheticsApiServiceMonitor(c))
	}

	// Set template params
	params := []templatev1.Parameter{}
	params = append(params, templatev1.Parameter{
		Name:  "NAMESPACE",
		Value: "rhobs",
	}, templatev1.Parameter{
		Name:  "IMAGE_TAG",
		Value: "cea7d4656cd0ad338e580cc6ba266264a9938e5c",
	}, templatev1.Parameter{
		Name:  "IMAGE_DIGEST",
		Value: "",
	})

	template := openshift.WrapInTemplate(objs, metav1.ObjectMeta{
		Name: syntheticsApiName,
	}, sortTemplateParams(params))
	enc := encoding.GhodssYAML(template)
	gen := g()
	gen.Add(syntheticsApiTemplate, enc)
	gen.Generate()

	template = openshift.WrapInTemplate(sms, metav1.ObjectMeta{
		Name: syntheticsApiName + "-service-monitor",
	}, nil)
	gen = g()
	gen.Add("service-monitor-"+syntheticsApiTemplate, encoding.GhodssYAML(template))
	gen.Generate()
}

func newSyntheticsApiConfig(m clusters.TemplateMaps, namespace string) *syntheticsApiConfig {
	return &syntheticsApiConfig{
		Flags:              &syntheticsApiFlags{},
		Name:               syntheticsApiName,
		Namespace:          namespace,
		SyntheticsApiImage: m.Images[syntheticsAPI],
		Labels: map[string]string{
			"app.kubernetes.io/component": syntheticsApiName,
			"app.kubernetes.io/instance":  "rhobs",
			"app.kubernetes.io/name":      syntheticsApiName,
			"app.kubernetes.io/part-of":   "rhobs",
			"app.kubernetes.io/version":   m.Versions[syntheticsAPI],
		},
		Replicas: defaultGatewaySyntheticsApiReplicas,
	}
}

func createSyntheticsApiServiceAccount(config *syntheticsApiConfig) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
	}
}

// createSyntheticsApiRole creates a Role for synthetics API with configmaps permissions
func createSyntheticsApiRole(config *syntheticsApiConfig) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
}

// createSyntheticsApiRoleBinding creates a RoleBinding for synthetics API
func createSyntheticsApiRoleBinding(config *syntheticsApiConfig) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     config.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.Name,
				Namespace: config.Namespace,
			},
		},
	}
}

func createSyntheticsApiService(config *syntheticsApiConfig) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       syntheticsApiName,
					Port:       syntheticsApiPort,
					TargetPort: intstr.FromInt32(syntheticsApiPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: config.Labels,
		},
	}
}

func createSyntheticsApiServiceMonitor(config *syntheticsApiConfig) *monitoringv1.ServiceMonitor {
	labels := deepCopyMap(config.Labels)
	// Remove version label as it goes stale
	delete(labels, "app.kubernetes.io/version")
	labels[openshiftCustomerMonitoringLabel] = openShiftClusterMonitoringLabelValue

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: openshiftCustomerMonitoringNamespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:        "metrics",
					Path:        "/metrics",
					Interval:    monitoringv1.Duration("30s"),
					HonorLabels: true,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: createServiceSelectorLabels(config.Labels),
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{config.Namespace},
			},
		},
	}
}

func (b Build) SyntheticsApi(config clusters.ClusterConfig) {
	// For rhobss01ue1 and rhobsi01uw2 clusters, generate synthetics bundle with individual resources
	if config.Name == "rhobss01ue1" || config.Name == "rhobsi01uw2" {
		if err := generateSyntheticsBundle(config); err != nil {
			log.Printf("Error generating synthetics bundle: %v", err)
		}
		return
	}

	ns := config.Namespace
	gen := func() *mimic.Generator {
		return b.generator(config, syntheticsApiName)
	}
	syntheticsApis := []*syntheticsApiConfig{
		newSyntheticsApiConfig(clusters.ProductionMaps, ns),
	}
	syntheticsApi(gen, clusters.ProductionMaps, syntheticsApis)
}

// generateUnifiedSyntheticsApi generates a single, environment-agnostic template
func generateUnifiedSyntheticsApi() {
	var u Unified
	gen := func() *mimic.Generator {
		return u.generator(syntheticsApiName)
	}

	// Create a single config without environment-specific values
	config := &syntheticsApiConfig{
		Flags:              &syntheticsApiFlags{},
		Name:               syntheticsApiName,
		Namespace:          "", // Will be parameterized
		SyntheticsApiImage: "", // Not used since we use template parameter
		Labels: map[string]string{
			"app.kubernetes.io/component": syntheticsApiName,
			"app.kubernetes.io/instance":  "rhobs",
			"app.kubernetes.io/name":      syntheticsApiName,
			"app.kubernetes.io/part-of":   "rhobs",
			"app.kubernetes.io/version":   "${IMAGE_TAG}",
		},
		Replicas: defaultGatewaySyntheticsApiReplicas,
	}

	syntheticsApi(gen, clusters.TemplateMaps{}, []*syntheticsApiConfig{config})
}

// generateSyntheticsBundle generates individual synthetics component resources for bundle deployment
func generateSyntheticsBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With("resources", "clusters", string(config.Environment), string(config.Name), "synthetics", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// Create synthetics API resources with concrete values
	syntheticsConfig := newBundleSyntheticsApiConfig(config.Templates, ns)

	// Create synthetics Agent resources with concrete values
	syntheticsAgentConfig := newBundleSyntheticsAgentConfig(config.Templates, ns)

	// Generate individual synthetics resource files (API + Agent)
	syntheticsObjs := []runtime.Object{
		createBundleSyntheticsApiDeployment(syntheticsConfig, config.Templates),
		createSyntheticsApiServiceAccount(syntheticsConfig),
		createSyntheticsApiRole(syntheticsConfig),
		createSyntheticsApiRoleBinding(syntheticsConfig),
		createSyntheticsApiService(syntheticsConfig),
		createSyntheticsAgentDeployment(syntheticsAgentConfig, config.Templates),
		createSyntheticsAgentService(syntheticsAgentConfig),
		createSyntheticsAgentServiceAccount(syntheticsAgentConfig),
		createSyntheticsAgentClusterRole(),
		createSyntheticsAgentClusterRoleBinding(syntheticsAgentConfig),
		createSyntheticsAgentConfigMap(syntheticsAgentConfig),
	}

	for i, obj := range syntheticsObjs {
		resourceKind := getSyntheticsResourceKind(obj)
		resourceName := getSyntheticsResourceName(obj)
		filename := fmt.Sprintf("%02d-%s-%s.yaml", i+1, resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add consolidated ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	syntheticsServiceMonitors := createConsolidatedSyntheticsServiceMonitors(ns)

	for _, sm := range syntheticsServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	return nil
}

// newBundleSyntheticsApiConfig creates a synthetics API config with concrete values for bundle deployment
func newBundleSyntheticsApiConfig(m clusters.TemplateMaps, namespace string) *syntheticsApiConfig {
	return &syntheticsApiConfig{
		Flags:              &syntheticsApiFlags{},
		Name:               syntheticsApiName,
		Namespace:          namespace,
		SyntheticsApiImage: m.Images[clusters.SyntheticsAPI], // Use cluster parameter for image
		Labels: map[string]string{
			"app.kubernetes.io/component": syntheticsApiName,
			"app.kubernetes.io/instance":  "rhobs",
			"app.kubernetes.io/name":      syntheticsApiName,
			"app.kubernetes.io/part-of":   "rhobs",
			"app.kubernetes.io/version":   m.Versions[clusters.SyntheticsAPI], // Use cluster parameter for version
		},
		Replicas: defaultGatewaySyntheticsApiReplicas,
	}
}

// createBundleSyntheticsApiDeployment creates a Deployment with concrete values following the exact template
func createBundleSyntheticsApiDeployment(config *syntheticsApiConfig, m clusters.TemplateMaps) *appsv1.Deployment {
	labels := config.Labels

	syntheticsApiContainer := corev1.Container{
		Name:  syntheticsApiName,
		Image: config.SyntheticsApiImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          syntheticsApiName,
				ContainerPort: syntheticsApiPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "NAMESPACE",
				Value: config.Namespace,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/livez",
					Port: intstr.FromInt32(syntheticsApiPort),
				},
			},
			InitialDelaySeconds: 30,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.FromInt32(syntheticsApiPort),
				},
			},
			InitialDelaySeconds: 5,
		},
		Resources: corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{},
			Requests: corev1.ResourceList{},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		ImagePullPolicy:          corev1.PullIfNotPresent,
	}

	// Set resource limits exactly as in template
	syntheticsApiContainer.Resources.Limits[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	syntheticsApiContainer.Resources.Limits[corev1.ResourceMemory] = *resource.NewQuantity(2*1024*1024*1024, resource.BinarySI) // 2Gi
	syntheticsApiContainer.Resources.Requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(100, resource.DecimalSI)         // 100m
	syntheticsApiContainer.Resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(100*1024*1024, resource.BinarySI)  // 100Mi

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &config.Replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: config.Name,
					Containers: []corev1.Container{
						syntheticsApiContainer,
					},
				},
			},
		},
	}
	return deployment
}

// getSyntheticsResourceKind returns the Kind field from a Kubernetes object for synthetics resources
func getSyntheticsResourceKind(obj runtime.Object) string {
	switch obj.(type) {
	case *appsv1.StatefulSet:
		return "StatefulSet"
	case *appsv1.Deployment:
		return "Deployment"
	case *corev1.Service:
		return "Service"
	case *corev1.ServiceAccount:
		return "ServiceAccount"
	case *corev1.ConfigMap:
		return "ConfigMap"
	case *rbacv1.Role:
		return "Role"
	case *rbacv1.RoleBinding:
		return "RoleBinding"
	case *rbacv1.ClusterRole:
		return "ClusterRole"
	case *rbacv1.ClusterRoleBinding:
		return "ClusterRoleBinding"
	default:
		// Try to get the kind from TypeMeta as a fallback
		if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Kind != "" {
			return gvk.Kind
		}
		// If TypeMeta doesn't have Kind, try to infer from type name
		objType := fmt.Sprintf("%T", obj)
		if objType != "" && len(objType) > 1 {
			// Extract the last part after the dot and asterisk (e.g., "*v1.StatefulSet" -> "StatefulSet")
			parts := strings.Split(objType, ".")
			if len(parts) > 0 {
				typeName := parts[len(parts)-1]
				return typeName
			}
		}
		return "Unknown"
	}
}

// getSyntheticsResourceName extracts a meaningful name from a synthetics Kubernetes object
func getSyntheticsResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	switch o := obj.(type) {
	case metav1.Object:
		name := o.GetName()
		if name != "" {
			return name
		}
	}

	// Fallback to the object type
	return "unnamed"
}

// createConsolidatedSyntheticsServiceMonitors creates ServiceMonitors for synthetics components
func createConsolidatedSyntheticsServiceMonitors(namespace string) []runtime.Object {
	const openshiftCustomerMonitoringNamespace = "openshift-customer-monitoring"

	return []runtime.Object{
		// Synthetics API
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "synthetics-api",
				Namespace: openshiftCustomerMonitoringNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component": "synthetics-api",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						HonorLabels: true,
						Interval:    "30s",
						Path:        "/metrics",
						Port:        "synthetics-api",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "synthetics-api",
					},
				},
			},
		},
		// Synthetics Agent
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "synthetics-agent",
				Namespace: openshiftCustomerMonitoringNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component": "synthetics-agent",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						HonorLabels: true,
						Interval:    "30s",
						Path:        "/metrics",
						Port:        "http-metrics",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "synthetics-agent",
					},
				},
			},
		},
		// Blackbox Exporter
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "synthetics-bb-exporter",
				Namespace: openshiftCustomerMonitoringNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "blackbox-exporter",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						HonorLabels: true,
						Interval:    "30s",
						Path:        "/metrics",
						Port:        "http",
					},
				},
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{namespace},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "blackbox-exporter",
					},
				},
			},
		},
	}
}

// newBundleSyntheticsAgentConfig creates a synthetics agent config with concrete values for bundle deployment
func newBundleSyntheticsAgentConfig(m clusters.TemplateMaps, namespace string) *syntheticsAgentConfig {
	return &syntheticsAgentConfig{
		Flags:                &syntheticsAgentFlags{},
		Name:                 syntheticsAgentName,
		Namespace:            namespace,
		SyntheticsAgentImage: "quay.io/redhat-services-prod/openshift/rhobs-synthetics-agent:3012046",
		Labels: map[string]string{
			"app.kubernetes.io/component": syntheticsAgentName,
			"app.kubernetes.io/instance":  "rhobs",
			"app.kubernetes.io/name":      syntheticsAgentName,
			"app.kubernetes.io/part-of":   "rhobs",
			"app.kubernetes.io/version":   "3012046",
		},
		Replicas: defaultSyntheticsAgentReplicas,
	}
}

// createSyntheticsAgentDeployment creates a Deployment for the synthetics agent following the exact template
func createSyntheticsAgentDeployment(config *syntheticsAgentConfig, m clusters.TemplateMaps) *appsv1.Deployment {
	labels := config.Labels

	syntheticsAgentContainer := corev1.Container{
		Name:    syntheticsAgentName,
		Image:   config.SyntheticsAgentImage,
		Command: []string{"./rhobs-synthetics-agent"},
		Args:    []string{"start", "--config", "/etc/config/config.yaml"},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http-metrics",
				ContainerPort: syntheticsAgentPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config",
				MountPath: "/etc/config",
			},
		},
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &config.Replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: config.Name,
					Containers: []corev1.Container{
						syntheticsAgentContainer,
					},
					SecurityContext: &corev1.PodSecurityContext{},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: config.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

// createSyntheticsAgentService creates a Service for the synthetics agent
func createSyntheticsAgentService(config *syntheticsAgentConfig) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Port:       syntheticsAgentPort,
					TargetPort: intstr.FromInt32(syntheticsAgentPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: config.Labels,
		},
	}
}

// createSyntheticsAgentServiceAccount creates a ServiceAccount for the synthetics agent
func createSyntheticsAgentServiceAccount(config *syntheticsAgentConfig) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
	}
}

// createSyntheticsAgentClusterRole creates a ClusterRole for the synthetics agent
func createSyntheticsAgentClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: syntheticsAgentName,
			Labels: map[string]string{
				"app.kubernetes.io/component": syntheticsAgentName,
				"app.kubernetes.io/instance":  "rhobs",
				"app.kubernetes.io/name":      syntheticsAgentName,
				"app.kubernetes.io/part-of":   "rhobs",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"monitoring.rhobs"},
				Resources: []string{"probes"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"probes"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

// createSyntheticsAgentClusterRoleBinding creates a ClusterRoleBinding for the synthetics agent
func createSyntheticsAgentClusterRoleBinding(config *syntheticsAgentConfig) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: syntheticsAgentName,
			Labels: map[string]string{
				"app.kubernetes.io/component": syntheticsAgentName,
				"app.kubernetes.io/instance":  "rhobs",
				"app.kubernetes.io/name":      syntheticsAgentName,
				"app.kubernetes.io/part-of":   "rhobs",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     syntheticsAgentName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      config.Name,
				Namespace: config.Namespace,
			},
		},
	}
}

// createSyntheticsAgentConfigMap creates a ConfigMap for the synthetics agent following the exact template
func createSyntheticsAgentConfigMap(config *syntheticsAgentConfig) *corev1.ConfigMap {
	// Build the proper API URL with full cluster DNS name and correct port
	apiURL := fmt.Sprintf("http://synthetics-api.%s.svc.cluster.local:8080/probes", config.Namespace)

	configData := "# Logging configuration\n" +
		"log_level: info\n" +
		"log_format: json\n\n" +
		"# Polling configuration\n" +
		"polling_interval: 30s\n" +
		"graceful_timeout: 30s\n\n" +
		"# API Configuration\n" +
		"api_urls: " + apiURL + "\n" +
		"label_selector: private=false\n\n" +
		"# Kubernetes Configuration\n" +
		"namespace: " + config.Namespace + "\n\n" +
		"# Blackbox Configuration\n" +
		"blackbox:\n" +
		"  interval: 30s\n" +
		"  module: http_2xx\n" +
		"  prober_url: synthetics-blackbox-prober-default-service:9115"

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels:    config.Labels,
		},
		Data: map[string]string{
			"config.yaml": configData,
		},
	}
}
