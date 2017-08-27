//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strconv"
	"sync"
)

// ReaderDef define interface for readers
type ReaderDef interface {
	Match(conf *WorkerConf) (prio int)
	Create(conf *WorkerConf, l logger) (p Reader, err error)
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

func getReaderForConf(conf *WorkerConf) (rd ReaderDef) {
	registeredReaders.mu.RLock()
	defer registeredReaders.mu.RUnlock()

	prio := -1

	for _, r := range registeredReaders.readers {
		if p := r.Match(conf); p >= 0 && p > prio {
			rd = r
			prio = p
		}
	}

	return
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

	metricsCollection *MetricCollection
)

func init() {
	prometheus.MustRegister(lineProcessedCntr)
	prometheus.MustRegister(lineLastProcessed)
	prometheus.MustRegister(lineErrosCntr)
}

func initMetrics(c *Configuration) {
	if metricsCollection != nil {
		metricsCollection.UnregisterMetrics()
	} else {
		metricsCollection = NewMetricCollection()
	}
	metricsCollection.RegisterMetrics(c)
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
				return false
			}
		}
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
	labels  []string

	extractPattern *regexp.Regexp
}

func (m metricFilters) String() string {
	return m.name
}

func (m *metricFilters) AcceptLine(line string) (accepted bool) {
	if len(m.filters) == 0 {
		return true
	}

	for _, p := range m.filters {
		if p.match(line) {
			return true
		}
	}

	return false
}

// Worker watch one file and report matched lines
type Worker struct {
	c *WorkerConf

	metrics []*metricFilters

	log    logger
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

	rd := getReaderForConf(conf)
	if rd == nil {
		return nil, errors.Errorf("none of readers can be used with for %s", conf.File)
	}

	if w.reader, err = rd.Create(conf, w.log); err != nil {
		return nil, errors.Wrapf(err, "create reader for %s error", conf.File)
	}

	for _, metric := range conf.Metrics {
		if metric.Disabled {
			continue
		}

		var ftrs []*Filters
		ftrs, err = BuildFilters(metric.Patterns)
		if err != nil {
			return nil, errors.Wrapf(err, "build filters for '%v' error", metric.Patterns)
		}

		mf := &metricFilters{
			name:    metric.Name,
			filters: ftrs,
			labels:  metric.StaticLabels,
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

		if w.stopping {
			return
		}

		if err != nil {
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
			if !mf.AcceptLine(line) {
				continue
			}

			log.Debugf("accepted: %v, by %#v", line, mf)

			if mf.extractPattern == nil {
				metricsCollection.Observe(mf.name, mf.labels)
			} else {
				// extract value from line, convert to float64 and expose
				m := mf.extractPattern.FindStringSubmatch(line)
				log.Debug("%#v", len(m))
				if len(m) > 1 {
					if val, err := strconv.ParseFloat(m[1], 64); err == nil {
						metricsCollection.ObserveWV(mf.name, mf.labels, val)
					} else {
						w.log.Info("convert '%v' in line '%v' to float failed: %s", m[1], line, err)
					}
				}
			}
		}
	}
}
