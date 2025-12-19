package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	kghelpers "github.com/observatorium/observatorium/configuration_go/kubegen/helpers"
	"github.com/observatorium/observatorium/configuration_go/kubegen/openshift"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/configuration/clusters"
	"github.com/thanos-community/thanos-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func (b Build) DefaultThanosStack(config clusters.ClusterConfig) {
	// For rhobss01ue1 and rhobsi01uw2 clusters, generate metrics bundle with individual resources
	if config.Name == "rhobss01ue1" || config.Name == "rhobsi01uw2" {
		if err := generateMetricsBundle(config); err != nil {
			log.Printf("Error generating metrics bundle: %v", err)
		}
		return
	}

	gen := b.generator(config, "thanos-operator-default-cr")
	var objs []runtime.Object

	objs = append(objs, defaultQueryCR(config.Namespace, config.Templates, true)...)
	objs = append(objs, defaultReceiveCR(config.Namespace, config.Templates))
	objs = append(objs, defaultCompactCR(config.Namespace, config.Templates, true)...)
	objs = append(objs, defaultRulerCR(config.Namespace, config.Templates))
	objs = append(objs, defaultStoreCR(config.Namespace, config.Templates))

	// Sort objects by Kind then Name
	sort.Slice(objs, func(i, j int) bool {
		iMeta := objs[i].(metav1.Object)
		jMeta := objs[j].(metav1.Object)
		iType := objs[i].GetObjectKind().GroupVersionKind().Kind
		jType := objs[j].GetObjectKind().GroupVersionKind().Kind

		if iType != jType {
			return iType < jType
		}
		return iMeta.GetName() < jMeta.GetName()
	})

	gen.Add("thanos-operator-default-cr.yaml", encoding.GhodssYAML(
		openshift.WrapInTemplate(
			objs,
			metav1.ObjectMeta{Name: "thanos-rhobs"},
			[]templatev1.Parameter{
				{
					Name:     "OAUTH_PROXY_COOKIE_SECRET",
					Generate: "expression",
					From:     `[a-zA-Z0-9]{40}`,
				},
			},
		),
	))

	gen.Generate()
}

// Thanos Generates the RHOBS-specific CRs for Thanos Operator.
func (p Production) Thanos() {
	templateDir := "rhobs-thanos-operator"

	gen := p.generator(templateDir)
	ns := p.namespace()
	var objs []runtime.Object

	tmpAdditionalQueryArgs := []string{
		`--endpoint=dnssrv+_grpc._tcp.observatorium-thanos-rule.observatorium-metrics-production.svc.cluster.local`,
		`--endpoint=dnssrv+_grpc._tcp.observatorium-thanos-receive-default.observatorium-metrics-production.svc.cluster.local`,
	}

	objs = append(objs, queryCR(ns, clusters.ProductionMaps, true, tmpAdditionalQueryArgs...)...)
	objs = append(objs, tmpStoreProduction(ns, clusters.ProductionMaps)...)
	objs = append(objs, compactTempProduction(clusters.ProductionMaps)...)
	// objs = append(objs, tmpRulerCR(ns, clusters.ProductionMaps))

	// Sort objects by Kind then Name
	sort.Slice(objs, func(i, j int) bool {
		iMeta := objs[i].(metav1.Object)
		jMeta := objs[j].(metav1.Object)
		iType := objs[i].GetObjectKind().GroupVersionKind().Kind
		jType := objs[j].GetObjectKind().GroupVersionKind().Kind

		if iType != jType {
			return iType < jType
		}
		return iMeta.GetName() < jMeta.GetName()
	})

	gen.Add("rhobs.yaml", encoding.GhodssYAML(
		openshift.WrapInTemplate(
			objs,
			metav1.ObjectMeta{Name: "thanos-rhobs"},
			[]templatev1.Parameter{
				{
					Name:     "OAUTH_PROXY_COOKIE_SECRET",
					Generate: "expression",
					From:     `[a-zA-Z0-9]{40}`,
				},
			},
		),
	))

	gen.Generate()
}

// Thanos Generates the RHOBS-specific CRs for Thanos Operator.
func (s Stage) Thanos() {
	templateDir := "rhobs-thanos-operator"

	gen := s.generator(templateDir)
	tmpAdditionalQueryArgs := []string{
		`--endpoint=dnssrv+_grpc._tcp.observatorium-thanos-receive-default.observatorium-metrics-stage.svc.cluster.local`,
	}
	var objs []runtime.Object

	objs = append(objs, receiveCR(s.namespace(), clusters.StageMaps))
	objs = append(objs, queryCR(s.namespace(), clusters.StageMaps, true, tmpAdditionalQueryArgs...)...)
	objs = append(objs, rulerCR(s.namespace(), clusters.StageMaps)...)
	// TODO: Add compact CRs for stage once we shut down previous
	// objs = append(objs, compactCR(s.namespace(), templates, true)...)
	objs = append(objs, stageCompactCR(s.namespace(), clusters.StageMaps)...)
	objs = append(objs, storeCR(s.namespace(), clusters.StageMaps)...)

	// Sort objects by Kind then Name
	sort.Slice(objs, func(i, j int) bool {
		iMeta := objs[i].(metav1.Object)
		jMeta := objs[j].(metav1.Object)
		iType := objs[i].GetObjectKind().GroupVersionKind().Kind
		jType := objs[j].GetObjectKind().GroupVersionKind().Kind

		if iType != jType {
			return iType < jType
		}
		return iMeta.GetName() < jMeta.GetName()
	})

	gen.Add("rhobs.yaml", encoding.GhodssYAML(
		openshift.WrapInTemplate(
			objs,
			metav1.ObjectMeta{Name: "thanos-rhobs"},
			[]templatev1.Parameter{
				{
					Name:     "OAUTH_PROXY_COOKIE_SECRET",
					Generate: "expression",
					From:     `[a-zA-Z0-9]{40}`,
				},
			},
		),
	))

	gen.Generate()
}

