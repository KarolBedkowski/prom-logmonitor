//
// metrics.go
// Copyright (C) Karol Będkowski, 2017
//
// Distributed under terms of the GPLv3 license.
//
package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"sort"
)

type metricsGroup struct {
	lineMatchedCntr *prometheus.CounterVec
	lineLastMatch   *prometheus.GaugeVec
	valuesExtracted *prometheus.GaugeVec
}

func newMetricsGroup(metric string, labels []string) metricsGroup {
	mg := metricsGroup{
		lineMatchedCntr: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metric,
				Help: "Total number lines matched by worker",
			},
			labels,
		),
		lineLastMatch: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: metric,
				Name:      "last_match_seconds",
				Help:      "Last line match unix time",
			},
			labels,
		),
		valuesExtracted: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: metric,
				Name:      "value",
				Help:      "Values extracted from log files",
			},
			labels,
		),
	}
	mg.register()
	return mg
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

// MetricCollection group prometheus collectors for configured metrics
type MetricCollection struct {
	metrics map[string]metricsGroup
}

// NewMetricCollection create empty MetricCollection
func NewMetricCollection() *MetricCollection {
	return &MetricCollection{
		metrics: make(map[string]metricsGroup),
	}
}

// RegisterMetrics create & register collectors according to configuration
func (m *MetricCollection) RegisterMetrics(c *Configuration) {
	for _, f := range c.Workers {
		if f.Disabled {
			continue
		}

		for _, cm := range f.Metrics {
			if _, exists := m.metrics[cm.Name]; exists {
				continue
			}

			var labels []string
			if len(cm.Labels) > 0 {
				for k := range cm.Labels {
					labels = append(labels, k)
				}
				sort.Strings(labels)
				labels = append([]string{"file"}, labels...)
			} else {
				labels = []string{"file"}
			}

			m.metrics[cm.Name] = newMetricsGroup(cm.Name, labels)
			log.Debugf("Registered %s with labels: %#v", cm.Name, labels)
		}
	}
}

// UnregisterMetrics remove all configured collectors
func (m *MetricCollection) UnregisterMetrics() {
	for _, mg := range m.metrics {
		mg.unregister()
	}
	m.metrics = make(map[string]metricsGroup)
}

// Observe register event for metrics and labels
func (m *MetricCollection) Observe(metric string, labels []string) {
	log.Debugf("Observe: %s %#v", metric, labels)
	mg := m.metrics[metric]
	mg.lineMatchedCntr.WithLabelValues(labels...).Inc()
	mg.lineLastMatch.WithLabelValues(labels...).SetToCurrentTime()
}

// ObserveWV register event for metrics and labels and store value
func (m *MetricCollection) ObserveWV(metric string, labels []string, value float64) {
	log.Debugf("ObserveWV: %s %#v, %v", metric, labels, value)
	mg := m.metrics[metric]
	mg.lineMatchedCntr.WithLabelValues(labels...).Inc()
	mg.lineLastMatch.WithLabelValues(labels...).SetToCurrentTime()
	mg.valuesExtracted.WithLabelValues(labels...).Set(value)
}

var (
	lineProcessedCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_processed_total",
			Help:      "Total number lines processed by worker",
		},
		[]string{"file"},
	)

	lineErrosCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_read_errors_total",
			Help:      "Total number errors occurred while reading lines by worker",
		},
		[]string{"file"},
	)

	lineLastProcessed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "logmonitor",
			Name:      "line_last_processed_seconds",
			Help:      "Last line processed unix time",
		},
		[]string{"file"},
	)
)

func init() {
	prometheus.MustRegister(lineProcessedCntr)
	prometheus.MustRegister(lineLastProcessed)
	prometheus.MustRegister(lineErrosCntr)
}

// ObserveReadError mark read file error
func ObserveReadError(filename string) {
	lineErrosCntr.WithLabelValues(filename).Inc()
}

// ObserveReadSuccess mark read file success
func ObserveReadSuccess(filename string) {
	lineProcessedCntr.WithLabelValues(filename).Inc()
	lineLastProcessed.WithLabelValues(filename).SetToCurrentTime()
}
