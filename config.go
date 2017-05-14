//
// config.go
// Copyright (C) Karol Będkowski, 2017

package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type (
	// Filter define patterns for include/exclude
	Filter struct {
		// Include is list patterns to find in files
		Include []string
		// Exclude is list patterns that line must not contain to accept
		Exclude []string
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
	}

	// Configuration keep application configuration
	Configuration struct {
		// Workers is list of workers
		Workers []*WorkerConf
	}
)

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
