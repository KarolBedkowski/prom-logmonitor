//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"regexp"
	"strconv"
	"sync"
)

// ReaderDef define interface for readers
type ReaderDef interface {
	Match(conf *WorkerConf) (prio int)
	Create(conf *WorkerConf, l log.Logger) (p Reader, err error)
}

var registeredReaders struct {
	mu      sync.RWMutex
	readers []ReaderDef
}

// MustRegisterReader try to register given reader
func MustRegisterReader(r ReaderDef) {
	registeredReaders.mu.Lock()
	defer registeredReaders.mu.Unlock()

	registeredReaders.readers = append(registeredReaders.readers, r)
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

	lineMatchedCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_matched_total",
			Help:      "Total number lines matched by worker",
		},
		[]string{"file", "metric"},
	)

	lineErrosCntr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logmonitor",
			Name:      "lines_read_errors_total",
			Help:      "Total number errors occurred while reading lines by worker",
		},
		[]string{"file"},
	)

	lineLastMatch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "logmonitor",
			Name:      "line_last_match_seconds",
			Help:      "Last line match unix time",
		},
		[]string{"file", "metric"},
	)
	lineLastProcessed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "logmonitor",
			Name:      "line_last_processed_seconds",
			Help:      "Last line processed unix time",
		},
		[]string{"file"},
	)

	valuesExtracted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "logmonitor",
			Name:      "value",
			Help:      "Values extracted from log files",
		},
		[]string{"file", "metric"},
	)
)

func init() {
	prometheus.MustRegister(lineProcessedCntr)
	prometheus.MustRegister(lineMatchedCntr)
	prometheus.MustRegister(lineErrosCntr)
	prometheus.MustRegister(lineLastMatch)
	prometheus.MustRegister(lineLastProcessed)
	prometheus.MustRegister(valuesExtracted)
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
				return nil, errors.Wrapf(err, "error compile pattern 'include' '%s'")
			}
			f.includes = append(f.includes, r)
		}
		for _, e := range p.Exclude {
			r, err := regexp.Compile(e)
			if err != nil {
				return nil, errors.Wrapf(err,
					"error compile pattern 'exclude' '%s'", e)
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

type metricFilters struct {
	name    string
	filters []*Filters

	extractPattern *regexp.Regexp
}

func (m metricFilters) String() string {
	return m.name
}

// Worker watch one file and report matched lines
type Worker struct {
	c *WorkerConf

	metrics []*metricFilters

	log    log.Logger
	reader Reader

	stopping bool
}

// NewWorker create new background worker according to configuration
// Each worker monitor only one file and one file can be monitored only
// by one worker.
func NewWorker(conf *WorkerConf) (worker *Worker, err error) {
	w := &Worker{
		c:   conf,
		log: log.With("file", conf.File),
	}

	var reader Reader

	var rd ReaderDef
	var prio = -1

	registeredReaders.mu.RLock()
	defer registeredReaders.mu.RUnlock()

	for _, r := range registeredReaders.readers {
		if p := r.Match(conf); p >= 0 && p > prio {
			rd = r
			prio = p
		}
	}

	if rd == nil {
		return nil, errors.Errorf("none of reader match configuration for %s", conf.File)
	}

	reader, err = rd.Create(conf, w.log)
	if err != nil {
		return nil, err
	}
	w.reader = reader

	for _, metric := range conf.Metrics {
		if metric.Disabled {
			continue
		}

		var ftrs []*Filters
		ftrs, err = BuildFilters(metric.Patterns)
		if err != nil {
			return nil, err
		}

		mf := &metricFilters{
			name:    metric.Name,
			filters: ftrs,
		}

		if metric.ValuePattern != "" {
			p, err := regexp.Compile(metric.ValuePattern)
			if err != nil {
				return nil, errors.Wrapf(err,
					"error compile extract pattern '%s'", metric.ValuePattern)
			}
			mf.extractPattern = p
			// if not defined filters; use variable pattern re for filtering
			if len(mf.filters) == 0 {
				mf.filters = []*Filters{&Filters{includes: []*regexp.Regexp{p}}}
			}
		}
		w.metrics = append(w.metrics, mf)
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

// Filename returns file monitored by worker
func (w *Worker) Filename() string {
	return w.c.File
}

// Stop worker
func (w *Worker) Stop() {
	w.stopping = true
	if w.reader != nil {
		w.log.Debug("stop monitoring")
		w.reader.Stop()
	}
}

func (w *Worker) read() {
	var line string
	var err error

	for {
		line, err = w.reader.Read()
		if err != nil {
			if w.stopping {
				return
			}

			w.log.Info("read file error:", err.Error())
			lineErrosCntr.WithLabelValues(w.c.File).Inc()
			continue
		}

		if line == "" {
			continue
		}

		lineProcessedCntr.WithLabelValues(w.c.File).Inc()
		lineLastProcessed.WithLabelValues(w.c.File).SetToCurrentTime()

		for _, mf := range w.metrics {
			//			w.log.Debugf("checking %s", mf)
			accepted := false
			// process file
			if len(mf.filters) == 0 {
				accepted = true
			} else {
				for _, p := range mf.filters {
					if p.match(line) {
						accepted = true
						break
					}
				}
			}

			if accepted {
				w.log.Debugf("accepted line '%v' to '%v' by '%v'", line, mf.name, mf.filters)
				lineMatchedCntr.WithLabelValues(w.c.File, mf.name).Inc()
				lineLastMatch.WithLabelValues(w.c.File, mf.name).SetToCurrentTime()

				if mf.extractPattern != nil {
					// extract value from line, convert to float64 and expose
					m := mf.extractPattern.FindStringSubmatch(line)
					if len(m) > 1 {
						if val, err := strconv.ParseFloat(m[1], 66); err == nil {
							valuesExtracted.WithLabelValues(w.c.File, mf.name).Set(val)
						} else {
							w.log.Info("convert '%v' in line '%v' to float failed: %s", m[1], line, err)
						}
					}
				}
			}
		}
	}
}
