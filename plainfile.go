//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"os"
)

// PlainFileReader read plain file
type PlainFileReader struct {
	c *WorkerConf
	t *tail.Tail

	log log.Logger
}

func init() {
	MustRegisterReader(&PlainFileReader{})
}

// Match reader to configuration file.
// PlainFileReader has very low priority and cannot be used when file start with ":"
func (p *PlainFileReader) Match(conf *WorkerConf) (prio int) {
	if conf.File[0] == ':' {
		return -1
	}
	return 0
}

// Create new reader for plain files
func (p *PlainFileReader) Create(conf *WorkerConf, l log.Logger) (pfr Reader, err error) {
	l.Infof("Monitoring '%s' by Plain File Reader", conf.File)
	pfr = &PlainFileReader{
		c:   conf,
		log: l,
	}
	return
}

// Start worker (reading file)
func (p *PlainFileReader) Start() error {
	if p.t != nil {
		return errors.Errorf("already reading")
	}

	t, err := tail.TailFile(p.c.File,
		tail.Config{
			Follow:   true,
			ReOpen:   true,
			Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
			Logger:   tail.DiscardingLogger,
		},
	)

	if err != nil {
		return errors.Wrap(err, "tail file error")
	}

	p.t = t
	return nil
}

// Stop reading plain file
func (p *PlainFileReader) Stop() error {
	if p.t != nil {
		p.t.Stop()
		p.t = nil
	}
	return nil
}

func (p *PlainFileReader) Read() (line string, err error) {
	if p.t == nil {
		return "", errors.New("file not opened")
	}

	if l, ok := <-p.t.Lines; ok {
		return l.Text, errors.Wrap(l.Err, "read line error")
	}

	// eof
	return "", nil
}
