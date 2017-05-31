// +build sdjournal
//
// journal.go
// based on: https://gist.github.com/stuart-warren/240aaa21fa6f2d69457a,
// https://github.com/remerge/j2q/blob/master/service.go

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

// SDJournalReader watch one file and report matched lines
type SDJournalReader struct {
	c *WorkerConf
	j *C.struct_sd_journal

	log log.Logger
}

func init() {
	MustRegisterReader(&SDJournalReader{})
}

func (s *SDJournalReader) Match(conf *WorkerConf) (prio int) {
	if strings.HasPrefix(conf.File, ":sd_journal") {
		return 99
	}
	return -1
}

// NewSDJournalReader create reader for systemd journal
func (s *SDJournalReader) Create(conf *WorkerConf, l log.Logger) (Reader, error) {
	l.Infof("Monitoring '%s' by SystemD Journal Reader", conf.File)
	w := &SDJournalReader{
		c:   conf,
		log: l,
	}
	return w, nil
}

// Start worker (reading file)
func (s *SDJournalReader) Start() error {
	if s.j != nil {
		return fmt.Errorf("already reading")
	}

	s.j = new(C.struct_sd_journal)

	var flag C.int = C.SD_JOURNAL_LOCAL_ONLY
	switch s.c.File {
	case ":sd_journal/system":
		flag = C.SD_JOURNAL_SYSTEM
	case ":sd_journal/user":
		flag = C.SD_JOURNAL_CURRENT_USER
	case ":sd_journal/root":
		flag = C.SD_JOURNAL_OS_ROOT
	case ":sd_journal/local":
		flag = C.SD_JOURNAL_LOCAL_ONLY
	default:
		s.log.Warnf("unknown sd_journal type: '%v'; using local_only ", s.c.File)
	}
	if res := C.sd_journal_open(&s.j, flag); res < 0 {
		s.j = nil
		return fmt.Errorf("journal open error: %s", C.GoString(C.strerror(-res)))
	}

	C.sd_journal_seek_tail(s.j)
	return nil
}

// Stop worker
func (s *SDJournalReader) Stop() error {
	if s.j != nil {
		C.sd_journal_close(s.j)
		s.j = nil
	}
	return nil
}

func (s *SDJournalReader) Read() (line string, err error) {
	for {
		if res := C.sd_journal_next(s.j); res < 0 {
			continue
		} else if res == 0 {
			res = C.sd_journal_wait(s.j, 1000000)
			if res < 0 {
				s.log.Debugf("failed to wait for changes: %s", C.GoString(C.strerror(-res)))
			}
			continue
		}

		var cursor *C.char
		if res := C.sd_journal_get_cursor(s.j, &cursor); res < 0 {
			s.log.Warnf("failed to get cursor: %s", C.GoString(C.strerror(-res)))
			continue
		}

		C.sd_journal_restart_data(s.j)

		var data *C.char
		var length C.size_t
		for C.sd_journal_enumerate_data(s.j, (*unsafe.Pointer)(unsafe.Pointer(&data)), &length) > 0 {
			data := C.GoString(data)
			//s.log.Debugf("parts: '%v'", data)
			if strings.HasPrefix(data, "MESSAGE=") {
				parts := strings.Split(data, "=")
				line := parts[1]
				if line != "" {
					return line, nil
				}
			}
		}
	}
}
