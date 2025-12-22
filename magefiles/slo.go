package main

import (
	"fmt"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	pyrrav1alpha1 "github.com/pyrra-dev/pyrra/kubernetes/api/v1alpha1"
	"github.com/pyrra-dev/pyrra/slo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Resource string

const (
	MetricsResource Resource = "metrics"
	LogsResource    Resource = "logs"
	ProbesResource  Resource = "probes"
)

const (
	globalSLOWindowDuration                   = "28d"  // Window over which all RHOBS SLOs are calculated.
	globalMetricsSLOAvailabilityTargetPercent = "99.9" // The Availability Target percentage for RHOBS metrics availability SLOs.
	globalSLOLatencyTargetPercent             = "90"   // The Latency Target percentage for RHOBS latency SLOs.
	genericSLOLatencySeconds                  = "5"    // Latency request duration to measure percentile target (this is diff for query SLOs).

	rhobsNextServiceLabel = "rhobs.regional"
)

var (
	// Reusable k8s type metas.
	pyrraTypeMeta = metav1.TypeMeta{
		Kind:       "ServiceLevelObjective",
		APIVersion: pyrrav1alpha1.GroupVersion.Version,
	}
)

// sloType indicates the type of a particular SLO in rhobsSLOs shorthand.
type sloType string

const (
	// Pyrra Latency SLO, calculated as percentile ratio of successful requests
	// in a latency bucket by total successful requests. For example, p90 of
	// # of http requests with 2xx response code, under 5s / # of http requests with 2xx.
	sloTypeLatency sloType = "latency"

	// Pyrra Availablity SLO, calculated as the inverse percentage ratio of errors by total
	// requests. For example, (1 - # of http requests with 5xx response code / # of http requests) * 100.
	sloTypeAvailability sloType = "availability"
)

// rhobsSLOs is a shorthand struct to generate Pyrra SLOs.
type rhobsSLOs struct {
	name                string
	labels              map[string]string
	description         string
	summary             string
	successOrErrorsExpr string
	totalExpr           string
	alertName           string
	sloType             sloType
}

// rhobSLOList is a list of shorthand SLOs.
type rhobSLOList []rhobsSLOs

// GetObjectives returns Pyrra Objectives from a rhobsSLOList shorthand.
func (slos rhobSLOList) GetObjectives() []pyrrav1alpha1.ServiceLevelObjective {
	objectives := []pyrrav1alpha1.ServiceLevelObjective{}

	for _, s := range slos {
		objective := pyrrav1alpha1.ServiceLevelObjective{
			TypeMeta: pyrraTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:   s.name,
				Labels: s.labels,
				Annotations: map[string]string{
					slo.PropagationLabelsPrefix + "description": s.description,
					slo.PropagationLabelsPrefix + "summary":     s.summary,
					slo.PropagationLabelsPrefix + "dashboard":   getGrafanaLink(),
					slo.PropagationLabelsPrefix + "runbook":     getRunbookLink(s.alertName),
				},
			},
			Spec: pyrrav1alpha1.ServiceLevelObjectiveSpec{
				Description: s.description,
				Window:      globalSLOWindowDuration,
				Alerting: pyrrav1alpha1.Alerting{
					Name: s.alertName,
				},
			},
		}

		if s.sloType == sloTypeAvailability {
			// Metrics availability target as the default.
			objective.Spec.Target = globalMetricsSLOAvailabilityTargetPercent
			objective.Spec.ServiceLevelIndicator = pyrrav1alpha1.ServiceLevelIndicator{
				Ratio: &pyrrav1alpha1.RatioIndicator{
					Errors: pyrrav1alpha1.Query{
						Metric: s.successOrErrorsExpr,
					},
					Total: pyrrav1alpha1.Query{
						Metric: s.totalExpr,
					},
				},
			}
		} else {
			objective.Spec.Target = globalSLOLatencyTargetPercent
			objective.Spec.ServiceLevelIndicator = pyrrav1alpha1.ServiceLevelIndicator{
				Latency: &pyrrav1alpha1.LatencyIndicator{
					Success: pyrrav1alpha1.Query{
						Metric: s.successOrErrorsExpr,
					},
					Total: pyrrav1alpha1.Query{
						Metric: s.totalExpr,
					},
				},
			}
		}

		objectives = append(objectives, objective)
	}

	return objectives
}