func storeCR(namespace string, m clusters.TemplateMaps) []runtime.Object {
	store0to2w := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-0to2w",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("STORE02W", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE02W", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE02W", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE02W", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE02W", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 627040000,
				StoreLimitsRequestSeries:  1000000,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-2w")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE02W", m.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	store2wto90d := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-2wto90d",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("STORE2W90D", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE2W90D", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE2W90D", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE2W90D", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE2W90D", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 627040000,
				StoreLimitsRequestSeries:  1000000,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-90d")),
				MaxTime: ptr.To(v1alpha1.Duration("-2w")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE2W90D", m.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
				PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	store90dplus := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-90dplus",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("STORE90D+", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE90D+", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE90D+", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE90D+", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE90D+", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 627040000,
				StoreLimitsRequestSeries:  1000000,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-90d")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE90D+", m.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	// RHOBS-904: Standalone Store for RH Resource Optimisation (ROS) Managed Service
	storeRos := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ros",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("STORE_ROS", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE_ROS", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE_ROS", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE_ROS", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE_ROS", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("ROS", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE_ROS", m.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	storeDefault := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE_DEFAULT", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("DEFAULT", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-22h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE_DEFAULT", m.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	objs := []runtime.Object{store0to2w, store2wto90d, store90dplus, storeDefault}

	//TODO @moadz RHOBS-904: Temporary block, only return in stage
	if clusters.TemplateFn("STORE_ROS", m.Replicas) > 0 {
		objs = append(objs, storeRos)
	}

	return objs
}

func tmpStoreProduction(namespace string, m clusters.TemplateMaps) []runtime.Object {
	iC := `--index-cache.config="config":
  "addresses":
    - "dnssrv+_client._tcp.thanos-index-cache.rhobs-production.svc"
  "dns_provider_update_interval": "30s"
  "max_async_buffer_size": 50000000
  "max_async_concurrency": 1000
  "max_get_multi_batch_size": 1000
  "max_get_multi_concurrency": 100
  "max_idle_connections": 500
  "max_item_size": "1000MiB"
  "timeout": "5s"
"type": "memcached"`

	inMem := `--index-cache.config="config":
  "max_size": "10000MB"
  "max_item_size": "1000MB"
"type": "IN-MEMORY"`

	bc := `--store.caching-bucket.config=
  "type": "memcached"
  "blocks_iter_ttl": "10m"
  "chunk_object_attrs_ttl": "48h"
  "chunk_subrange_size": 16000
  "chunk_subrange_ttl": "48h"
  "metafile_content_ttl": "48h"
  "metafile_doesnt_exist_ttl": "30m"
  "metafile_exists_ttl": "24h"
  "metafile_max_size": "20MiB"
  "max_chunks_get_range_requests": 5
  "config":
    "addresses":
      - "dnssrv+_client._tcp.thanos-bucket-cache.rhobs-production.svc"
    "dns_provider_update_interval": "30s"
    "max_async_buffer_size": 1000000
    "max_async_concurrency": 100
    "max_get_multi_batch_size": 500
    "max_get_multi_concurrency": 100
    "max_idle_connections": 500
    "max_item_size": "500MiB"
    "timeout": "5s"`

	additionalCacheArgs := v1alpha1.Additional{
		Args: []string{
			inMem,
		},
	}

	zero2wArgs := v1alpha1.Additional{
		Args: []string{
			bc,
			iC,
		},
	}

	store0to2w := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-0to2w",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			Additional: zero2wArgs,
			CommonFields: v1alpha1.CommonFields{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "workload-type",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"query"},
										},
									},
								},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app.kubernetes.io/instance",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"telemeter-0to2w"},
										},
									},
								},
							},
						},
					},
				},
				Image:                ptr.To(clusters.TemplateFn("STORE02W", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE02W", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE02W", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE02W", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE02W", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("12h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-336h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE02W", m.StorageSize),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	store2wto90d := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-2wto90d",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			Additional: additionalCacheArgs,
			CommonFields: v1alpha1.CommonFields{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "workload-type",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"query"},
										},
									},
								},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app.kubernetes.io/instance",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"telemeter-2wto90d"},
										},
									},
								},
							},
						},
					},
				},
				Image:                ptr.To(clusters.TemplateFn("STORE2W90D", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE2W90D", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE2W90D", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE2W90D", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE2W90D", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-2160h")),
				MaxTime: ptr.To(v1alpha1.Duration("-336h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE2W90D", m.StorageSize),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
				PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	store90dplus := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter-90dplus",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			Additional: additionalCacheArgs,
			CommonFields: v1alpha1.CommonFields{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "workload-type",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"query"},
										},
									},
								},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app.kubernetes.io/instance",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"telemeter-90dplus"},
										},
									},
								},
							},
						},
					},
				},
				Image:                ptr.To(clusters.TemplateFn("STORE90D+", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE90D+", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE90D+", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE90D+", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE90D+", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-8760h")),
				MaxTime: ptr.To(v1alpha1.Duration("-2160h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE90D+", m.StorageSize),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	storeDefault := &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			Additional: additionalCacheArgs,
			CommonFields: v1alpha1.CommonFields{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "workload-type",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"query"},
										},
									},
								},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app.kubernetes.io/instance",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"default"},
										},
									},
								},
							},
						},
					},
				},
				Image:                ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.Images)),
				Version:              ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("STORE_DEFAULT", m.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn("STORE_DEFAULT", m.Replicas),
			ObjectStorageConfig: clusters.TemplateFn("DEFAULT", m.ObjectStorageBucket),
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-22h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("STORE_DEFAULT", m.StorageSize),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
	return []runtime.Object{store0to2w, store2wto90d, store90dplus, storeDefault}
}

