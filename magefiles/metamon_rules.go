package main

import (
	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	alertmanagerrules "github.com/perses/community-mixins/pkg/rules/alertmanager"
	thanosrules "github.com/perses/community-mixins/pkg/rules/thanos"
	thanosoperatorrules "github.com/perses/community-mixins/pkg/rules/thanos-operator"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// Dashboard URLs
const (
	dashboardThanosCompact = "https://grafana.app-sre.devshift.net/d/651943d05a8123e32867b4673963f42b/thanos-compact?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosQuery   = "https://grafana.app-sre.devshift.net/d/98fde97ddeaf2981041745f1f2ba68c2/thanos-query?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosReceive = "https://grafana.app-sre.devshift.net/d/916a852b00ccc5ed81056644718fa4fb/thanos-receive?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosStore   = "https://grafana.app-sre.devshift.net/d/e832e8f26403d95fac0ea1c59837588b/thanos-store?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosRule    = "https://grafana.app-sre.devshift.net/d/35da848f5f92b2dc612e0c3a0577b8a1/thanos-rule?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"

	dashboardAlertmanager   = "https://grafana.app-sre.devshift.net/d/50b36e28785705570854022296f14821/alertmanager?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
	dashboardThanosOperator = "https://grafana.app-sre.devshift.net/d/3da9a026333052b2733299a69c302074/thanos-operator?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"
)

// Runbook URLs
const (
	runbookBaseURL = "https://github.com/rhobs/configuration/blob/main/docs/sop/observatorium.md"
)

func (b Build) Rules() error {
	b.ThanosRules()
	b.ThanosOperatorRules()
	b.AlertmanagerRules()
	b.SLORules()
	return nil
}

func (b Build) ThanosRules() {
	gen := b.o11yGenerator("thanos-rules")
	thanosRules(gen)
}

func thanosRules(gen *mimic.Generator) {
	gen.Add("thanos-rules.yaml", encoding.GhodssYAML("", ThanosPrometheusRule(false)))
	gen.Add("thanos-rules-non-critical.yaml", encoding.GhodssYAML("", ThanosPrometheusRule(true)))
	gen.Generate()
}

func (b Build) ThanosOperatorRules() {
	gen := b.o11yGenerator("thanos-operator-rules")
	thanosOperatorRules(gen)
}

func thanosOperatorRules(gen *mimic.Generator) {
	gen.Add("thanos-operator-rules.yaml", encoding.GhodssYAML("", ThanosOperatorPrometheusRule(false)))
	gen.Add("thanos-operator-rules-non-critical.yaml", encoding.GhodssYAML("", ThanosOperatorPrometheusRule(true)))
	gen.Generate()
}

func (b Build) AlertmanagerRules() {
	gen := b.o11yGenerator("alertmanager-rules")
	alertmanagerRules(gen)
}

func alertmanagerRules(gen *mimic.Generator) {
	gen.Add("alertmanager-rules.yaml", encoding.GhodssYAML("", AlertmanagerPrometheusRule(false)))
	gen.Add("alertmanager-rules-non-critical.yaml", encoding.GhodssYAML("", AlertmanagerPrometheusRule(true)))
	gen.Generate()
}

func ThanosPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := thanosrules.NewThanosRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "thanos",
			"app.kubernetes.io/name":      "thanos-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		thanosrules.WithRunbookURL(runbookBaseURL),
		thanosrules.WithServiceLabelValue("thanos"),
		thanosrules.WithCompactDashboardURL(dashboardThanosCompact),
		thanosrules.WithQueryDashboardURL(dashboardThanosQuery),
		thanosrules.WithReceiveDashboardURL(dashboardThanosReceive),
		thanosrules.WithStoreDashboardURL(dashboardThanosStore),
		thanosrules.WithRuleDashboardURL(dashboardThanosRule),
		thanosrules.WithServiceLabelValue(rhobsNextServiceLabel),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.PrometheusRule.Spec.Groups = ReplaceSummaryWithMessage(builder.PrometheusRule.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func ThanosOperatorPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := thanosoperatorrules.NewThanosOperatorRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "thanos-operator",
			"app.kubernetes.io/name":      "thanos-operator-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		thanosoperatorrules.WithRunbookURL(runbookBaseURL),
		thanosoperatorrules.WithServiceLabelValue(rhobsNextServiceLabel),
		thanosoperatorrules.WithDashboardURL(dashboardThanosOperator),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.PrometheusRule.Spec.Groups = ReplaceSummaryWithMessage(builder.PrometheusRule.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func AlertmanagerPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := alertmanagerrules.NewAlertmanagerRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "alertmanager",
			"app.kubernetes.io/name":      "alertmanager-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		alertmanagerrules.WithRunbookURL(runbookBaseURL),
		alertmanagerrules.WithServiceLabelValue(rhobsNextServiceLabel),
		alertmanagerrules.WithCriticalIntegrationSelectorRegexp("slack|pagerduty|email|webhook"),
		alertmanagerrules.WithNonCriticalIntegrationSelectorRegexp("slack|pagerduty|email|webhook"),
		alertmanagerrules.WithDashboardURL(dashboardAlertmanager),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.PrometheusRule.Spec.Groups = ReplaceSummaryWithMessage(builder.PrometheusRule.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func RuleNonCriticalPostProcessing(rule v1.PrometheusRule) v1.PrometheusRule {
	for i := range rule.Spec.Groups {
		for j := range rule.Spec.Groups[i].Rules {
			if v, ok := rule.Spec.Groups[i].Rules[j].Labels["severity"]; ok {
				switch v {
				case "critical":
					rule.Spec.Groups[i].Rules[j].Labels["severity"] = "high"
				case "warning":
					rule.Spec.Groups[i].Rules[j].Labels["severity"] = "medium"
				}
			}
		}
	}
	return rule
}

// Usually summary + description is the best practice, but this is so that we are compatible with app-interface schema.
// To be removed once https://issues.redhat.com/browse/APPSRE-7834 is closed
func ReplaceSummaryWithMessage(groups []v1.RuleGroup) []v1.RuleGroup {
	for i := range groups {
		for j := range groups[i].Rules {
			if groups[i].Rules[j].Annotations["summary"] != "" {
				groups[i].Rules[j].Annotations["message"] = groups[i].Rules[j].Annotations["summary"]
				delete(groups[i].Rules[j].Annotations, "summary")
			}
		}
	}
	return groups
}
