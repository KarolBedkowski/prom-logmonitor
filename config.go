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
	// LogFile configure one worker
	LogFile struct {
		// File to read
		File string
		// Metric name to export
		Metric string
		// Filters (regexp)
		Filter []string
		// Enabled allow disable some workers
		Enabled bool
	}

	// Configuration keep application configuration
	Configuration struct {
		// Files is list of workers
		Files []*LogFile
	}
)

func (c *Configuration) validate() error {

	if len(c.Files) == 0 {
		return fmt.Errorf("no files to monitor")
	}
	for _, f := range c.Files {
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
