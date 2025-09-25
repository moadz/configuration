package main

import (
	"time"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/configuration/clusters"
)

// Dashboard URLs
const (
	dashboardComponentAbsent = "https://grafana.app-sre.devshift.net/d/no-dashboard/thanos-component-absent?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosCompact   = "https://grafana.app-sre.devshift.net/d/651943d05a8123e32867b4673963f42b/thanos-compact?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosQuery     = "https://grafana.app-sre.devshift.net/d/98fde97ddeaf2981041745f1f2ba68c2/thanos-query?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosReceive   = "https://grafana.app-sre.devshift.net/d/916a852b00ccc5ed81056644718fa4fb/thanos-receive?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosStore     = "https://grafana.app-sre.devshift.net/d/e832e8f26403d95fac0ea1c59837588b/thanos-store?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosRule      = "https://grafana.app-sre.devshift.net/d/35da848f5f92b2dc612e0c3a0577b8a1/thanos-rule?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
)

// Runbook URLs
const (
	runbookBaseURL                                                 = "https://github.com/rhobs/configuration/blob/main/docs/sop/observatorium.md"
	runbookThanosCompactIsDown                                     = runbookBaseURL + "#thanoscompactisdown"
	runbookThanosQueryIsDown                                       = runbookBaseURL + "#thanosqueryisdown"
	runbookThanosReceiveIsDown                                     = runbookBaseURL + "#thanosreceiveisdown"
	runbookThanosRuleIsDown                                        = runbookBaseURL + "#thanosruleisdown"
	runbookThanosStoreIsDown                                       = runbookBaseURL + "#thanosstoreisdown"
	runbookThanosCompactMultipleRunning                            = runbookBaseURL + "#thanoscompactmultiplerunning"
	runbookThanosCompactHalted                                     = runbookBaseURL + "#thanoscompacthalted"
	runbookThanosCompactHighCompactionFailures                     = runbookBaseURL + "#thanoscompacthighcompactionfailures"
	runbookThanosCompactBucketHighOperationFailures                = runbookBaseURL + "#thanoscompactbuckethighoperationfailures"
	runbookThanosCompactHasNotRun                                  = runbookBaseURL + "#thanoscompacthasnotrun"
	runbookThanosQueryHttpRequestQueryErrorRateHigh                = runbookBaseURL + "#thanosqueryhttprequestqueryerrorratehigh"
	runbookThanosQueryGrpcServerErrorRate                          = runbookBaseURL + "#thanosquerygrpcservererrorrate"
	runbookThanosQueryGrpcClientErrorRate                          = runbookBaseURL + "#thanosquerygrpcclienterrorrate"
	runbookThanosQueryHighDNSFailures                              = runbookBaseURL + "#thanosqueryhighdnsfailures"
	runbookThanosQueryInstantLatencyHigh                           = runbookBaseURL + "#thanosqueryinstantlatencyhigh"
	runbookThanosReceiveHttpRequestErrorRateHigh                   = runbookBaseURL + "#thanosreceivehttprequesterrorratehigh"
	runbookThanosReceiveHttpRequestLatencyHigh                     = runbookBaseURL + "#thanosreceivehttprequestlatencyhigh"
	runbookThanosReceiveHighReplicationFailures                    = runbookBaseURL + "#thanosreceivehighreplicationfailures"
	runbookThanosReceiveHighForwardRequestFailures                 = runbookBaseURL + "#thanosreceivehighforwardrequestfailures"
	runbookThanosReceiveHighHashringFileRefreshFailures            = runbookBaseURL + "#thanosreceivehighhashringfilerefreshfailures"
	runbookThanosReceiveConfigReloadFailure                        = runbookBaseURL + "#thanosreceiveconfigreloadfailure"
	runbookThanosReceiveNoUpload                                   = runbookBaseURL + "#thanosreceivenoupload"
	runbookThanosReceiveLimitsConfigReloadFailure                  = runbookBaseURL + "#thanosreceivelimitsconfigreloadfailure"
	runbookThanosReceiveLimitsHighMetaMonitoringQueriesFailureRate = runbookBaseURL + "#thanosreceivelimitshighmetamonitoringqueriesfailurerate"
	runbookThanosReceiveTenantLimitedByHeadSeries                  = runbookBaseURL + "#thanosreceivetenantlimitedbyheadseries"
	runbookThanosStoreGrpcErrorRate                                = runbookBaseURL + "#thanosstoregrpcerrorrate"
	runbookThanosStoreBucketHighOperationFailures                  = runbookBaseURL + "#thanosstorebuckethighoperationfailures"
	runbookThanosStoreObjstoreOperationLatencyHigh                 = runbookBaseURL + "#thanosstoreobjstoreoperationlatencyhigh"
	runbookThanosRuleQueueIsDroppingAlerts                         = runbookBaseURL + "#thanosrulequeueisdroppingalerts"
	runbookThanosRuleSenderIsFailingAlerts                         = runbookBaseURL + "#thanosrulesenderisfailingalerts"
	runbookThanosRuleHighRuleEvaluationFailures                    = runbookBaseURL + "#thanosrulehighruleevaluationfailures"
	runbookThanosRuleHighRuleEvaluationWarnings                    = runbookBaseURL + "#thanosrulehighruleevaluationwarnings"
	runbookThanosRuleRuleEvaluationLatencyHigh                     = runbookBaseURL + "#thanosruleruleevaluationlatencyhigh"
	runbookThanosRuleGrpcErrorRate                                 = runbookBaseURL + "#thanosrulegrpcerrorrate"
	runbookThanosRuleConfigReloadFailure                           = runbookBaseURL + "#thanosruleconfigreloadfailure"
	runbookThanosRuleQueryHighDNSFailures                          = runbookBaseURL + "#thanosrulequeryhighdnsfailures"
	runbookThanosRuleAlertmanagerHighDNSFailures                   = runbookBaseURL + "#thanosrulealertmanagerhighdnsfailures"
	runbookThanosRuleNoEvaluationFor10Intervals                    = runbookBaseURL + "#thanosrulenoevaluationfor10intervals"
	runbookThanosNoRuleEvaluations                                 = runbookBaseURL + "#thanosnoruleevaluations"
)