// getGrafanaLink returns the AppSRE production Grafana dashboard for a particular RHOBS environment.
func getGrafanaLink() string {
	return "https://grafana.app-sre.devshift.net/d/283e7002d85c08126681241df2fdb22b/rhobs-next-slos?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
}

// getRunbookLink returns the rhobs/config runbook link for a particular alert.
func getRunbookLink(alert string) string {
	return fmt.Sprintf(
		"https://github.com/rhobs/configuration/blob/main/docs/sop/observatorium.md#%s",
		alert,
	)
}

// ObservatoriumSLOs returns the observatorium/observatorium specific SLOs we maintain.
//
// This set of SLOs are driven by the RHOBS Service Level Objectives document
// https://docs.google.com/document/d/1wJjcpgg-r8rlnOtRiqWGv0zwr1MB6WwkQED1XDWXVQs/edit
func ObservatoriumSLOs(signal Resource) []pyrrav1alpha1.ServiceLevelObjective {
	var slos rhobSLOList
	switch signal {
	case MetricsResource:
		slos = rhobSLOList{
			// Observatorium Metrics Availability SLOs.
			{
				name: "api-metrics-write-availability-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API /receive handler is burning too much error budget to guarantee availability SLOs.",
				summary:             "API /receive handler is burning too much error budget to guarantee availability SLOs.",
				successOrErrorsExpr: "http_requests_total{job=\"rhobs-gateway\", handler=\"receive\", group=\"metricsv1\", code=~\"^5..$\"}",
				totalExpr:           "http_requests_total{job=\"rhobs-gateway\", handler=\"receive\", group=\"metricsv1\"}",
				alertName:           "APIMetricsWriteAvailabilityErrorBudgetBurning",
				sloType:             sloTypeAvailability,
			},
			// Queriers are deployed as separate instances for adhoc and rule queries.
			// The read availability SLO is split to reflect this deployment topology.
			{
				name: "api-metrics-query-availability-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API /query handler is burning too much error budget to guarantee availability SLOs.",
				summary:             "API /query handler is burning too much error budget to guarantee availability SLOs.",
				successOrErrorsExpr: "http_requests_total{job=\"rhobs-gateway\", handler=\"query\", group=\"metricsv1\", code=~\"^5..$\"}",
				totalExpr:           "http_requests_total{job=\"rhobs-gateway\", handler=\"query\", group=\"metricsv1\"}",
				alertName:           "APIMetricsQueryAvailabilityErrorBudgetBurning",
				sloType:             sloTypeAvailability,
			},
			{
				name: "api-metrics-query-range-availability-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API /query_range handler is burning too much error budget to guarantee availability SLOs.",
				summary:             "API /query_range handler is burning too much error budget to guarantee availability SLOs.",
				successOrErrorsExpr: "http_requests_total{job=\"rhobs-gateway\", handler=\"query_range\", group=\"metricsv1\", code=~\"^5..$\"}",
				totalExpr:           "http_requests_total{job=\"rhobs-gateway\", handler=\"query_range\", group=\"metricsv1\"}",
				alertName:           "APIMetricsQueryRangeAvailabilityErrorBudgetBurning",
				sloType:             sloTypeAvailability,
			},
			{
				name: "api-alerting-availability-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API Thanos Rule failing to send alerts to Alertmanager and is burning too much error budget to guarantee availability SLOs.",
				summary:             "API Thanos Rule is burning too much error budget to guarantee availability SLOs.",
				successOrErrorsExpr: "thanos_alert_sender_alerts_dropped_total{container=\"thanos-ruler\", code=~\"^5..$\"}",
				totalExpr:           "thanos_alert_sender_alerts_dropped_total{container=\"thanos-ruler\"}",
				alertName:           "APIAlertmanagerAvailabilityErrorBudgetBurning",
				sloType:             sloTypeAvailability,
			},
			{
				name: "api-alerting-notif-availability-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API Alertmanager failing to deliver alerts to upstream targets and is burning too much error budget to guarantee availability SLOs.",
				summary:             "API Alertmanager is burning too much error budget to guarantee availability SLOs.",
				successOrErrorsExpr: "alertmanager_notifications_failed_total{job=\"alertmanager\", code=~\"^5..$\"}",
				totalExpr:           "alertmanager_notifications_failed_total{job=\"alertmanager\"}",
				alertName:           "APIAlertmanagerNotificationsAvailabilityErrorBudgetBurning",
				sloType:             sloTypeAvailability,
			},

			// Observatorium Metrics Latency SLOs.
			{
				name: "api-metrics-write-latency-slo",
				labels: map[string]string{
					slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
				},
				description:         "API /receive handler is burning too much error budget to guarantee latency SLOs.",
				summary:             "API /receive handler is burning too much latency error budget.",
				successOrErrorsExpr: "http_request_duration_seconds_bucket{job=\"rhobs-gateway\", handler=\"receive\", group=\"metricsv1\", code=~\"^2..$\", le=\"" + genericSLOLatencySeconds + "\"}",
				totalExpr:           "http_request_duration_seconds_count{job=\"rhobs-gateway\", handler=\"receive\", group=\"metricsv1\", code=~\"^2..$\"}",
				alertName:           "APIMetricsWriteLatencyErrorBudgetBurning",
				sloType:             sloTypeLatency,
			},
			// These are commented out as we are not deploying synthetic avalanche/up jobs to rhobs.regional yet.
			// We might choose to use other metrics/deploy those later on. For now dropping these SLOs.
			//
			// Queriers are deployed as separate instances for adhoc and rule queries.
			// The read latencies SLO are split to reflect this deployment topology.
			// {
			// 	name: "api-metrics-read-1M-latency-slo",
			// 	labels: map[string]string{
			// 		slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
			// 	},
			// 	description:         "API /query endpoint is burning too much error budget for 1M samples, to guarantee latency SLOs.",
			// 	summary:             "API /query endpoint is burning too much latency error budget.",
			// 	successOrErrorsExpr: "up_custom_query_duration_seconds_bucket{query=\"query-path-sli-1M-samples\", http_code=~\"^2..$\", le=\"10\"}",
			// 	totalExpr:           "up_custom_query_duration_seconds_count{query=\"query-path-sli-1M-samples\", http_code=~\"^2..$\"}",
			// 	alertName:           "APIMetricsReadLatency1MErrorBudgetBurning",
			// 	sloType:             sloTypeLatency,
			// },
			// {
			// 	name: "api-metrics-read-10M-latency-slo",
			// 	labels: map[string]string{
			// 		slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
			// 	},
			// 	description:         "API /query endpoint is burning too much error budget for 100M samples, to guarantee latency SLOs.",
			// 	summary:             "API /query endpoint is burning too much latency error budget.",
			// 	successOrErrorsExpr: "up_custom_query_duration_seconds_bucket{query=\"query-path-sli-10M-samples\", http_code=~\"^2..$\", le=\"30\"}",
			// 	totalExpr:           "up_custom_query_duration_seconds_count{query=\"query-path-sli-10M-samples\", http_code=~\"^2..$\"}",
			// 	alertName:           "APIMetricsReadLatency10MErrorBudgetBurning",
			// 	sloType:             sloTypeLatency,
			// },
			// {
			// 	name: "api-metrics-read-100M-latency-slo",
			// 	labels: map[string]string{
			// 		slo.PropagationLabelsPrefix + "service": rhobsNextServiceLabel,
			// 	},
			// 	description:         "API /query endpoint is burning too much error budget for 100M samples, to guarantee latency SLOs.",
			// 	summary:             "API /query endpoint is burning too much latency error budget.",
			// 	successOrErrorsExpr: "up_custom_query_duration_seconds_bucket{query=\"query-path-sli-1M-samples\", http_code=~\"^2..$\", le=\"120\"}",
			// 	totalExpr:           "up_custom_query_duration_seconds_count{query=\"query-path-sli-1M-samples\", http_code=~\"^2..$\"}",
			// 	alertName:           "APIMetricsReadLatency100MErrorBudgetBurning",
			// 	sloType:             sloTypeLatency,
			// },
		}
	default:
		panic(signal + " is not an Observatorium Resource")
	}

	return slos.GetObjectives()
}

