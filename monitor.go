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
)

func init() {
	prometheus.MustRegister(lineProcessedCntr)
	prometheus.MustRegister(lineMatchedCntr)
}

// Monitor watch one file and report matched lines
type Monitor struct {
	c *LogFile
	t *tail.Tail

	r []*regexp.Regexp

	log log.Logger
}

// NewMonitor create new worker from configuration
func NewMonitor(conf *LogFile) (*Monitor, error) {
	m := &Monitor{
		c:   conf,
		log: log.With("metric", conf.Metric),
	}

	for _, f := range conf.Filter {
		r, err := regexp.Compile(f)
		if err != nil {
			return nil, err
		}
		m.r = append(m.r, r)
	}

	return m, nil
}

// Start worker (reading file)
func (m *Monitor) Start() error {
	m.log.Debug("start monitoring")

	if m.t != nil {
		return fmt.Errorf("already failing")
	}

	t, err := tail.TailFile(m.c.File,
		tail.Config{
			Follow:   true,
			ReOpen:   true,
			Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
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

// Stop worker
func (m *Monitor) Stop() {
	if m.t != nil {
		m.log.Debug("stop monitoring")
		m.t.Stop()
	}
	m.t = nil
}

func (m *Monitor) readFile() {
	for line := range m.t.Lines {
		if line.Err != nil {
			m.log.Info("read file error:", line.Err.Error())
			lineErrosCntr.WithLabelValues(m.c.Metric).Inc()
			continue
		}
		lineProcessedCntr.WithLabelValues(m.c.Metric).Inc()
		accepted := false
		// process file
		if len(m.r) == 0 {
			accepted = true
		} else {
			for _, r := range m.r {
				if r.MatchString(line.Text) {
					accepted = true
					break
				}
			}
		}
		if accepted {
			m.log.Debugf("accepted line '%v'", line.Text)
			lineMatchedCntr.WithLabelValues(m.c.Metric).Inc()
		}
	}
}