func TmpRulerCR(namespace string, templates clusters.TemplateMaps) *v1alpha1.ThanosRuler {
	return &v1alpha1.ThanosRuler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosRuler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("RULER", templates.Images)),
				Version:              ptr.To(clusters.TemplateFn("RULER", templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("RULER", templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("RULER", templates.ResourceRequirements)),
			},
			Replicas: clusters.TemplateFn("RULER", templates.Replicas),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("RULER", templates.StorageSize),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
			RuleConfigSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/rule-file": "true",
				},
			},
			PrometheusRuleSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/prometheus-rule": "true",
				},
			},
			QueryLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/query-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
				TenantLabel:      "tenant_id",
				TenantValueLabel: "operator.thanos.io/tenant",
			},
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", templates.ObjectStorageBucket),
			ExternalLabels: map[string]string{
				"rule_replica": "$(NAME)",
			},
			AlertmanagerURL:    "dnssrv+http://alertmanager-cluster." + namespace + ".svc.cluster.local:9093",
			AlertLabelDrop:     []string{"rule_replica"},
			Retention:          v1alpha1.Duration("2h"),
			EvaluationInterval: v1alpha1.Duration("1m"),
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
		},
	}
}

func receiveCR(namespace string, templates clusters.TemplateMaps) *v1alpha1.ThanosReceive {
	return &v1alpha1.ThanosReceive{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosReceive",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosReceiveSpec{
			Router: v1alpha1.RouterSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn("RECEIVE_ROUTER", templates.Images)),
					Version:              ptr.To(clusters.TemplateFn("RECEIVE_ROUTER", templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn("RECEIVE_ROUTER", templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn("RECEIVE_ROUTER", templates.ResourceRequirements)),
				},
				Replicas:          clusters.TemplateFn("RECEIVE_ROUTER", templates.Replicas),
				ReplicationFactor: 3,
				ExternalLabels: map[string]string{
					"receive": "true",
				},
				Additional: v1alpha1.Additional{
					Args: []string{},
				},
			},
			Ingester: v1alpha1.IngesterSpec{
				DefaultObjectStorageConfig: clusters.TemplateFn("TELEMETER", templates.ObjectStorageBucket),
				Additional: v1alpha1.Additional{
					Args: []string{},
				},
				Hashrings: []v1alpha1.IngesterHashringSpec{
					{
						Name: "telemeter",
						CommonFields: v1alpha1.CommonFields{
							Image:                ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_TELEMETER", templates.Images)),
							Version:              ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_TELEMETER", templates.Versions)),
							ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
							LogLevel:             ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_TELEMETER", templates.LogLevels)),
							LogFormat:            ptr.To("logfmt"),
							ResourceRequirements: ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_TELEMETER", templates.ResourceRequirements)),
						},
						ExternalLabels: map[string]string{
							"replica": "$(POD_NAME)",
						},
						Replicas: clusters.TemplateFn("RECEIVE_INGESTOR_TELEMETER", templates.Replicas),
						TSDBConfig: v1alpha1.TSDBConfig{
							Retention: v1alpha1.Duration("4h"),
						},
						AsyncForwardWorkerCount:  ptr.To(uint64(50)),
						TooFarInFutureTimeWindow: ptr.To(v1alpha1.Duration("5m")),
						StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
							StoreLimitsRequestSamples: 627040000,
							StoreLimitsRequestSeries:  1000000,
						},
						TenancyConfig: &v1alpha1.TenancyConfig{
							TenantMatcherType: "exact",
							DefaultTenantID:   "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
							TenantHeader:      "THANOS-TENANT",
							TenantLabelName:   "tenant_id",
						},
						StorageConfiguration: v1alpha1.StorageConfiguration{
							Size: clusters.TemplateFn("RECEIVE_TELEMETER", templates.StorageSize),
						},
					},
					{
						Name: "default",
						CommonFields: v1alpha1.CommonFields{
							Image:                ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_DEFAULT", templates.Images)),
							Version:              ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_DEFAULT", templates.Versions)),
							ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
							LogLevel:             ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_DEFAULT", templates.LogLevels)),
							LogFormat:            ptr.To("logfmt"),
							ResourceRequirements: ptr.To(clusters.TemplateFn("RECEIVE_INGESTOR_DEFAULT", templates.ResourceRequirements)),
						},
						ExternalLabels: map[string]string{
							"replica": "$(POD_NAME)",
						},
						Replicas: clusters.TemplateFn("RECEIVE_INGESTOR_DEFAULT", templates.Replicas),
						TSDBConfig: v1alpha1.TSDBConfig{
							Retention: v1alpha1.Duration("1d"),
						},
						AsyncForwardWorkerCount:  ptr.To(uint64(5)),
						TooFarInFutureTimeWindow: ptr.To(v1alpha1.Duration("5m")),
						StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
							StoreLimitsRequestSamples: 0,
							StoreLimitsRequestSeries:  0,
						},
						TenancyConfig: &v1alpha1.TenancyConfig{
							TenantMatcherType: "exact",
							DefaultTenantID:   "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
							TenantHeader:      "THANOS-TENANT",
							TenantLabelName:   "tenant_id",
						},
						ObjectStorageConfig: ptr.To(clusters.TemplateFn("DEFAULT", templates.ObjectStorageBucket)),
						StorageConfiguration: v1alpha1.StorageConfiguration{
							Size: clusters.TemplateFn("RECEIVE_DEFAULT", templates.StorageSize),
						},
					},
				},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
}

