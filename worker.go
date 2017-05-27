//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"fmt"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"os"
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

type filters struct {
	includes []*regexp.Regexp
	excludes []*regexp.Regexp
}

func BuildFilters(patterns []*Filter) (fs []*filters, err error) {
	for _, p := range patterns {
		f := &filters{}

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

func (f *filters) match(line string) (match bool) {
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

type Worker interface {
	Start() error
	Metric() string
	Stop()
}

// Worker watch one file and report matched lines
type WorkerFile struct {
	c *WorkerConf
	t *tail.Tail

	filters []*filters

	log log.Logger
}

// NewWorker create new worker from configuration
func NewWorkerFile(conf *WorkerConf) (Worker, error) {
	m := &WorkerFile{
		c:   conf,
		log: log.With("metric", conf.Metric),
	}

	var err error
	m.filters, err = BuildFilters(conf.Patterns)
	if err != nil {
		return nil, fmt.Errorf("in '%s' for '%s' %s", err.Error())
	}

	return m, nil
}

// Start worker (reading file)
func (m *WorkerFile) Start() error {
	m.log.Debug("start monitoring")

	if m.t != nil {
		return fmt.Errorf("already failing")
	}

	t, err := tail.TailFile(m.c.File,
		tail.Config{
			Follow:   true,
			ReOpen:   true,
			Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
			Logger:   tail.DiscardingLogger,
		},
	)
	if err != nil {
		return err
	}
	m.t = t

	go m.readFile()

	m.log.Info("worker started")
	return nil
}

// Metric returns metric name from monitor
func (m *WorkerFile) Metric() string {
	return m.c.Metric
}

// Stop worker
func (m *WorkerFile) Stop() {
	if m.t != nil {
		m.log.Debug("stop monitoring")
		m.t.Stop()
	}
	m.t = nil
}

func (m *WorkerFile) readFile() {
	for line := range m.t.Lines {
		if line.Err != nil {
			m.log.Info("read file error:", line.Err.Error())
			lineErrosCntr.WithLabelValues(m.c.Metric).Inc()
			continue
		}
		lineProcessedCntr.WithLabelValues(m.c.Metric).Inc()
		accepted := false
		// process file
		if len(m.filters) == 0 {
			accepted = true
		} else {
			for _, p := range m.filters {
				if p.match(line.Text) {
					accepted = true
					break
				}
			}
		}
		if accepted {
			m.log.Debugf("accepted line '%v'", line.Text)
			lineMatchedCntr.WithLabelValues(m.c.Metric).Inc()
			lineLastMatch.WithLabelValues(m.c.Metric).SetToCurrentTime()
		}
	}
}

func NewWorker(conf *WorkerConf) (Worker, error) {
	if strings.HasPrefix(conf.File, ":sd_journal") {
		return NewWorkerSDJournal(conf)
	}
	return NewWorkerFile(conf)
}
