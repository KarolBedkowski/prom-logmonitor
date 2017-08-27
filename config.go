//
// config.go
// Copyright (C) Karol Będkowski, 2017

package main

import (
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"regexp"
	"sort"
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

	// Metric define one metric and group of patterns that launches this metric
	Metric struct {
		// Name of metri
		Name string
		// Filters (regexp)
		Patterns []*Filter
		// Disabled allow disable some workers
		Disabled bool

		Labels map[string]string

		// ValuePattern define re pattern extracted from line and exposed as metrics.
		ValuePattern string `yaml:"value_pattern"`

		StaticLabels []string `yaml:"-"`
	}

	// WorkerConf configure one worker
	WorkerConf struct {
		// File to read
		File string
		// Metric name to export
		Metrics []*Metric
		// Disabled allow disable some workers
		Disabled bool
		// Stamp filename
		StampFile string `yaml:"stamp_file"`
		// options for worker
		Options map[string]string `yaml:"options"`

		XUnknown map[string]interface{} `yaml:",inline"`
	}

	// Configuration keep application configuration
	Configuration struct {
		// Workers is list of workers
		Workers []*WorkerConf

		XUnknown map[string]interface{} `yaml:",inline"`
	}
)

var isValidName = regexp.MustCompile(`^[a-zA-Z][_a-zA-Z0-9]*$`).MatchString

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
		return errors.Errorf("no files to monitor")
	}

	usedFiles := make(map[string]int)

	for i, f := range c.Workers {
		if f.Disabled {
			continue
		}
		if f.File == "" {
			return errors.Errorf("missing 'file' in %+v", f)
		}
		if ruleNum, exists := usedFiles[f.File]; exists {
			return errors.Errorf("file '%s' already defined in rule %d", f.File, ruleNum)
		}
		usedFiles[f.File] = i + 1
	}

	// check for unknown fields
	if msg := checkUnknown(c.XUnknown); msg != "" {
		log.Warnf("unknown fields in configuraton: %s", msg)
	}

	definedLabels := make(map[string][]string)

	for i, f := range c.Workers {
		if f.Disabled {
			continue
		}

		if msg := checkUnknown(f.XUnknown); msg != "" {
			log.Warnf("unknown fields in worker %d [%s]: %s", i+1, f.Metrics, msg)
		}

		for i, m := range f.Metrics {
			if m.Disabled {
				continue
			}

			if err := m.validate(f, i); err != nil {
				return err
			}

			if err := m.validateLabels(f, i, definedLabels); err != nil {
				return err
			}
		}
	}

	return nil
}

// prepareLabels make list of static labels
func (c *Configuration) prepareLabels() {
	for _, f := range c.Workers {
		if f.Disabled {
			continue
		}

		for _, m := range f.Metrics {
			var labels []string
			for k := range m.Labels {
				labels = append(labels, k)
			}
			sort.Strings(labels)
			m.StaticLabels = []string{f.File}
			for _, k := range labels {
				m.StaticLabels = append(m.StaticLabels, m.Labels[k])
			}
		}
	}
}

// LoadConfiguration from `filename`
func LoadConfiguration(filename string) (*Configuration, error) {
	c := &Configuration{}
	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, errors.Wrap(err, "read configuration file error")
	}

	if err = yaml.Unmarshal(b, c); err != nil {
		return nil, errors.Wrap(err, "configuration unmarshall error")
	}

	if err = c.validate(); err != nil {
		return nil, errors.Wrap(err, "configuration validate error")
	}

	c.prepareLabels()

	return c, nil
}

func (m *Metric) validate(f *WorkerConf, i int) error {
	if m.Name == "" {
		return errors.Errorf("missing metric name in %+v", m)
	}

	if !isValidName(m.Name) {
		return errors.Errorf("invalid metric name: '%s'", m.Name)
	}

	for j, p := range m.Patterns {
		if msg := checkUnknown(p.XUnknown); msg != "" {
			log.Warnf("unknown fields in worker %d [%s] patterns %d: %s", i+1, f.Metrics, j+1, msg)
		}
	}

	return nil
}

func (m *Metric) validateLabels(f *WorkerConf, i int, definedLabels map[string][]string) error {
	var mlabels []string
	for label := range m.Labels {
		if !isValidName(label) {
			return errors.Errorf("invalid label name '%s' in '%s'", label, m.Name)
		}
		mlabels = append(mlabels, label)
	}

	sort.Strings(mlabels)

	dlabels, ok := definedLabels[m.Name]
	if !ok {
		definedLabels[m.Name] = mlabels
		return nil
	}

	if len(dlabels) != len(mlabels) {
		return errors.Errorf("invalid number of labels (%v) in '%s', defined: %v",
			mlabels, m.Name, dlabels)
	}

	for i, l := range dlabels {
		if l != mlabels[i] {
			return errors.Errorf("invalid labels on pos %d in '%s' (%v), defined: %v",
				i+1, m.Name, mlabels, dlabels)
		}
	}

	return nil
}
