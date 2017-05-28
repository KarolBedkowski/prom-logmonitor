//
// journal.go
// From: https://gist.github.com/stuart-warren/240aaa21fa6f2d69457a

package main

// #include <stdio.h>
// #include <string.h>
// #include <systemd/sd-journal.h>
// #cgo LDFLAGS: -lsystemd
import "C"

import (
	"fmt"
	"github.com/prometheus/common/log"
	"strings"
	"unsafe"
)

// WorkerSDJournal watch one file and report matched lines
type WorkerSDJournal struct {
	c *WorkerConf
	j *C.struct_sd_journal

	filters []*filters

	log log.Logger
}

// NewWorker create new worker from configuration
func NewWorkerSDJournal(conf *WorkerConf) (Worker, error) {
	m := &WorkerSDJournal{
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
func (m *WorkerSDJournal) Start() error {
	m.log.Debug("start monitoring SD Journal")

	if m.j != nil {
		return fmt.Errorf("already failing")
	}

	m.j = new(C.struct_sd_journal)

	var flag C.int = C.SD_JOURNAL_LOCAL_ONLY
	switch m.c.File {
	case ":sd_journal/system":
		flag = C.SD_JOURNAL_SYSTEM
	case ":sd_journal/user":
		flag = C.SD_JOURNAL_CURRENT_USER
	case ":sd_journal/root":
		flag = C.SD_JOURNAL_OS_ROOT
	}
	if res := C.sd_journal_open(&m.j, flag); res < 0 {
		m.j = nil
		return fmt.Errorf("journal open error: %s", C.GoString(C.strerror(-res)))
	}

	C.sd_journal_seek_tail(m.j)

	go m.readFile()

	m.log.Info("worker started")
	return nil
}

// Metric returns metric name from monitor
func (m *WorkerSDJournal) Metric() string {
	return m.c.Metric
}

// Stop worker
func (m *WorkerSDJournal) Stop() {
	if m.j != nil {
		m.log.Debug("stop monitoring")
		C.sd_journal_close(m.j)
	}
	m.j = nil
}

func (m *WorkerSDJournal) readFile() {
	for {
		if res := C.sd_journal_next(m.j); res < 0 {
			continue
		} else if res == 0 {
			res = C.sd_journal_wait(m.j, 1000000)
			if res < 0 {
				m.log.Warnf("failed to wait for changes: %s", C.GoString(C.strerror(-res)))
			}
			continue
		}

		var cursor *C.char
		if res := C.sd_journal_get_cursor(m.j, &cursor); res < 0 {
			m.log.Warnf("failed to get cursor: %s", C.GoString(C.strerror(-res)))
			continue
		}

		var data *C.char
		var length C.size_t
		line := ""
		for C.sd_journal_restart_data(m.j); C.sd_journal_enumerate_data(m.j, (*unsafe.Pointer)(unsafe.Pointer(&data)), &length) > 0; {
			data := C.GoString(data)
			//m.log.Debugf("parts: '%v'", data)
			if strings.HasPrefix(data, "MESSAGE") {
				parts := strings.Split(data, "=")
				line = parts[1]
				break
			}
		}

		if line == "" {
			continue
		}

		lineProcessedCntr.WithLabelValues(m.c.Metric).Inc()
		accepted := false
		// process file
		if len(m.filters) == 0 {
			accepted = true
		} else {
			for _, p := range m.filters {
				if p.match(line) {
					accepted = true
					break
				}
			}
		}

		if accepted {
			m.log.Debugf("accepted line '%v'", line)
			lineMatchedCntr.WithLabelValues(m.c.Metric).Inc()
			lineLastMatch.WithLabelValues(m.c.Metric).SetToCurrentTime()
		}
	}
}