func defaultQueryCR(namespace string, templates clusters.TemplateMaps, oauth bool, withAdditionalArgs ...string) []runtime.Object {
	var objs []runtime.Object

	query := &v1alpha1.ThanosQuery{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosQuery",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosQuerySpec{
			Additional: v1alpha1.Additional{
				Args: withAdditionalArgs,
			},
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.Query, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.Query, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.Query, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.Query, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			StoreLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/store-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			Replicas: clusters.TemplateFn(clusters.Query, templates.Replicas),
			ReplicaLabels: []string{
				"prometheus_replica",
				"replica",
				"rule_replica",
			},
			WebConfig: &v1alpha1.WebConfig{
				PrefixHeader: ptr.To("X-Forwarded-Prefix"),
			},
			GRPCProxyStrategy: "lazy",
			TelemetryQuantiles: &v1alpha1.TelemetryQuantiles{
				Duration: []string{
					"0.1", "0.25", "0.75", "1.25", "1.75", "2.5", "3", "5", "10", "15", "30", "60", "120",
				},
			},
			QueryFrontend: &v1alpha1.QueryFrontendSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Images)),
					Version:              ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.ResourceRequirements)),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
				Replicas:             clusters.TemplateFn("QUERY_FRONTEND", templates.Replicas),
				CompressResponses:    true,
				LogQueriesLongerThan: ptr.To(v1alpha1.Duration("10s")),
				LabelsMaxRetries:     3,
				QueryRangeMaxRetries: 3,
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/query-api": "true",
					},
				},
				QueryRangeSplitInterval: ptr.To(v1alpha1.Duration("48h")),
				LabelsSplitInterval:     ptr.To(v1alpha1.Duration("48h")),
				LabelsDefaultTimeRange:  ptr.To(v1alpha1.Duration("336h")),
				QueryRangeResponseCacheConfig: &v1alpha1.CacheConfig{
					ExternalCacheConfig: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-query-range-cache-memcached",
						},
						Key: "thanos.yaml",
					},
				},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
				PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
	if oauth {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "route.openshift.io/v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thanos-query-frontend-rhobs",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "thanos",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "thanos-query-frontend-rhobs",
					Weight: ptr.To(int32(100)),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"), // Assuming the oauth-proxy is exposing on https port
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		objs = append(objs, route)
		query.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name":               "query-frontend-tls",
			"serviceaccounts.openshift.io/oauth-redirectreference.application": `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"thanos-query-frontend-rhobs"}}`,
		}
		query.Spec.QueryFrontend.Additional.ServicePorts = append(query.Spec.QueryFrontend.Additional.ServicePorts, corev1.ServicePort{
			Name: "https",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
		})
		query.Spec.QueryFrontend.Additional.Containers = append(query.Spec.QueryFrontend.Additional.Containers, makeOauthProxyContainer(9090, namespace, "thanos-query-frontend-rhobs", "query-frontend-tls"))
		query.Spec.QueryFrontend.Additional.Volumes = append(query.Spec.QueryFrontend.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("tls", "query-frontend-tls"))
		query.Spec.QueryFrontend.Additional.Volumes = append(query.Spec.QueryFrontend.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("oauth-cookie", "oauth-cookie"))
	}

	objs = append(objs, query)
	return objs
}

func defaultStoreCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	return &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn(clusters.StoreDefault, templates.Replicas),
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			IndexCacheConfig: &v1alpha1.CacheConfig{
				ExternalCacheConfig: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-index-cache-memcached",
					},
					Key: "thanos.yaml",
				},
			},
			CachingBucketConfig: &v1alpha1.CacheConfig{
				ExternalCacheConfig: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-bucket-cache-memcached",
					},
					Key: "thanos.yaml",
				},
			},
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 1,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-22h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.StoreDefault, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
}

func defaultReceiveCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	grpcDisableEndlessRetry := `{
  "loadBalancingPolicy":"round_robin",
  "retryPolicy": {
    "maxAttempts": 0,
    "initialBackoff": "0.1s",
    "backoffMultiplier": 1,
    "retryableStatusCodes": [
  	  "UNAVAILABLE"
    ]
  }
}`

	return &v1alpha1.ThanosReceive{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosReceive",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosReceiveSpec{
			Router: v1alpha1.RouterSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.Images)),
					Version:              ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.ResourceRequirements)),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
				Replicas:          clusters.TemplateFn(clusters.ReceiveRouter, templates.Replicas),
				ReplicationFactor: 3,
				ExternalLabels: map[string]string{
					"receive": "true",
				},
				Additional: v1alpha1.Additional{
					Args: []string{
						fmt.Sprintf("--receive.grpc-service-config=%s", grpcDisableEndlessRetry),
					},
				},
			},
			Ingester: v1alpha1.IngesterSpec{
				DefaultObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
				Additional:                 v1alpha1.Additional{},
				Hashrings: []v1alpha1.IngesterHashringSpec{
					{
						Name: "default",
						CommonFields: v1alpha1.CommonFields{
							Image:                ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Images)),
							Version:              ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Versions)),
							ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
							LogLevel:             ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.LogLevels)),
							LogFormat:            ptr.To("logfmt"),
							ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.ResourceRequirements)),
							SecurityContext: &corev1.PodSecurityContext{
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
						ExternalLabels: map[string]string{
							"replica": "$(POD_NAME)",
						},
						Replicas: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Replicas),
						TSDBConfig: v1alpha1.TSDBConfig{
							Retention: v1alpha1.Duration("1d"),
						},
						AsyncForwardWorkerCount:  ptr.To(uint64(50)),
						TooFarInFutureTimeWindow: ptr.To(v1alpha1.Duration("5m")),
						StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
							StoreLimitsRequestSamples: 0,
							StoreLimitsRequestSeries:  0,
						},
						TenancyConfig: &v1alpha1.TenancyConfig{
							TenantMatcherType: "exact",
							DefaultTenantID:   "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
							TenantHeader:      "THANOS-TENANT",
							TenantLabelName:   "tenant_id",
						},
						ObjectStorageConfig: ptr.To(clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket)),
						StorageConfiguration: v1alpha1.StorageConfiguration{
							Size: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.StorageSize),
						},
					},
				},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
}

func defaultCompactCR(namespace string, templates clusters.TemplateMaps, oauth bool) []runtime.Object {
	var objs []runtime.Object
	defaultCompact := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("365d"),
				FiveMinutes: v1alpha1.Duration("365d"),
				OneHour:     v1alpha1.Duration("365d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(1)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				CompactConcurrency: ptr.To(int32(1)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(true),
				MaxCompactionLevel:   ptr.To(int32(3)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.CompactDefault, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{
					`--deduplication.replica-label=replica`,
				},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	if oauth {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "route.openshift.io/v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thanos-compact-rhobs",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "thanos",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "thanos-compact-rhobs",
					Weight: ptr.To(int32(100)),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"), // Assuming the oauth-proxy is exposing on https port
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		objs = append(objs, route)
		defaultCompact.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name":               "compact-tls",
			"serviceaccounts.openshift.io/oauth-redirectreference.application": `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"thanos-compact-rhobs"}}`,
		}
		defaultCompact.Spec.Additional.ServicePorts = append(defaultCompact.Spec.Additional.ServicePorts, corev1.ServicePort{
			Name: "https",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
		})
		defaultCompact.Spec.Additional.Containers = append(defaultCompact.Spec.Additional.Containers, makeOauthProxyContainer(10902, namespace, "thanos-compact-rhobs", "compact-tls"))
		defaultCompact.Spec.Additional.Volumes = append(defaultCompact.Spec.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("tls", "compact-tls"))
		defaultCompact.Spec.Additional.Volumes = append(defaultCompact.Spec.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("oauth-cookie", "oauth-cookie"))
	}

	objs = append(objs, defaultCompact)
	return objs
}

func defaultRulerCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	return &v1alpha1.ThanosRuler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosRuler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.Ruler, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.Ruler, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.Ruler, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.Ruler, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas: clusters.TemplateFn(clusters.Ruler, templates.Replicas),
			RuleConfigSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/rule-file": "true",
				},
			},
			PrometheusRuleSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/prometheus-rule": "true",
				},
			},
			QueryLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/query-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
				TenantLabel:      "tenant_id",
				TenantValueLabel: "operator.thanos.io/tenant",
			},
			ExternalLabels: map[string]string{
				"rule_replica": "$(NAME)",
			},
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			AlertmanagerURL:     "dnssrv+http://alertmanager-cluster." + namespace + ".svc.cluster.local:9093",
			AlertLabelDrop:      []string{"rule_replica"},
			Retention:           v1alpha1.Duration("48h"),
			EvaluationInterval:  v1alpha1.Duration("1m"),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.Ruler, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
}

