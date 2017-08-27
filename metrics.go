//
// metrics.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the GPLv3 license.
//
package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"sort"
)

type metricsGroup struct {
	lineMatchedCntr *prometheus.CounterVec
	lineLastMatch   *prometheus.GaugeVec
	valuesExtracted *prometheus.GaugeVec
}

func (m *metricsGroup) register() {
	prometheus.Register(m.lineMatchedCntr)
	prometheus.Register(m.lineLastMatch)
	prometheus.Register(m.valuesExtracted)
}

func (m *metricsGroup) unregister() {
	prometheus.Unregister(m.lineMatchedCntr)
	prometheus.Unregister(m.lineLastMatch)
	prometheus.Unregister(m.valuesExtracted)
}

type MetricCollection struct {
	metrics map[string]metricsGroup
}

func NewMetricCollection() *MetricCollection {
	return &MetricCollection{
		metrics: make(map[string]metricsGroup),
	}
}

func getMetricsLabes(c *Configuration) map[string][]string {
	metricsLabels := make(map[string][]string)

	for _, f := range c.Workers {
		if f.Disabled {
			continue
		}

		for _, cm := range f.Metrics {
			if _, exists := metricsLabels[cm.Name]; exists {
				continue
			}

			var labels []string
			for k := range cm.Labels {
				labels = append(labels, k)
			}
			sort.Strings(labels)
			metricsLabels[cm.Name] = labels
		}
	}

	return metricsLabels
}

func (m *MetricCollection) RegisterMetrics(c *Configuration) {
	labels := getMetricsLabes(c)

	for _, f := range c.Workers {
		if f.Disabled {
			continue
		}

		for _, cm := range f.Metrics {
			if _, exists := m.metrics[cm.Name]; exists {
				continue
			}
			l := []string{"file"}
			if mlabels := labels[cm.Name]; len(mlabels) > 0 {
				l = append(l, mlabels...)
			}
			log.Debugf("metric: %#v, labels: %#v", cm, l)
			mg := metricsGroup{
				lineMatchedCntr: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Namespace: cm.Name,
						Name:      "lines_matched_total",
						Help:      "Total number lines matched by worker",
					},
					l,
				),
				lineLastMatch: prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Namespace: cm.Name,
						Name:      "line_last_match_seconds",
						Help:      "Last line match unix time",
					},
					l,
				),
				valuesExtracted: prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Namespace: cm.Name,
						Name:      "value",
						Help:      "Values extracted from log files",
					},
					l,
				),
			}
			m.metrics[cm.Name] = mg
			mg.register()
			log.Debugf("Registered %s with labels: %#v", cm.Name, l)
		}
	}
}

func (m *MetricCollection) UnregisterMetrics() {
	for _, mg := range m.metrics {
		mg.unregister()
	}
	m.metrics = make(map[string]metricsGroup)
}

func (m *MetricCollection) Observe(metric string, labels []string) {
	log.Debugf("Observe: %s %#v", metric, labels)
	mg := m.metrics[metric]
	mg.lineMatchedCntr.WithLabelValues(labels...).Inc()
	mg.lineLastMatch.WithLabelValues(labels...).SetToCurrentTime()
}

func (m *MetricCollection) ObserveWV(metric string, labels []string, value float64) {
	log.Debugf("ObserveWV: %s %#v, %v", metric, labels, value)
	mg := m.metrics[metric]
	mg.lineMatchedCntr.WithLabelValues(labels...).Inc()
	mg.lineLastMatch.WithLabelValues(labels...).SetToCurrentTime()
	mg.valuesExtracted.WithLabelValues(labels...).Set(value)
}
