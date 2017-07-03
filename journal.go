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
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"io/ioutil"
	"strings"
	"time"
	"unsafe"
)

// SDJournalReader watch one file and report matched lines
type SDJournalReader struct {
	c      *WorkerConf
	j      *C.struct_sd_journal
	cursor *C.char
	filter []string

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
		return errors.Errorf("already reading")
	}

	s.j = new(C.struct_sd_journal)

	var fname, args string
	if sr := strings.IndexRune(s.c.File, '?'); sr > 0 {
		fname = s.c.File[:sr]
		args = s.c.File[sr+1:]
	} else {
		fname = s.c.File
	}

	var flag C.int = C.SD_JOURNAL_LOCAL_ONLY
	switch fname {
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
		return errors.Errorf("journal open error: %s", C.GoString(C.strerror(-res)))
	}

	if s.c.StampFile == "" || !s.seekLastPos() {
		// move to end
		if res := C.sd_journal_seek_tail(s.j); res < 0 {
			s.Stop()
			return errors.Errorf("journal seek tail error: %s", C.GoString(C.strerror(-res)))
		}
	}

	if args != "" {
		s.filter = strings.Split(args, "&")
	}

	return nil
}

func (s *SDJournalReader) seekLastPos() (success bool) {
	s.log.Debugf("seek to last cursor; file: %s", s.c.StampFile)
	stamp, err := ioutil.ReadFile(s.c.StampFile)
	if err != nil {
		s.log.Infof("open stamp file %s error: %s", s.c.StampFile, err)
		return false
	}

	if len(stamp) == 0 {
		return
	}

	s.cursor = C.CString(string(stamp))
	if res := C.sd_journal_seek_cursor(s.j, s.cursor); res < 0 {
		s.log.Warnf("failed to seek last cursor: %s", C.GoString(C.strerror(-res)))
		return
	}

	if res := C.sd_journal_next_skip(s.j, 1); res < 0 {
		s.log.Warnf("failed to seek next: %s", C.GoString(C.strerror(-res)))
		return
	}

	s.log.Debugf("seek to last cursor success")
	return true
}

// Stop worker
func (s *SDJournalReader) Stop() error {
	if s.j != nil {
		C.sd_journal_close(s.j)
		s.j = nil
		if s.c.StampFile != "" {
			ioutil.WriteFile(s.c.StampFile, []byte(C.GoString(s.cursor)), 0644)
		}
	}
	return nil
}

func (s *SDJournalReader) Read() (line string, err error) {
	var res C.int
	var data *C.char
	var length C.size_t

	for {
		if s.j == nil {
			return
		}

		if res = C.sd_journal_next(s.j); res < 0 {
			s.log.Warnf("journal next error: %s", C.GoString(C.strerror(-res)))
			time.Sleep(time.Duration(1) * time.Second)
			continue
		} else if res == 0 {
			res = C.sd_journal_wait(s.j, 1000000)
			if res < 0 {
				s.log.Debugf("failed to wait for changes: %s", C.GoString(C.strerror(-res)))
			}
			continue
		}

		if res = C.sd_journal_get_cursor(s.j, &s.cursor); res < 0 {
			s.log.Warnf("failed to get cursor: %s", C.GoString(C.strerror(-res)))
			continue
		}

		C.sd_journal_restart_data(s.j)

		// number of arguments to find in record to accept record
		argsMissing := len(s.filter)

		for C.sd_journal_enumerate_data(s.j, (*unsafe.Pointer)(unsafe.Pointer(&data)), &length) > 0 {
			data := C.GoString(data)
			//s.log.Debugf("parts: '%v'", data)
			if len(data) > 8 && strings.HasPrefix(data, "MESSAGE=") {
				line = data[8:]
			}

			if argsMissing > 0 {
				for _, f := range s.filter {
					if f == data {
						argsMissing--
					}
				}
			}
		}

		if argsMissing == 0 {
			// record accepted
			return
		}
	}
}