func queryCR(namespace string, templates clusters.TemplateMaps, oauth bool, withAdditonalArgs ...string) []runtime.Object {
	// placeholder for prod caches - temp removed whilst debugging
	qfeCacheTempProd := v1alpha1.Additional{
		Args: []string{`--query-range.response-cache-config=
  "type": "memcached"
  "config":
    "addresses":
      - "dnssrv+_client._tcp.thanos-query-range-cache.rhobs-production.svc"
    "dns_provider_update_interval": "30s"
    "max_async_buffer_size": 1000000
    "max_async_concurrency": 100
    "max_get_multi_batch_size": 500
    "max_get_multi_concurrency": 100
    "max_idle_connections": 500
    "max_item_size": "100MiB"
    "timeout": "5s"`,
		},
	}
	log.Println(qfeCacheTempProd)

	var objs []runtime.Object

	query := &v1alpha1.ThanosQuery{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosQuery",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosQuerySpec{
			Additional: v1alpha1.Additional{
				Args: withAdditonalArgs,
			},
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("QUERY", templates.Images)),
				Version:              ptr.To(clusters.TemplateFn("QUERY", templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("QUERY", templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("QUERY", templates.ResourceRequirements)),
			},
			StoreLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/store-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			Replicas: clusters.TemplateFn("QUERY", templates.Replicas),
			ReplicaLabels: []string{
				"prometheus_replica",
				"replica",
				"rule_replica",
			},
			WebConfig: &v1alpha1.WebConfig{
				PrefixHeader: ptr.To("X-Forwarded-Prefix"),
			},
			GRPCProxyStrategy: "lazy",
			TelemetryQuantiles: &v1alpha1.TelemetryQuantiles{
				Duration: []string{
					"0.1", "0.25", "0.75", "1.25", "1.75", "2.5", "3", "5", "10", "15", "30", "60", "120",
				},
			},
			QueryFrontend: &v1alpha1.QueryFrontendSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Images)),
					Version:              ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.ResourceRequirements)),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
				Replicas:             clusters.TemplateFn("QUERY_FRONTEND", templates.Replicas),
				CompressResponses:    true,
				LogQueriesLongerThan: ptr.To(v1alpha1.Duration("10s")),
				LabelsMaxRetries:     3,
				QueryRangeMaxRetries: 3,
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/query-api": "true",
					},
				},
				QueryRangeSplitInterval: ptr.To(v1alpha1.Duration("48h")),
				LabelsSplitInterval:     ptr.To(v1alpha1.Duration("48h")),
				LabelsDefaultTimeRange:  ptr.To(v1alpha1.Duration("336h")),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
				PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}
	if oauth {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "route.openshift.io/v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thanos-query-frontend-rhobs",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "thanos",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "thanos-query-frontend-rhobs",
					Weight: ptr.To(int32(100)),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"), // Assuming the oauth-proxy is exposing on https port
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		objs = append(objs, route)
		query.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name":               "query-frontend-tls",
			"serviceaccounts.openshift.io/oauth-redirectreference.application": `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"thanos-query-frontend-rhobs"}}`,
		}
		query.Spec.QueryFrontend.Additional.ServicePorts = append(query.Spec.QueryFrontend.Additional.ServicePorts, corev1.ServicePort{
			Name: "https",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
		})
		query.Spec.QueryFrontend.Additional.Containers = append(query.Spec.QueryFrontend.Additional.Containers, makeOauthProxyContainer(9090, namespace, "thanos-query-frontend-rhobs", "query-frontend-tls"))
		query.Spec.QueryFrontend.Additional.Volumes = append(query.Spec.QueryFrontend.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("tls", "query-frontend-tls"))
		query.Spec.QueryFrontend.Additional.Volumes = append(query.Spec.QueryFrontend.Additional.Volumes, kghelpers.NewPodVolumeFromSecret("oauth-cookie", "oauth-cookie"))
	}

	objs = append(objs, query)
	return objs
}

func rulerCR(namespace string, templates clusters.TemplateMaps) []runtime.Object {
	return []runtime.Object{
		&v1alpha1.ThanosRuler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.thanos.io/v1alpha1",
				Kind:       "ThanosRuler",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemeter",
				Namespace: namespace,
			},
			Spec: v1alpha1.ThanosRulerSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn("RULER", templates.Images)),
					Version:              ptr.To(clusters.TemplateFn("RULER", templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn("RULER", templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn("RULER", templates.ResourceRequirements)),
				},
				Replicas: clusters.TemplateFn("RULER", templates.Replicas),
				RuleConfigSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/rule-file": "true",
					},
				},
				PrometheusRuleSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/prometheus-rule": "true",
					},
				},
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/query-api": "true",
						"app.kubernetes.io/part-of":    "thanos",
					},
				},
				ExternalLabels: map[string]string{
					"rule_replica": "$(NAME)",
				},
				ObjectStorageConfig: clusters.TemplateFn("TELEMETER", templates.ObjectStorageBucket),
				RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
					TenantLabel:      "tenant_id",
					TenantValueLabel: "operator.thanos.io/tenant",
				},
				AlertmanagerURL:    "dnssrv+http://alertmanager-cluster." + namespace + ".svc.cluster.local:9093",
				AlertLabelDrop:     []string{"rule_replica"},
				Retention:          v1alpha1.Duration("48h"),
				EvaluationInterval: v1alpha1.Duration("1m"),
				StorageConfiguration: v1alpha1.StorageConfiguration{
					Size: clusters.TemplateFn("RULER", templates.StorageSize),
				},
				FeatureGates: &v1alpha1.FeatureGates{
					ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
						Enable: ptr.To(false),
					},
				},
			},
		},
	}
}

