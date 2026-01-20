package main

import (
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/promql/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// appInterfacePrometheusRule allows adding schema field to the generated YAML.
type appInterfacePrometheusRule struct {
	Schema string `json:"$schema"`
	monitoringv1.PrometheusRule
}

const schemaPath = "/openshift/prometheus-rule-1.yml"

var (
	// Needed appSRE labels for prom-operator PromethuesRule file.
	ruleFileLabels = map[string]string{
		openshiftCustomerMonitoringLabel: openShiftClusterMonitoringLabelValue,
		"role":                           "alert-rules",
	}

	promRuleTypeMeta = metav1.TypeMeta{
		APIVersion: monitoring.GroupName + "/" + monitoringv1.Version,
		Kind:       monitoringv1.PrometheusRuleKind,
	}

	MustHavePrometheusRuleLabelsForTenantRules = map[string]string{
		"operator.thanos.io/prometheus-rule": "true",
	}
)

// NewPrometheusRule creates a new PrometheusRule object
// and ensures labels have the required keys for RHOBS.
func NewPrometheusTenantRule(
	name, namespace string,
	labels map[string]string,
	annotations map[string]string,
	groups []monitoringv1.RuleGroup) *monitoringv1.PrometheusRule {

	// Merge MustHavePrometheusRuleLabels with the provided labels
	for key, value := range MustHavePrometheusRuleLabelsForTenantRules {
		if _, exists := labels[key]; !exists {
			labels[key] = value
		}
	}

	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: groups,
		},
	}
}

// NewPrometheusRule creates a new PrometheusRule App-interface schema object
// and ensures labels have the required app-interface keys.
func NewPrometheusRuleForAppInterface(
	name string,
	labels map[string]string,
	annotations map[string]string,
	groups []monitoringv1.RuleGroup) *appInterfacePrometheusRule {

	// Merge ruleFileLabels with the provided labels
	for key, value := range ruleFileLabels {
		if _, exists := labels[key]; !exists {
			labels[key] = value
		}
	}

	return &appInterfacePrometheusRule{
		Schema: schemaPath,
		PrometheusRule: monitoringv1.PrometheusRule{
			TypeMeta: promRuleTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: groups,
			},
		},
	}
}

// NewRuleGroup creates a new RuleGroup object
func NewRuleGroup(name, interval string, labels map[string]string, rules []monitoringv1.Rule) monitoringv1.RuleGroup {
	intervalDuration := monitoringv1.Duration(interval)
	return monitoringv1.RuleGroup{
		Name:     name,
		Interval: &intervalDuration,
		Rules:    rules,
	}
}

// NewAlertingRule creates a new Rule object
func NewAlertingRule(alertName string, expr parser.Expr, forTime string, labels map[string]string, annotations map[string]string) monitoringv1.Rule {
	duration := monitoringv1.Duration(forTime)
	return monitoringv1.Rule{
		Alert:       alertName,
		Expr:        intstr.FromString(expr.Pretty(0)),
		For:         &duration,
		Labels:      labels,
		Annotations: annotations,
	}
}

// NewRecordingRule creates a new Rule object
func NewRecordingRule(recordName string, expr parser.Expr, labels map[string]string, annotations map[string]string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record:      recordName,
		Expr:        intstr.FromString(expr.Pretty(0)),
		Labels:      labels,
		Annotations: annotations,
	}
}
