package lokirules

import (
	"time"

	"github.com/perses/community-mixins/pkg/rules/rule-sdk/alerting"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/common"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/promtheusrule"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/recording"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/rulegroup"
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
)

func NewLokiRulesBuilder(
	namespace string,
	labels map[string]string,
	annotations map[string]string,
	options ...ConfigOption,
) (promtheusrule.Builder, error) {
	cfg := &RulesConfig{}
	for _, option := range options {
		option(cfg)
	}

	promRule, err := promtheusrule.New(
		"loki-rules",
		namespace,
		promtheusrule.Labels(labels),
		promtheusrule.Annotations(annotations),
		promtheusrule.AddRuleGroup("loki-record", cfg.lokiRecordingRules()...),
		promtheusrule.AddRuleGroup("loki-alerts", cfg.lokiAlerts()...),
	)

	return promRule, err
}

type RulesConfig struct {
	runbookURL        string
	serviceLabelValue string
}

type ConfigOption func(*RulesConfig)

func WithRunbookURL(url string) ConfigOption {
	return func(config *RulesConfig) {
		config.runbookURL = url
	}
}

func WithServiceLabelValue(labelValue string) ConfigOption {
	return func(config *RulesConfig) {
		config.serviceLabelValue = labelValue
	}
}

func (cfg *RulesConfig) lokiRecordingRules() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.AddRule(
			"job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m",
			recording.Expr(
				promqlbuilder.Sum(
					promqlbuilder.IRate(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_request_duration_seconds_count"),
							),
							matrix.WithRange(time.Minute),
						),
					),
				).By("job", "namespace", "route", "status_code")),
		),
	}
}

func (cfg *RulesConfig) lokiAlerts() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.AddRule(
			"LokiRequestErrors",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Mul(
						promqlbuilder.Div(
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
									vector.WithLabelMatchers(
										label.New("status_code").EqualRegexp("5xx"),
									),
								),
							).By("job", "namespace", "route"),
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
								),
							).By("job", "namespace", "route"),
						),
						promqlbuilder.NewNumber(100),
					),
					promqlbuilder.NewNumber(10),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-errors",
					`{{ $labels.job }} {{ $labels.route }} is experiencing {{ printf "%.2f" $value }}% errors.`,
					"At least 10% of requests are responded by 5xx server errors.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRequestPanics",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Increase(
							matrix.New(
								vector.New(
									vector.WithMetricName("loki_panic_total"),
								),
								matrix.WithRange(10*time.Minute),
							),
						),
					).By("job", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-panics",
					`{{ $labels.job }} is experiencing an increase of {{ $value }} panics.`,
					"A panic was triggered.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRequestLatency",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.IRate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_request_duration_seconds_bucket"),
										vector.WithLabelMatchers(
											label.New("route").NotEqualRegexp("(?i).*tail.*"),
										),
									),
									matrix.WithRange(2*time.Minute),
								),
							),
						).By("job", "namespace", "route", "le"),
					),
					promqlbuilder.NewNumber(1),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-latency",
					`{{ $labels.job }} {{ $labels.route }} is experiencing {{ printf "%.2f" $value }}s 99th percentile latency.`,
					"The 99th percentile is experiencing high latency (higher than 1 second).",
				),
			),
		),
		rulegroup.AddRule(
			"LokiTenantRateLimit",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Mul(
						promqlbuilder.Div(
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
									vector.WithLabelMatchers(
										label.New("status_code").Equal("429"),
									),
								),
							).By("job", "namespace", "route"),
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
								),
							).By("job", "namespace", "route"),
						),
						promqlbuilder.NewNumber(100),
					),
					promqlbuilder.NewNumber(10),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-tenant-rate-limit",
					`{{ $labels.job }} {{ $labels.route }} is experiencing 429 errors.`,
					"At least 10% of requests are responded with the rate limit error code.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiWritePathHighLoad",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						vector.New(
							vector.WithMetricName("loki_ingester_wal_replay_flushing"),
						),
					).By("job", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-write-path-high-load",
					`The write path is experiencing high load.`,
					"The write path is experiencing high load, causing backpressure storage flushing.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiReadPathHighLoad",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_logql_querystats_latency_seconds_bucket"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("job", "namespace", "le"),
					),
					promqlbuilder.NewNumber(30),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-read-path-high-load",
					`The read path is experiencing high load.`,
					"The read path has high volume of queries, causing longer response times.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiDiscardedSamplesWarning",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.IRate(
							matrix.New(
								vector.New(
									vector.WithMetricName("loki_discarded_samples_total"),
									vector.WithLabelMatchers(
										label.New("reason").NotEqual("rate_limited"),
										label.New("reason").NotEqual("per_stream_rate_limit"),
										label.New("reason").NotEqual("stream_limit"),
									),
								),
								matrix.WithRange(2*time.Minute),
							),
						),
					).By("namespace", "tenant", "reason"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-discarded-samples-warning",
					`Loki in namespace {{ $labels.namespace }} is discarding samples in the "{{ $labels.tenant }}" tenant during ingestion. Samples are discarded because of "{{ $labels.reason }}" at a rate of {{ .Value | humanize }} samples per second.`,
					"Loki is discarding samples during ingestion because they fail validation.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiIngesterFlushFailureRateCritical",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Div(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_ingester_chunks_flush_failures_total"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_ingester_chunks_flush_requests_total"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						),
					).By("namespace", "pod"),
					promqlbuilder.NewNumber(0.2),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ingester-flush-failure-rate-critical",
					`Loki ingester {{ $labels.pod }} in the namespace {{ $labels.namespace }} has a critical flush failure rate of {{ $value | humanizePercentage }} over the last 5 minutes. This requires immediate attention as data is not being flushed to the storage. Validate if the storage configuration is still valid and if the storage is still reachable. Current failure rate: {{ $value | humanizePercentage }} Threshold: 20%`,
					"Loki ingester has critical flush failure rate.",
				),
			),
		),
		rulegroup.AddRule(
			"LokistackComponentsNotReadyWarning",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.LabelReplace(
							vector.New(
								vector.WithMetricName("lokistack_status_condition"),
								vector.WithLabelMatchers(
									label.New("reason").Equal("ReadyComponents"),
									label.New("status").Equal("false"),
								),
							),
							"namespace", "$1", "stack_namespace", "(.+)",
						),
					).By("stack_name", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#lokistack-components-not-ready-warning",
					`The LokiStack "{{ $labels.stack_name }}" in namespace "{{ $labels.namespace }}" has components that are not ready.`,
					"One or more LokiStack components are not ready.",
				),
			),
		),
	}
}

func (cfg *RulesConfig) alertLabels(severity string) map[string]string {
	return map[string]string{
		"service":  cfg.serviceLabelValue,
		"severity": severity,
	}
}
