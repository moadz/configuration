package main

import (
	"fmt"
	"log"
	"maps"
	"os"
	"time"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	"github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/alertmanager"
	kghelpers "github.com/observatorium/observatorium/configuration_go/kubegen/helpers"
	"github.com/observatorium/observatorium/configuration_go/kubegen/openshift"
	"github.com/observatorium/observatorium/configuration_go/kubegen/workload"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	monv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/configuration/clusters"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	alertManagerName     = "alertmanager"
	alertmanagerTemplate = "alertmanager-template.yaml"

	defaultAlertmanagerReplicas = 2

	defaultAlertManagerImage     = "registry.redhat.io/openshift4/ose-prometheus-alertmanager"
	defaultAlertManagerImageTag  = "v4.15"
	alertmanagerConfigSecretName = "alertmanager-config"
	alertmanagerConfigSecretKey  = "alertmanager.yaml"
	alertmanagerTLSSecret        = "alertmanager-tls"

	defaultAlertmanagerCPURequest    = "100m"
	defaultAlertmanagerCPULimit      = "5"
	defaultAlertmanagerMemoryRequest = "256Mi"
	defaultAlertmanagerMemoryLimit   = "5Gi"
)

func (b Build) Alertmanager(config clusters.ClusterConfig) {
	// For migrated clusters, generate alertmanager bundle with individual resources
	if isMigratedCluster(config) {
		if err := generateAlertmanagerBundleFromTemplate(config); err != nil {
			log.Printf("Error generating alertmanager bundle: %v", err)
		}
		return
	}

	gen := b.generator(config, "alertmanager")

	// TODO: @moadz Extract Alertmanager options to an envTemplate in template.go
	k8s := alertmanagerKubernetes(alertManagerOptions(), manifestOptions{
		namespace: config.Namespace,
		image:     defaultAlertManagerImage,
		imageTag:  defaultAlertManagerImageTag,
		resourceRequirements: resourceRequirements{
			cpuRequest:    defaultAlertmanagerCPURequest,
			cpuLimit:      defaultAlertmanagerCPULimit,
			memoryRequest: defaultAlertmanagerMemoryRequest,
			memoryLimit:   defaultAlertmanagerMemoryLimit,
		},
	})
	buildAlertmanager(k8s.Objects(), config.Namespace, gen)
}

// Alertmanager Generates the Alertmanager configuration for the stage environment.
func (s Stage) Alertmanager() {
	gen := s.generator(alertManagerName)

	const (
		alertManagerImageTag = defaultAlertManagerImageTag

		cpuRequest = defaultAlertmanagerCPURequest
		cpuLimit   = defaultAlertmanagerCPULimit
		memRequest = defaultAlertmanagerMemoryRequest
		memLimit   = defaultAlertmanagerMemoryLimit
	)

	k8s := alertmanagerKubernetes(alertManagerOptions(), manifestOptions{
		namespace: s.namespace(),
		image:     defaultAlertManagerImage,
		imageTag:  alertManagerImageTag,
		resourceRequirements: resourceRequirements{
			cpuRequest:    cpuRequest,
			cpuLimit:      cpuLimit,
			memoryRequest: memRequest,
			memoryLimit:   memLimit,
		},
	})
	buildAlertmanager(k8s.Objects(), s.namespace(), gen)
}

// Alertmanager Generates the Alertmanager configuration for the production environment.
func (p Production) Alertmanager() {
	gen := p.generator(alertManagerName)

	const (
		alertManagerImageTag = defaultAlertManagerImageTag

		cpuRequest = defaultAlertmanagerCPURequest
		cpuLimit   = defaultAlertmanagerCPULimit
		memRequest = defaultAlertmanagerMemoryRequest
		memLimit   = defaultAlertmanagerMemoryLimit
	)

	k8s := alertmanagerKubernetes(alertManagerOptions(), manifestOptions{
		namespace: p.namespace(),
		image:     defaultAlertManagerImage,
		imageTag:  alertManagerImageTag,
		resourceRequirements: resourceRequirements{
			cpuRequest:    cpuRequest,
			cpuLimit:      cpuLimit,
			memoryRequest: memRequest,
			memoryLimit:   memLimit,
		},
	})
	buildAlertmanager(k8s.Objects(), p.namespace(), gen)
}

func buildAlertmanager(manifests []runtime.Object, namespace string, generator *mimic.Generator) {
	var sm *monv1.ServiceMonitor
	sm, manifests = getAndRemoveObject[*monv1.ServiceMonitor](manifests, "")
	smEnc := postProcessServiceMonitor(sm, namespace)
	enc := alertmanagerPostProcess(manifests, namespace)
	generator.Add(alertmanagerTemplate, enc)
	generator.Add(serviceMonitorTemplate, smEnc)
	generator.Generate()
}

