//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"os"
)

// PlainFileReader read plain file
type PlainFileReader struct {
	c *WorkerConf
	t *tail.Tail

	log logger
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
func (p *PlainFileReader) Create(conf *WorkerConf, l logger) (pfr Reader, err error) {
	l.Infof("Monitoring '%s' by Plain File Reader", conf.File)
	pfr = &PlainFileReader{
		c:   conf,
		log: l,
	}
	return
}

// Start worker (reading file)
func (p *PlainFileReader) Start() (err error) {
	if p.t != nil {
		return errors.Errorf("already reading")
	}

	p.t, err = tail.TailFile(p.c.File,
		tail.Config{
			Follow:   true,
			ReOpen:   true,
			Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
			Logger:   tail.DiscardingLogger,
			Poll:     p.c.Options["poll"] == "yes",
			Pipe:     p.c.Options["pipe"] == "yes",
		},
	)

	return errors.Wrap(err, "open file error")
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
