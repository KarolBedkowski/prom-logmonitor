//
// config.go
// Copyright (C) Karol Będkowski, 2017

package main

import (
	"fmt"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
)

type (
	// Filter define patterns for include/exclude
	Filter struct {
		// Include is list patterns to find in files
		Include []string
		// Exclude is list patterns that line must not contain to accept
		Exclude []string

		XUnknown map[string]interface{} `yaml:",inline"`
	}

	// WorkerConf configure one worker
	WorkerConf struct {
		// File to read
		File string
		// Metric name to export
		Metric string
		// Filters (regexp)
		Patterns []*Filter
		// Enabled allow disable some workers
		Enabled bool

		XUnknown map[string]interface{} `yaml:",inline"`
	}

	// Configuration keep application configuration
	Configuration struct {
		// Workers is list of workers
		Workers []*WorkerConf

		XUnknown map[string]interface{} `yaml:",inline"`
	}
)

func checkUnknown(m map[string]interface{}) (invalid string) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return strings.Join(keys, ", ")
}

func (c *Configuration) validate() error {
	if len(c.Workers) == 0 {
		return fmt.Errorf("no files to monitor")
	}

	for _, f := range c.Workers {
		if f.File == "" {
			return fmt.Errorf("missing 'file' in %+v", f)
		}
		if f.Metric == "" {
			return fmt.Errorf("missing 'metric' in %+v", f)
		}
	}

	// check for unknown fields
	if msg := checkUnknown(c.XUnknown); msg != "" {
		log.Warnf("unknown fields in configuraton: %s", msg)
	}

	for i, f := range c.Workers {
		if msg := checkUnknown(f.XUnknown); msg != "" {
			log.Warnf("unknown fields in worker %d [%s]: %s", i+1, f.Metric, msg)
		}

		for j, p := range f.Patterns {
			if msg := checkUnknown(p.XUnknown); msg != "" {
				log.Warnf("unknown fields in worker %d [%s] patterns %d: %s", i+1, f.Metric, j+1, msg)
			}
		}
	}

	return nil
}

// LoadConfiguration from `filename`
func LoadConfiguration(filename string) (*Configuration, error) {
	c := &Configuration{}
	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}

	if err = c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}