func compactTempProduction(templates clusters.TemplateMaps) []runtime.Object {
	ns := "rhobs-production"
	image := string(clusters.TemplateFn("COMPACT", templates.Images))
	version := string(clusters.TemplateFn("RULER", templates.Versions))
	storageBucket := "TELEMETER"

	m := clusters.ProductionMaps

	notTelemeter := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rules-and-rhobs",
			Namespace: ns,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			Additional: v1alpha1.Additional{
				Args: []string{
					`--deduplication.replica-label=replica`,
				},
			},
			ShardingConfig: []v1alpha1.ShardingConfig{
				{
					ShardName: "rhobs",
					ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{
						{
							Label: "receive",
							Value: "true",
						},
						{
							Label: "tenant_id",
							Value: "0fc2b00e-201b-4c17-b9f2-19d91adc4fd2",
						},
					},
				},
				{
					ShardName: "rules",
					ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{
						{
							Label: "receive",
							Value: "!true",
						},
					},
				},
			},
			CommonFields: v1alpha1.CommonFields{
				Image:           ptr.To(image),
				Version:         ptr.To(version),
				ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
				LogLevel:        ptr.To("warn"),
				LogFormat:       ptr.To("logfmt"),
			},
			ObjectStorageConfig: clusters.TemplateFn(storageBucket, m.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("3650d"),
				FiveMinutes: v1alpha1.Duration("3650d"),
				OneHour:     v1alpha1.Duration("3650d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(4)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				BlockFetchConcurrency: ptr.To(int32(8)),
				CompactConcurrency:    ptr.To(int32(8)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(false),
				MaxCompactionLevel:   ptr.To(int32(4)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: v1alpha1.StorageSize("500Gi"),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	telemeterHistoric := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "receive-historic",
			Namespace: ns,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			Additional: v1alpha1.Additional{
				Args: []string{
					`--deduplication.replica-label=replica`,
				},
			},
			ShardingConfig: []v1alpha1.ShardingConfig{
				{
					ShardName: "telemeter",
					ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{
						{
							Label: "receive",
							Value: "true",
						},
						{
							Label: "tenant_id",
							Value: "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
						},
					},
				},
			},
			CommonFields: v1alpha1.CommonFields{
				Image:           ptr.To(image),
				Version:         ptr.To(version),
				ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
				LogLevel:        ptr.To("info"),
				LogFormat:       ptr.To("logfmt"),
			},
			ObjectStorageConfig: clusters.TemplateFn(storageBucket, m.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("3650d"),
				FiveMinutes: v1alpha1.Duration("3650d"),
				OneHour:     v1alpha1.Duration("3650d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(4)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				BlockFetchConcurrency: ptr.To(int32(4)),
				CompactConcurrency:    ptr.To(int32(4)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(true),
				MaxCompactionLevel:   ptr.To(int32(4)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: v1alpha1.StorageSize("3000Gi"),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-120d")),
			},
		},
	}

	telemeter := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "receive",
			Namespace: ns,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			Additional: v1alpha1.Additional{
				Args: []string{
					`--deduplication.replica-label=replica`,
				},
			},
			ShardingConfig: []v1alpha1.ShardingConfig{
				{
					ShardName: "telemeter",
					ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{
						{
							Label: "receive",
							Value: "true",
						},
						{
							Label: "tenant_id",
							Value: "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
						},
					},
				},
			},
			CommonFields: v1alpha1.CommonFields{
				Image:           ptr.To(image),
				Version:         ptr.To(version),
				ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
				LogLevel:        ptr.To("info"),
				LogFormat:       ptr.To("logfmt"),
			},
			ObjectStorageConfig: clusters.TemplateFn(storageBucket, m.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("3650d"),
				FiveMinutes: v1alpha1.Duration("3650d"),
				OneHour:     v1alpha1.Duration("3650d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(4)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				BlockFetchConcurrency: ptr.To(int32(4)),
				CompactConcurrency:    ptr.To(int32(4)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(true),
				MaxCompactionLevel:   ptr.To(int32(4)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: v1alpha1.StorageSize("3000Gi"),
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MinTime: ptr.To(v1alpha1.Duration("-61d")),
			},
		},
	}
	return []runtime.Object{notTelemeter, telemeterHistoric, telemeter}
}

// RHOBS-904: Standalone Compact for RH Resource Optimisation (ROS) Managed Service
func stageCompactCR(namespace string, templates clusters.TemplateMaps) []runtime.Object {
	rosCompact := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ros",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("COMPACT_ROS", templates.Images)),
				Version:              ptr.To(clusters.TemplateFn("COMPACT_ROS", templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("COMPACT_ROS", templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("COMPACT_ROS", templates.ResourceRequirements)),
			},
			ObjectStorageConfig: clusters.TemplateFn("ROS", templates.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("14d"),
				FiveMinutes: v1alpha1.Duration("14d"),
				OneHour:     v1alpha1.Duration("14d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(1)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				CompactConcurrency: ptr.To(int32(1)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(true),
				MaxCompactionLevel:   ptr.To(int32(3)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("COMPACT_ROS", templates.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
			FeatureGates: &v1alpha1.FeatureGates{
				ServiceMonitorConfig: &v1alpha1.ServiceMonitorConfig{
					Enable: ptr.To(false),
				},
			},
		},
	}

	return []runtime.Object{rosCompact}
}

// generateMetricsBundle generates individual metrics bundle resources for Thanos components
// Ordering: CRDs, operator, cache, then Thanos components
func generateMetricsBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With(templatePath, templateClustersPath, string(config.Environment), string(config.Name), "metrics", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// 1. CRDs (prefix: 01-*)
	crdObjs := getCRDObjects()
	crdNames := []string{"compacts", "queries", "receives", "rulers", "stores"}
	for i, crd := range crdObjs {
		crdName := "unknown"
		if i < len(crdNames) {
			crdName = crdNames[i]
		}
		filename := fmt.Sprintf("01-crd-%s.yaml", crdName)
		bundleGen.Add(filename, encoding.GhodssYAML(crd))
	}

	// 2. OPERATOR (prefix: 02-*)
	operatorObjs, err := operatorResources(ns, config.Templates)
	if err != nil {
		return fmt.Errorf("failed to generate operator resources: %w", err)
	}
	for _, obj := range operatorObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getSmartResourceName(obj)
		filename := fmt.Sprintf("02-operator-%s-%s.yaml", resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// 3. CACHE (prefix: 03-*)
	cacheObjs := getThanosCacheObjects(ns, config.Templates)
	for _, obj := range cacheObjs {
		cacheKind := getResourceKind(obj)
		cacheName := getResourceName(obj)
		// Remove thanos- prefix and simplify cache names
		cleanCacheName := strings.TrimPrefix(cacheName, "thanos-")
		filename := fmt.Sprintf("03-cache-%s-%s.yaml", cleanCacheName, cacheKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// 4. CUSTOM RESOURCES (prefix: 04-*)
	thanosObjs := make([]runtime.Object, 0, 7) // Pre-allocate for expected ~7 resources (query+route, receive, compact+route, ruler, store)
	thanosObjs = append(thanosObjs, defaultQueryCR(ns, config.Templates, true)...)
	thanosObjs = append(thanosObjs, defaultReceiveCR(ns, config.Templates))
	thanosObjs = append(thanosObjs, defaultCompactCR(ns, config.Templates, true)...)
	thanosObjs = append(thanosObjs, defaultRulerCR(ns, config.Templates))
	thanosObjs = append(thanosObjs, defaultStoreCR(ns, config.Templates))

	for i, obj := range thanosObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getResourceName(obj)
		// Clean up names and remove redundant prefixes
		cleanName := strings.TrimPrefix(resourceName, "thanos-")
		cleanName = strings.TrimPrefix(cleanName, "rhobs-")
		if cleanName == "Unknown" || cleanName == resourceKind || cleanName == "" {
			filename := fmt.Sprintf("04-%s-%d.yaml", strings.ToLower(resourceKind), i+1)
			bundleGen.Add(filename, encoding.GhodssYAML(obj))
		} else {
			filename := fmt.Sprintf("04-%s-%s.yaml", cleanName, resourceKind)
			bundleGen.Add(filename, encoding.GhodssYAML(obj))
		}
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add consolidated ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	thanosServiceMonitors := createConsolidatedThanosServiceMonitors(ns)
	operatorServiceMonitors := thanosOperatorServiceMonitor(ns)

	for _, sm := range thanosServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}
	for _, sm := range operatorServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	// Generate the individual ServiceMonitor files
	if err := monBundle.Generate(); err != nil {
		return fmt.Errorf("failed to generate monitoring bundle: %w", err)
	}

	return nil
}

// getCRDObjects retrieves Thanos operator CRDs
func getCRDObjects() []runtime.Object {
	const (
		compact   = "thanoscompacts.yaml"
		queries   = "thanosqueries.yaml"
		receivers = "thanosreceives.yaml"
		rulers    = "thanosrulers.yaml"
		stores    = "thanosstores.yaml"
		base      = "https://raw.githubusercontent.com/thanos-community/thanos-operator/" + thanosOperatorCRDRef + "/config/crd/bases/monitoring.thanos.io_"
	)

	var objs []runtime.Object
	for _, component := range []string{compact, queries, receivers, rulers, stores} {
		crd, err := getCustomResourceDefinition(base + component)
		if err != nil {
			log.Printf("Error fetching CRD %s: %v", component, err)
			continue
		}
		objs = append(objs, crd)
	}
	return objs
}

// getThanosCacheObjects returns cache objects for Thanos components
func getThanosCacheObjects(namespace string, templates clusters.TemplateMaps) []runtime.Object {
	var objs []runtime.Object

	// Index cache
	indexCacheConfig := indexCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(indexCacheConfig, templates))
	objs = append(objs, createServiceAccount(indexCacheConfig.Name, indexCacheConfig.Namespace, indexCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(indexCacheConfig))

	// Bucket cache
	bucketCacheConfig := bucketCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(bucketCacheConfig, templates))
	objs = append(objs, createServiceAccount(bucketCacheConfig.Name, bucketCacheConfig.Namespace, bucketCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(bucketCacheConfig))

	// Query range cache
	queryRangeCacheConfig := queryRangeCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(queryRangeCacheConfig, templates))
	objs = append(objs, createServiceAccount(queryRangeCacheConfig.Name, queryRangeCacheConfig.Namespace, queryRangeCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(queryRangeCacheConfig))

	// Cache secrets
	cacheSecrets := memcachedCacheSecrets(namespace)
	for _, secret := range cacheSecrets {
		objs = append(objs, secret)
	}

	return objs
}

// getResourceName extracts a meaningful name from a Kubernetes object
func getResourceName(obj runtime.Object) string {
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

// getSmartResourceName generates meaningful names for operator resources
func getSmartResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	// Get the basic name first
	basicName := getResourceName(obj)
	if basicName == "unnamed" || basicName == "unknown" {
		return basicName
	}

	// For operator resources that have thanos-operator prefix, remove it since it's implied
	basicName = strings.TrimPrefix(basicName, "thanos-operator-")

	return basicName
}