func alertManagerOptions() *alertmanager.AlertManagerOptions {
	o := alertmanager.NewDefaultOptions()
	o.ConfigFile = alertmanager.NewConfigFile(nil).
		WithExistingResource(alertmanagerConfigSecretName, alertmanagerConfigSecretKey).AsSecret()
	o.ClusterReconnectTimeout = 5 * time.Minute
	return o
}

func alertmanagerKubernetes(opts *alertmanager.AlertManagerOptions, options manifestOptions) *alertmanager.AlertManagerStatefulSet {
	namespace := options.namespace
	alertmanSts := alertmanager.NewAlertManager(opts, namespace, options.imageTag)
	alertmanSts.Image = options.image
	alertmanSts.Replicas = defaultAlertmanagerReplicas
	alertmanSts.Name = alertManagerName
	alertmanSts.VolumeSize = "1Gi"
	alertmanSts.VolumeType = "gp2"
	alertmanSts.ContainerResources = kghelpers.NewResourcesRequirements(options.cpuRequest, options.cpuLimit, options.memoryRequest, options.memoryLimit)
	alertmanSts.Sidecars = []workload.ContainerProvider{
		makeOauthProxy(9093, namespace, alertmanSts.Name, alertmanagerTLSSecret),
	}

	headlessServiceName := alertmanSts.Name + "-cluster"
	if alertmanSts.Replicas > 1 {
		for i := 0; i < int(alertmanSts.Replicas); i++ {
			opts.ClusterPeer = append(opts.ClusterPeer,
				fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:9094", alertmanSts.Name, i, headlessServiceName, namespace))
		}
	}
	return alertmanSts
}

func alertmanagerPostProcess(manifests []runtime.Object, namespace string) encoding.Encoder {
	service := kghelpers.GetObject[*corev1.Service](manifests, alertManagerName)
	service.ObjectMeta.Annotations[servingCertSecretNameAnnotation] = alertmanagerTLSSecret
	service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
		Name:       "https",
		Port:       8443,
		TargetPort: intstr.FromInt32(8443),
	})
	// Add annotations for openshift oauth so that the route to access the query ui works
	serviceAccount := kghelpers.GetObject[*corev1.ServiceAccount](manifests, "")
	if serviceAccount.Annotations == nil {
		serviceAccount.Annotations = map[string]string{}
	}
	serviceAccount.Annotations[serviceRedirectAnnotation] = fmt.Sprintf(`{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"%s"}}`, alertManagerName)

	// Add route for oauth-proxy
	manifests = append(manifests, &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertManagerName,
			Namespace: namespace,
			Labels:    maps.Clone(kghelpers.GetObject[*appsv1.StatefulSet](manifests, "").ObjectMeta.Labels),
			Annotations: map[string]string{
				"cert-manager.io/issuer-kind": "ClusterIssuer",
				"cert-manager.io/issuer-name": "letsencrypt-prod-http",
			},
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: alertManagerName,
			},
		},
	})

	return encoding.GhodssYAML(
		openshift.WrapInTemplate(
			manifests,
			metav1.ObjectMeta{Name: alertManagerName},
			[]templatev1.Parameter{
				{
					Name:     "OAUTH_PROXY_COOKIE_SECRET",
					Generate: "expression",
					From:     `[a-zA-Z0-9]{40}`,
				},
			},
		))
}

// generateAlertmanagerBundleFromTemplate generates individual alertmanager component resources for bundle deployment
// This function reuses the existing template-based alertmanager generation but outputs individual files instead
func generateAlertmanagerBundleFromTemplate(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With("resources", "clusters", string(config.Environment), string(config.Name), "alertmanager", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// Generate alertmanager objects using existing template logic
	k8s := alertmanagerKubernetes(alertManagerOptions(), manifestOptions{
		namespace: ns,
		image:     defaultAlertManagerImage,
		imageTag:  defaultAlertManagerImageTag,
		resourceRequirements: resourceRequirements{
			cpuRequest:    defaultAlertmanagerCPURequest,
			cpuLimit:      defaultAlertmanagerCPULimit,
			memoryRequest: defaultAlertmanagerMemoryRequest,
			memoryLimit:   defaultAlertmanagerMemoryLimit,
		},
	})

	manifests := k8s.Objects()

	// Extract and handle ServiceMonitor separately
	var sm *monv1.ServiceMonitor
	sm, manifests = getAndRemoveObject[*monv1.ServiceMonitor](manifests, "")

	// Generate individual resource files for each alertmanager component
	for i, obj := range manifests {
		resourceKind := getResourceKind(obj)
		resourceName := getKubernetesResourceName(obj)
		filename := fmt.Sprintf("%02d-%s-%s.yaml", i+1, resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add ServiceMonitor to monitoring bundle if it exists
	if sm != nil {
		monBundle := GetMonitoringBundle(config)
		monBundle.AddServiceMonitor(sm)
	}

	return nil
}