func (b Build) ThanosRules(config clusters.ClusterConfig) {
	gen := b.generator(config, "thanos-rules")
	thanosRules(gen, string(config.Name))
}

func thanosRules(gen *mimic.Generator, name string) {
	gen.Add("thanos-rules.yaml", encoding.GhodssYAML(ThanosPrometheusRule(name)))
	gen.Generate()
}

func ThanosPrometheusRule(name string) *appInterfacePrometheusRule {
	labels := map[string]string{
		"prometheus": "app-sre",
		"role":       "alert-rules",
	}

	annotations := map[string]string{}

	groups := []monitoringv1.RuleGroup{
		thanosComponentAbsentGroup(),
		thanosCompactGroup(),
		thanosQueryGroup(),
		thanosReceiveGroup(),
		thanosStoreGroup(),
		thanosRuleGroup(),
	}

	return NewPrometheusRuleForAppInterface(
		"thanos-"+name,
		labels,
		annotations,
		groups,
	)
}

func thanosComponentAbsentGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosCompactIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-compact.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosCompact has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosCompact has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosCompactIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
		NewAlertingRule(
			"ThanosQueryIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-query.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosQuery has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosQuery has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosQueryIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveRouterIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-receive-router.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosReceiveRouter has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosReceiveRouter has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosReceiveIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveIngesterIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-receive-ingester.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosReceiveIngester has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosReceiveIngester has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosReceiveIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
		NewAlertingRule(
			"ThanosRuleIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-ruler.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosRule has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosRule has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosRuleIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
		NewAlertingRule(
			"ThanosStoreIsDown",
			promqlbuilder.Absent(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-store.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardComponentAbsent,
				"description": "ThanosStore has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"message":     "ThanosStore has disappeared from {{$labels.namespace}}. Prometheus target for the component cannot be discovered.",
				"runbook":     runbookThanosStoreIsDown,
				"summary":     "Thanos component has disappeared from {{$labels.namespace}}.",
			},
		),
	}

	return NewRuleGroup("thanos-component-absent", "", nil, rules)
}

func thanosCompactGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosCompactMultipleRunning",
			promqlbuilder.Sum(
				promqlbuilder.Gtr(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-compact.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
			).By("namespace", "job"),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosCompact,
				"description": "No more than one Thanos Compact instance should be running at once. There are {{$value}} in {{$labels.namespace}} instances running.",
				"message":     "No more than one Thanos Compact instance should be running at once. There are {{$value}} in {{$labels.namespace}} instances running.",
				"runbook":     runbookThanosCompactMultipleRunning,
				"summary":     "Thanos Compact has multiple instances running.",
			},
		),
		NewAlertingRule(
			"ThanosCompactHalted",
			promqlbuilder.Eqlc(
				vector.New(
					vector.WithMetricName("thanos_compact_halted"),
					vector.WithLabelMatchers(
						label.New("job").EqualRegexp("thanos-compact.*"),
					),
				),
				promqlbuilder.NewNumber(1),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosCompact,
				"description": "Thanos Compact {{$labels.job}} in {{$labels.namespace}} has failed to run and now is halted.",
				"message":     "Thanos Compact {{$labels.job}} in {{$labels.namespace}} has failed to run and now is halted.",
				"runbook":     runbookThanosCompactHalted,
				"summary":     "Thanos Compact has failed to run and is now halted.",
			},
		),
		NewAlertingRule(
			"ThanosCompactHighCompactionFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_compact_group_compactions_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-compact.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_compact_group_compactions_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-compact.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosCompact,
				"description": "Thanos Compact {{$labels.job}} in {{$labels.namespace}} is failing to execute {{$value | humanize}}% of compactions.",
				"message":     "Thanos Compact {{$labels.job}} in {{$labels.namespace}} is failing to execute {{$value | humanize}}% of compactions.",
				"runbook":     runbookThanosCompactHighCompactionFailures,
				"summary":     "Thanos Compact is failing to execute compactions.",
			},
		),
		NewAlertingRule(
			"ThanosCompactBucketHighOperationFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_objstore_bucket_operation_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-compact.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_objstore_bucket_operations_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-compact.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosCompact,
				"description": "Thanos Compact {{$labels.job}} in {{$labels.namespace}} Bucket is failing to execute {{$value | humanize}}% of operations.",
				"message":     "Thanos Compact {{$labels.job}} in {{$labels.namespace}} Bucket is failing to execute {{$value | humanize}}% of operations.",
				"runbook":     runbookThanosCompactBucketHighOperationFailures,
				"summary":     "Thanos Compact Bucket is having a high number of operation failures.",
			},
		),
		NewAlertingRule(
			"ThanosCompactHasNotRun",
			promqlbuilder.Gtr(
				promqlbuilder.Div(
					promqlbuilder.Div(
						promqlbuilder.Sub(
							promqlbuilder.Time(),
							promqlbuilder.Max(
								promqlbuilder.MaxOverTime(
									matrix.New(
										vector.New(
											vector.WithMetricName("thanos_objstore_bucket_last_successful_upload_time"),
											vector.WithLabelMatchers(
												label.New("job").EqualRegexp("thanos-compact.*"),
											),
										),
										matrix.WithRange(24*time.Hour),
									),
								),
							).By("namespace", "job"),
						),
						promqlbuilder.NewNumber(60),
					),
					promqlbuilder.NewNumber(60),
				),
				promqlbuilder.NewNumber(24),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosCompact,
				"description": "Thanos Compact {{$labels.job}} in {{$labels.namespace}} has not uploaded anything for 24 hours.",
				"message":     "Thanos Compact {{$labels.job}} in {{$labels.namespace}} has not uploaded anything for 24 hours.",
				"runbook":     runbookThanosCompactHasNotRun,
				"summary":     "Thanos Compact has not uploaded anything for last 24 hours.",
			},
		),
	}

	return NewRuleGroup("thanos-compact", "", nil, rules)
}

func thanosQueryGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosQueryHttpRequestQueryErrorRateHigh",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_requests_total"),
										vector.WithLabelMatchers(
											label.New("code").EqualRegexp("5.."),
											label.New("job").EqualRegexp("thanos-query.*"),
											label.New("handler").Equal("query"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_requests_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
											label.New("handler").Equal("query"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosQuery,
				"description": "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of \"query\" requests.",
				"message":     "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of \"query\" requests.",
				"runbook":     runbookThanosQueryHttpRequestQueryErrorRateHigh,
				"summary":     "Thanos Query is failing to handle requests.",
			},
		),
		NewAlertingRule(
			"ThanosQueryGrpcServerErrorRate",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_handled_total"),
										vector.WithLabelMatchers(
											label.New("grpc_code").EqualRegexp("Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"),
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_started_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosQuery,
				"description": "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"message":     "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosQueryGrpcServerErrorRate,
				"summary":     "Thanos Query is failing to handle requests.",
			},
		),
		NewAlertingRule(
			"ThanosQueryGrpcClientErrorRate",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_client_handled_total"),
										vector.WithLabelMatchers(
											label.New("grpc_code").NotEqual("OK"),
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_client_started_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosQuery,
				"description": "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to send {{$value | humanize}}% of requests.",
				"message":     "Thanos Query {{$labels.job}} in {{$labels.namespace}} is failing to send {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosQueryGrpcClientErrorRate,
				"summary":     "Thanos Query is failing to send requests.",
			},
		),
		NewAlertingRule(
			"ThanosQueryHighDNSFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_query_store_apis_dns_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_query_store_apis_dns_lookups_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(1),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosQuery,
				"description": "Thanos Query {{$labels.job}} in {{$labels.namespace}} have {{$value | humanize}}% of failing DNS queries for store endpoints.",
				"message":     "Thanos Query {{$labels.job}} in {{$labels.namespace}} have {{$value | humanize}}% of failing DNS queries for store endpoints.",
				"runbook":     runbookThanosQueryHighDNSFailures,
				"summary":     "Thanos Query is having high number of DNS failures.",
			},
		),
		NewAlertingRule(
			"ThanosQueryInstantLatencyHigh",
			promqlbuilder.And(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_request_duration_seconds_bucket"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-query.*"),
											label.New("handler").Equal("query"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "le"),
					),
					promqlbuilder.NewNumber(90),
				),
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("http_request_duration_seconds_count"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-query.*"),
										label.New("handler").Equal("query"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job"),
					promqlbuilder.NewNumber(0),
				),
			),
			"10m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosQuery,
				"description": "Thanos Query {{$labels.job}} in {{$labels.namespace}} has a 99th percentile latency of {{$value}} seconds for instant queries.",
				"message":     "Thanos Query {{$labels.job}} in {{$labels.namespace}} has a 99th percentile latency of {{$value}} seconds for instant queries.",
				"runbook":     runbookThanosQueryInstantLatencyHigh,
				"summary":     "Thanos Query has high latency for queries.",
			},
		),
	}

	return NewRuleGroup("thanos-query", "", nil, rules)
}

func thanosReceiveGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosReceiveHttpRequestErrorRateHigh",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_requests_total"),
										vector.WithLabelMatchers(
											label.New("code").EqualRegexp("5.."),
											label.New("job").EqualRegexp("thanos-receive-router.*"),
											label.New("handler").Equal("receive"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_requests_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-receive-router.*"),
											label.New("handler").Equal("receive"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosReceiveHttpRequestErrorRateHigh,
				"summary":     "Thanos Receive is failing to handle requests.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveHttpRequestLatencyHigh",
			promqlbuilder.And(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("http_request_duration_seconds_bucket"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-receive-router.*"),
											label.New("handler").Equal("receive"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "le"),
					),
					promqlbuilder.NewNumber(10),
				),
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("http_request_duration_seconds_count"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-receive-router.*"),
										label.New("handler").Equal("receive"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job"),
					promqlbuilder.NewNumber(0),
				),
			),
			"10m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has a 99th percentile latency of {{ $value }} seconds for requests.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has a 99th percentile latency of {{ $value }} seconds for requests.",
				"runbook":     runbookThanosReceiveHttpRequestLatencyHigh,
				"summary":     "Thanos Receive has high HTTP requests latency.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveHighReplicationFailures",
			promqlbuilder.And(
				promqlbuilder.Gtr(
					vector.New(
						vector.WithMetricName("thanos_receive_replication_factor"),
					),
					promqlbuilder.NewNumber(1),
				),
				promqlbuilder.Mul(
					promqlbuilder.Gtr(
						promqlbuilder.Div(
							promqlbuilder.Sum(
								promqlbuilder.Rate(
									matrix.New(
										vector.New(
											vector.WithMetricName("thanos_receive_replications_total"),
											vector.WithLabelMatchers(
												label.New("result").Equal("error"),
												label.New("job").EqualRegexp("thanos-receive-router.*"),
											),
										),
										matrix.WithRange(5*time.Minute),
									),
								),
							).By("namespace", "job"),
							promqlbuilder.Sum(
								promqlbuilder.Rate(
									matrix.New(
										vector.New(
											vector.WithMetricName("thanos_receive_replications_total"),
											vector.WithLabelMatchers(
												label.New("job").EqualRegexp("thanos-receive-router.*"),
											),
										),
										matrix.WithRange(5*time.Minute),
									),
								),
							).By("namespace", "job"),
						),
						promqlbuilder.Div(
							promqlbuilder.Max(
								promqlbuilder.Floor(
									promqlbuilder.Div(
										promqlbuilder.Add(
											vector.New(
												vector.WithMetricName("thanos_receive_replication_factor"),
												vector.WithLabelMatchers(
													label.New("job").EqualRegexp("thanos-receive-router.*"),
												),
											),
											promqlbuilder.NewNumber(1),
										),
										promqlbuilder.NewNumber(2),
									),
								),
							).By("namespace", "job"),
							promqlbuilder.Max(
								vector.New(
									vector.WithMetricName("thanos_receive_hashring_nodes"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-receive-router.*"),
									),
								),
							).By("namespace", "job"),
						),
					),
					promqlbuilder.NewNumber(100),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to replicate {{$value | humanize}}% of requests.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to replicate {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosReceiveHighReplicationFailures,
				"summary":     "Thanos Receive is having high number of replication failures.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveHighForwardRequestFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_receive_forward_requests_total"),
										vector.WithLabelMatchers(
											label.New("result").Equal("error"),
											label.New("job").EqualRegexp("thanos-receive-router.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_receive_forward_requests_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-receive-router.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(20),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to forward {{$value | humanize}}% of requests.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to forward {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosReceiveHighForwardRequestFailures,
				"summary":     "Thanos Receive is failing to forward requests.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveHighHashringFileRefreshFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Div(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("thanos_receive_hashrings_file_errors_total"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-receive-router.*"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job"),
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("thanos_receive_hashrings_file_refreshes_total"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-receive-router.*"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job"),
				),
				promqlbuilder.NewNumber(0),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to refresh hashring file, {{$value | humanize}} of attempts failed.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing to refresh hashring file, {{$value | humanize}} of attempts failed.",
				"runbook":     runbookThanosReceiveHighHashringFileRefreshFailures,
				"summary":     "Thanos Receive is failing to refresh hasring file.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveConfigReloadFailure",
			promqlbuilder.Neq(
				promqlbuilder.Avg(
					vector.New(
						vector.WithMetricName("thanos_receive_config_last_reload_successful"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-receive-router.*"),
						),
					),
				).By("namespace", "job"),
				promqlbuilder.NewNumber(1),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has not been able to reload hashring configurations.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has not been able to reload hashring configurations.",
				"runbook":     runbookThanosReceiveConfigReloadFailure,
				"summary":     "Thanos Receive has not been able to reload configuration.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveNoUpload",
			promqlbuilder.Add(
				promqlbuilder.Sub(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-receive-ingester.*"),
						),
					),
					promqlbuilder.NewNumber(1),
				),
				promqlbuilder.Eqlc(
					promqlbuilder.Sum(
						promqlbuilder.Increase(
							matrix.New(
								vector.New(
									vector.WithMetricName("thanos_shipper_uploads_total"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-receive-ingester.*"),
									),
								),
								matrix.WithRange(3*time.Hour),
							),
						),
					).By("namespace", "job", "instance"),
					promqlbuilder.NewNumber(0),
				),
			).On("namespace", "job", "instance"),
			"3h",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.instance}} in {{$labels.namespace}} has not uploaded latest data to object storage.",
				"message":     "Thanos Receive {{$labels.instance}} in {{$labels.namespace}} has not uploaded latest data to object storage.",
				"runbook":     runbookThanosReceiveNoUpload,
				"summary":     "Thanos Receive has not uploaded latest data to object storage.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveLimitsConfigReloadFailure",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					promqlbuilder.Increase(
						matrix.New(
							vector.New(
								vector.WithMetricName("thanos_receive_limits_config_reload_err_total"),
								vector.WithLabelMatchers(
									label.New("job").EqualRegexp("thanos-receive-router.*"),
								),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				).By("namespace", "job"),
				promqlbuilder.NewNumber(0),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has not been able to reload the limits configuration.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} has not been able to reload the limits configuration.",
				"runbook":     runbookThanosReceiveLimitsConfigReloadFailure,
				"summary":     "Thanos Receive has not been able to reload the limits configuration.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveLimitsHighMetaMonitoringQueriesFailureRate",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Increase(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_receive_metamonitoring_failed_queries_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-receive-router.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.NewNumber(20),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(20),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing for {{$value | humanize}}% of meta monitoring queries.",
				"message":     "Thanos Receive {{$labels.job}} in {{$labels.namespace}} is failing for {{$value | humanize}}% of meta monitoring queries.",
				"runbook":     runbookThanosReceiveLimitsHighMetaMonitoringQueriesFailureRate,
				"summary":     "Thanos Receive has not been able to update the number of head series.",
			},
		),
		NewAlertingRule(
			"ThanosReceiveTenantLimitedByHeadSeries",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					promqlbuilder.Increase(
						matrix.New(
							vector.New(
								vector.WithMetricName("thanos_receive_head_series_limited_requests_total"),
								vector.WithLabelMatchers(
									label.New("job").EqualRegexp("thanos-receive-router.*"),
								),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				).By("namespace", "job", "tenant"),
				promqlbuilder.NewNumber(0),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosReceive,
				"description": "Thanos Receive tenant {{$labels.tenant}} in {{$labels.namespace}} is limited by head series.",
				"message":     "Thanos Receive tenant {{$labels.tenant}} in {{$labels.namespace}} is limited by head series.",
				"runbook":     runbookThanosReceiveTenantLimitedByHeadSeries,
				"summary":     "A Thanos Receive tenant is limited by head series.",
			},
		),
	}

	return NewRuleGroup("thanos-receive", "", nil, rules)
}

func thanosStoreGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosStoreGrpcErrorRate",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_handled_total"),
										vector.WithLabelMatchers(
											label.New("grpc_code").EqualRegexp("Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"),
											label.New("job").EqualRegexp("thanos-store.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_started_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-store.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosStore,
				"description": "Thanos Store {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"message":     "Thanos Store {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosStoreGrpcErrorRate,
				"summary":     "Thanos Store is failing to handle gRPC requests.",
			},
		),
		NewAlertingRule(
			"ThanosStoreBucketHighOperationFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_objstore_bucket_operation_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-store.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_objstore_bucket_operations_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-store.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosStore,
				"description": "Thanos Store {{$labels.job}} in {{$labels.namespace}} Bucket is failing to execute {{$value | humanize}}% of operations.",
				"message":     "Thanos Store {{$labels.job}} in {{$labels.namespace}} Bucket is failing to execute {{$value | humanize}}% of operations.",
				"runbook":     runbookThanosStoreBucketHighOperationFailures,
				"summary":     "Thanos Store Bucket is failing to execute operations.",
			},
		),
		NewAlertingRule(
			"ThanosStoreObjstoreOperationLatencyHigh",
			promqlbuilder.And(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_objstore_bucket_operation_duration_seconds_bucket"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-store.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "le"),
					),
					promqlbuilder.NewNumber(7),
				),
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("thanos_objstore_bucket_operation_duration_seconds_count"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-store.*"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job"),
					promqlbuilder.NewNumber(0),
				),
			),
			"10m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosStore,
				"description": "Thanos Store {{$labels.job}} in {{$labels.namespace}} Bucket has a 99th percentile latency of {{$value}} seconds for the bucket operations.",
				"message":     "Thanos Store {{$labels.job}} in {{$labels.namespace}} Bucket has a 99th percentile latency of {{$value}} seconds for the bucket operations.",
				"runbook":     runbookThanosStoreObjstoreOperationLatencyHigh,
				"summary":     "Thanos Store is having high latency for bucket operations.",
			},
		),
	}

	return NewRuleGroup("thanos-store", "", nil, rules)
}

