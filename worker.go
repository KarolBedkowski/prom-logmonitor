//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"regexp"
	"strings"
)

var (
	lineProcessedCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_processed_total",
			Help:      "Total number lines processed by worker",
		},
		[]string{"metric"},
	)

	lineMatchedCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_matched_total",
			Help:      "Total number lines matched by worker",
		},
		[]string{"metric"},
	)

	lineErrosCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_read_errors_total",
			Help:      "Total number errors occurred while reading lines by worker",
		},
		[]string{"metric"},
	)

	lineLastMatch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "logmonitor",
			Name:      "line_last_match_seconds",
			Help:      "Last line match unix time",
		},
		[]string{"metric"},
	)
)

func init() {
	prometheus.MustRegister(lineProcessedCntr)
	prometheus.MustRegister(lineMatchedCntr)
	prometheus.MustRegister(lineLastMatch)
}

// Filters configure include/exclude patterns
type Filters struct {
	includes []*regexp.Regexp
	excludes []*regexp.Regexp
}

// BuildFilters build list of patterns according to configuration
func BuildFilters(patterns []*Filter) (fs []*Filters, err error) {
	for _, p := range patterns {
		f := &Filters{}

		for _, i := range p.Include {
			r, err := regexp.Compile(i)
			if err != nil {
				return nil, fmt.Errorf(
					"error compile pattern 'include' '%s': %s", i, err)
			}
			f.includes = append(f.includes, r)
		}
		for _, e := range p.Exclude {
			r, err := regexp.Compile(e)
			if err != nil {
				return nil, fmt.Errorf(
					"error compile pattern 'exclude' '%s': %s", e, err)
			}
			f.excludes = append(f.excludes, r)
		}

		if len(f.includes) > 0 || len(f.excludes) > 0 {
			fs = append(fs, f)
		}
	}
	return
}

func (f *Filters) match(line string) (match bool) {
	if len(f.includes) == 0 {
		// accept all lines
		match = true
	} else {
		for _, r := range f.includes {
			if r.MatchString(line) {
				match = true
				break
			}
		}
	}

	if match {
		for _, e := range f.excludes {
			if e.MatchString(line) {
				match = false
				return
			}
		}
		return
	}

	return
}

// Reader is generic interface for log readers
type Reader interface {
	Start() error
	Read() (line string, err error)
	Stop() error
}

// Worker watch one file and report matched lines
type Worker struct {
	c *WorkerConf

	filters []*Filters

	log    log.Logger
	reader Reader
}

// NewWorker create new background worker according to configuration
func NewWorker(conf *WorkerConf) (worker *Worker, err error) {
	w := &Worker{
		c:   conf,
		log: log.With("metric", conf.Metric),
	}

	var reader Reader

	switch {
	case strings.HasPrefix(conf.File, ":sd_journal"):
		reader, err = NewSDJournalReader(conf, w.log)
	default:
		reader, err = NewPlainFileReader(conf, w.log)
	}

	if err != nil {
		return nil, err
	}
	w.reader = reader

	w.filters, err = BuildFilters(conf.Patterns)
	if err != nil {
		return nil, err
	}

	return w, nil
}

// Start worker (reading file)
func (w *Worker) Start() error {
	if w.reader != nil {
		w.log.Debug("start monitoring")

		if err := w.reader.Start(); err != nil {
			return err
		}

		go w.read()

		w.log.Info("worker started")
	}
	return nil
}

// Metric returns metric name from monitor
func (w *Worker) Metric() string {
	return w.c.Metric
}

// Stop worker
func (w *Worker) Stop() {
	if w.reader != nil {
		w.log.Debug("stop monitoring")
		w.reader.Stop()
	}
}

func (w *Worker) read() {
	for {
		line, err := w.reader.Read()
		if err != nil {
			w.log.Info("read file error:", err.Error())
			lineErrosCntr.WithLabelValues(w.c.Metric).Inc()
			continue
		}

		lineProcessedCntr.WithLabelValues(w.c.Metric).Inc()

		accepted := false
		// process file
		if len(w.filters) == 0 {
			accepted = true
		} else {
			for _, p := range w.filters {
				if p.match(line) {
					accepted = true
					break
				}
			}
		}

		if accepted {
			w.log.Debugf("accepted line '%v'", line)
			lineMatchedCntr.WithLabelValues(w.c.Metric).Inc()
			lineLastMatch.WithLabelValues(w.c.Metric).SetToCurrentTime()
		}
	}
}
