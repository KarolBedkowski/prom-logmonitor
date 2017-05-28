//
// monitor.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"fmt"
	"github.com/hpcloud/tail"
	"github.com/prometheus/common/log"
	"os"
)

// PlainFileReader read plain file
type PlainFileReader struct {
	c *WorkerConf
	t *tail.Tail

	log log.Logger
}

// NewPlainFileReader create new reader for plain files
func NewPlainFileReader(conf *WorkerConf, l log.Logger) (p *PlainFileReader, err error) {
	p = &PlainFileReader{
		c:   conf,
		log: l,
	}
	return
}

// Start worker (reading file)
func (p *PlainFileReader) Start() error {
	if p.t != nil {
		return fmt.Errorf("already reading")
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
		return err
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
	l := <-p.t.Lines

	if l.Err != nil {
		return "", l.Err
	}

	return l.Text, nil
}