func thanosRuleGroup() monitoringv1.RuleGroup {
	rules := []monitoringv1.Rule{
		NewAlertingRule(
			"ThanosRuleQueueIsDroppingAlerts",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					promqlbuilder.Rate(
						matrix.New(
							vector.New(
								vector.WithMetricName("thanos_alert_queue_alerts_dropped_total"),
								vector.WithLabelMatchers(
									label.New("job").EqualRegexp("thanos-ruler.*"),
								),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				).By("namespace", "job", "instance"),
				promqlbuilder.NewNumber(0),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to queue alerts.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to queue alerts.",
				"runbook":     runbookThanosRuleQueueIsDroppingAlerts,
				"summary":     "Thanos Rule is failing to queue alerts.",
			},
		),
		NewAlertingRule(
			"ThanosRuleSenderIsFailingAlerts",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					promqlbuilder.Rate(
						matrix.New(
							vector.New(
								vector.WithMetricName("thanos_alert_sender_alerts_dropped_total"),
								vector.WithLabelMatchers(
									label.New("job").EqualRegexp("thanos-ruler.*"),
								),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				).By("namespace", "job", "instance"),
				promqlbuilder.NewNumber(0),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to send alerts to alertmanager.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to send alerts to alertmanager.",
				"runbook":     runbookThanosRuleSenderIsFailingAlerts,
				"summary":     "Thanos Rule is failing to send alerts to alertmanager.",
			},
		),
		NewAlertingRule(
			"ThanosRuleHighRuleEvaluationFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("prometheus_rule_evaluation_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("prometheus_rule_evaluations_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to evaluate rules.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} is failing to evaluate rules.",
				"runbook":     runbookThanosRuleHighRuleEvaluationFailures,
				"summary":     "Thanos Rule is failing to evaluate rules.",
			},
		),
		NewAlertingRule(
			"ThanosRuleHighRuleEvaluationWarnings",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					promqlbuilder.Rate(
						matrix.New(
							vector.New(
								vector.WithMetricName("thanos_rule_evaluation_with_warnings_total"),
								vector.WithLabelMatchers(
									label.New("job").EqualRegexp("thanos-ruler.*"),
								),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				).By("namespace", "job", "instance"),
				promqlbuilder.NewNumber(0),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has high number of evaluation warnings.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has high number of evaluation warnings.",
				"runbook":     runbookThanosRuleHighRuleEvaluationWarnings,
				"summary":     "Thanos Rule has high number of evaluation warnings.",
			},
		),
		NewAlertingRule(
			"ThanosRuleRuleEvaluationLatencyHigh",
			promqlbuilder.Gtr(
				promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName("prometheus_rule_group_last_duration_seconds"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-ruler.*"),
						),
					),
				).By("namespace", "job", "instance", "rule_group"),
				promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName("prometheus_rule_group_interval_seconds"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-ruler.*"),
						),
					),
				).By("namespace", "job", "instance", "rule_group"),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has higher evaluation latency than interval for {{$labels.rule_group}}.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has higher evaluation latency than interval for {{$labels.rule_group}}.",
				"runbook":     runbookThanosRuleRuleEvaluationLatencyHigh,
				"summary":     "Thanos Rule has high rule evaluation latency.",
			},
		),
		NewAlertingRule(
			"ThanosRuleGrpcErrorRate",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_handled_total"),
										vector.WithLabelMatchers(
											label.New("grpc_code").EqualRegexp("Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"),
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("grpc_server_started_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(5),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"message":     "Thanos Rule {{$labels.job}} in {{$labels.namespace}} is failing to handle {{$value | humanize}}% of requests.",
				"runbook":     runbookThanosRuleGrpcErrorRate,
				"summary":     "Thanos Rule is failing to handle grpc requests.",
			},
		),
		NewAlertingRule(
			"ThanosRuleConfigReloadFailure",
			promqlbuilder.Neq(
				promqlbuilder.Avg(
					vector.New(
						vector.WithMetricName("thanos_rule_config_last_reload_successful"),
						vector.WithLabelMatchers(
							label.New("job").EqualRegexp("thanos-ruler.*"),
						),
					),
				).By("namespace", "job", "instance"),
				promqlbuilder.NewNumber(1),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has not been able to reload its configuration.",
				"message":     "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has not been able to reload its configuration.",
				"runbook":     runbookThanosRuleConfigReloadFailure,
				"summary":     "Thanos Rule has not been able to reload configuration.",
			},
		),
		NewAlertingRule(
			"ThanosRuleQueryHighDNSFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_rule_query_apis_dns_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_rule_query_apis_dns_lookups_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(1),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has {{$value | humanize}}% of failing DNS queries for query endpoints.",
				"message":     "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has {{$value | humanize}}% of failing DNS queries for query endpoints.",
				"runbook":     runbookThanosRuleQueryHighDNSFailures,
				"summary":     "Thanos Rule is having high number of DNS failures.",
			},
		),
		NewAlertingRule(
			"ThanosRuleAlertmanagerHighDNSFailures",
			promqlbuilder.Gtr(
				promqlbuilder.Mul(
					promqlbuilder.Div(
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_rule_alertmanagers_dns_failures_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("thanos_rule_alertmanagers_dns_lookups_total"),
										vector.WithLabelMatchers(
											label.New("job").EqualRegexp("thanos-ruler.*"),
										),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("namespace", "job", "instance"),
					),
					promqlbuilder.NewNumber(100),
				),
				promqlbuilder.NewNumber(1),
			),
			"15m",
			map[string]string{
				"service":  "rhobs",
				"severity": "medium",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has {{$value | humanize}}% of failing DNS queries for Alertmanager endpoints.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} has {{$value | humanize}}% of failing DNS queries for Alertmanager endpoints.",
				"runbook":     runbookThanosRuleAlertmanagerHighDNSFailures,
				"summary":     "Thanos Rule is having high number of DNS failures.",
			},
		),
		NewAlertingRule(
			"ThanosRuleNoEvaluationFor10Intervals",
			promqlbuilder.Gtr(
				promqlbuilder.Sub(
					promqlbuilder.Time(),
					promqlbuilder.Max(
						vector.New(
							vector.WithMetricName("prometheus_rule_group_last_evaluation_timestamp_seconds"),
							vector.WithLabelMatchers(
								label.New("job").EqualRegexp("thanos-ruler.*"),
							),
						),
					).By("namespace", "job", "instance", "group"),
				),
				promqlbuilder.Mul(
					promqlbuilder.NewNumber(10),
					promqlbuilder.Max(
						vector.New(
							vector.WithMetricName("prometheus_rule_group_interval_seconds"),
							vector.WithLabelMatchers(
								label.New("job").EqualRegexp("thanos-ruler.*"),
							),
						),
					).By("namespace", "job", "instance", "group"),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "high",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has rule groups that did not evaluate for at least 10x of their expected interval.",
				"message":     "Thanos Rule {{$labels.job}} in {{$labels.namespace}} has rule groups that did not evaluate for at least 10x of their expected interval.",
				"runbook":     runbookThanosRuleNoEvaluationFor10Intervals,
				"summary":     "Thanos Rule has rule groups that did not evaluate for 10 intervals.",
			},
		),
		NewAlertingRule(
			"ThanosNoRuleEvaluations",
			promqlbuilder.And(
				promqlbuilder.Lte(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("prometheus_rule_evaluations_total"),
									vector.WithLabelMatchers(
										label.New("job").EqualRegexp("thanos-ruler.*"),
									),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job", "instance"),
					promqlbuilder.NewNumber(0),
				),
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						vector.New(
							vector.WithMetricName("thanos_rule_loaded_rules"),
							vector.WithLabelMatchers(
								label.New("job").EqualRegexp("thanos-ruler.*"),
							),
						),
					).By("namespace", "job", "instance"),
					promqlbuilder.NewNumber(0),
				),
			),
			"5m",
			map[string]string{
				"service":  "rhobs",
				"severity": "critical",
			},
			map[string]string{
				"dashboard":   dashboardThanosRule,
				"description": "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} did not perform any rule evaluations in the past 10 minutes.",
				"message":     "Thanos Rule {{$labels.instance}} in {{$labels.namespace}} did not perform any rule evaluations in the past 10 minutes.",
				"runbook":     runbookThanosNoRuleEvaluations,
				"summary":     "Thanos Rule did not perform any rule evaluations.",
			},
		),
	}

	return NewRuleGroup("thanos-rule", "", nil, rules)
}