// GenSLO is the function responsible for tying together Pyrra Objectives and converting them into Rule files.
func (b Build) SLORules() {
	gen := b.o11yGenerator("rhobs-next-slo-rules")

	envSLOs(
		ObservatoriumSLOs(MetricsResource),
		"rhobs-next-slos",
		gen,
	)
	gen.Generate()
}

// envSLOs generates the resultant config for a particular rhobsInstanceEnv.
func envSLOs(objs []pyrrav1alpha1.ServiceLevelObjective, ruleFilename string, genRules *mimic.Generator) {
	// We add "" to encoding as first arg, so that we get a YAML doc directive
	// at the start of the file as per app-interface format.
	genRules.Add(ruleFilename+".prometheusrules.yaml", encoding.GhodssYAML("", makePrometheusRule(objs, ruleFilename, false)))
	genRules.Add(ruleFilename+"-non-critical.prometheusrules.yaml", encoding.GhodssYAML("", makePrometheusRule(objs, ruleFilename, true)))
}

// Adapted from https://github.com/pyrra-dev/pyrra/blob/v0.5.3/kubernetes/controllers/servicelevelobjective.go#L207
// Helps us group and generate SLO rules into monitoringv1.PrometheusRule objects which are embedded in appInterfacePrometheusRule struct.
// Ideally, this can be done via pyrra generate command somehow. Upstream PR: https://github.com/pyrra-dev/pyrra/pull/620
// However even with CLI we might need to generate in specific format, and group together  SLO rules in different ways.
func makePrometheusRule(objs []pyrrav1alpha1.ServiceLevelObjective, name string, nonCriticalPostProcessing bool) appInterfacePrometheusRule {
	grp := []monitoringv1.RuleGroup{}
	for _, obj := range objs {
		objective, err := obj.Internal()
		if err != nil {
			mimic.PanicErr(err)
		}

		increases, err := objective.IncreaseRules()
		if err != nil {
			mimic.PanicErr(err)
		}
		grp = append(grp, increases)

		burnrates, err := objective.Burnrates()
		if err != nil {
			mimic.PanicErr(err)
		}
		grp = append(grp, burnrates)

		generic, err := objective.GenericRules()
		if err != nil {
			mimic.PanicErr(err)
		}
		grp = append(grp, generic)
	}

	// AppSRE customizations.
	for i := range grp {
		for j := range grp[i].Rules {
			if grp[i].Rules[j].Alert != "" {
				// Prune certain alert labels.
				delete(grp[i].Rules[j].Labels, "le")
				delete(grp[i].Rules[j].Labels, "client")
				delete(grp[i].Rules[j].Labels, "container")

				// Hack for AM alert labels.
				if v, ok := grp[i].Rules[j].Labels["service"]; ok && v == "alertmanager" {
					grp[i].Rules[j].Labels["service"] = rhobsNextServiceLabel
				}
			}

			// Make long/short labels more descriptive.
			if v, ok := grp[i].Rules[j].Labels["long"]; ok {
				grp[i].Rules[j].Labels["long_burnrate_window"] = v
				delete(grp[i].Rules[j].Labels, "long")
			}

			if v, ok := grp[i].Rules[j].Labels["short"]; ok {
				grp[i].Rules[j].Labels["short_burnrate_window"] = v
				delete(grp[i].Rules[j].Labels, "short")
			}
		}
	}
	// We do not want to page on SLO alerts until we're comfortable with how frequently
	// they fire.
	// Ticket: https://issues.redhat.com/browse/RHOBS-781
	// We also do not want to send noise to app-sre Slack when the SLOMetricAbsent alert
	// fires, as the metrics we use for the SLOs are sometimes unitialized for a long time.
	// For example, some SLOS on API endpoints with low traffic sometimes trigger this.
	for i := range grp {
		for j := range grp[i].Rules {
			if grp[i].Rules[j].Alert == "SLOMetricAbsent" {
				grp[i].Rules[j].Labels["severity"] = "medium"
				continue
			}
			if nonCriticalPostProcessing {
				if v, ok := grp[i].Rules[j].Labels["severity"]; ok {
					switch v {
					case "critical":
						grp[i].Rules[j].Labels["severity"] = "high"
					case "warning":
						grp[i].Rules[j].Labels["severity"] = "medium"
					}
				}
			}
		}
	}

	grp = ReplaceSummaryWithMessage(grp)

	return appInterfacePrometheusRule{
		Schema: "/openshift/prometheus-rule-1.yml",
		PrometheusRule: monitoringv1.PrometheusRule{
			TypeMeta: promRuleTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: ruleFileLabels,
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: grp,
			},
		},
	}
}
