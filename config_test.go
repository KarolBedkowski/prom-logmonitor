//
// config_test.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"testing"
)

func TestValidateMetrics(t *testing.T) {
	wc := &WorkerConf{}
	validNames := []string{
		"aaaALKLDDKL_addklak123",
		"dldsjl_231_1231",
		"l",
		"lslslsld_",
	}

	for i, v := range validNames {
		metric := &Metric{Name: v}
		if err := metric.validate(wc, i); err != nil {
			t.Errorf("error for valid name (%v): %s", v, err)
		}
	}

	invalidNames := []string{
		"_allskslksl",
		"1212_ds",
		"lkl_.daklkla#",
		"sljdsl  ",
	}
	for i, v := range invalidNames {
		metric := &Metric{Name: v}
		if err := metric.validate(wc, i); err == nil {
			t.Errorf("missing error for invalid name (%v)", v)
		}
	}
}

func TestValidateMetricLabels(t *testing.T) {
	wc := &WorkerConf{}
	definedMetris := make(map[string][]string)

	m1 := &Metric{
		Name:   "m1",
		Labels: make(map[string]string),
	}
	m1.Labels["label1"] = ""
	m1.Labels["label2"] = ""
	m1.Labels["label3"] = ""

	if err := m1.validateLabels(wc, 0, definedMetris); err != nil {
		t.Errorf("error in valid metric (%+v): %s", m1, err)
	}

	m2 := &Metric{
		Name:   "m1",
		Labels: make(map[string]string),
	}
	m2.Labels["label3"] = ""
	m2.Labels["label1"] = ""
	m2.Labels["label2"] = ""

	if err := m2.validateLabels(wc, 0, definedMetris); err != nil {
		t.Errorf("error in valid metric (%+v): %s", m2, err)
	}

	m3 := &Metric{
		Name:   "m1",
		Labels: make(map[string]string),
	}
	m3.Labels["label1"] = ""
	m3.Labels["label2"] = ""

	if err := m3.validateLabels(wc, 0, definedMetris); err == nil {
		t.Errorf("no error in invalid metric (%+v)", m3)
	}

	m4 := &Metric{
		Name:   "m1",
		Labels: make(map[string]string),
	}
	m4.Labels["label1"] = ""
	m4.Labels["label4"] = ""
	m4.Labels["label2"] = ""

	if err := m4.validateLabels(wc, 0, definedMetris); err == nil {
		t.Errorf("no error in invalid metric (%+v)", m4)
	}

	m5 := &Metric{
		Name:   "m5",
		Labels: make(map[string]string),
	}
	m5.Labels["label3"] = ""

	if err := m5.validateLabels(wc, 0, definedMetris); err != nil {
		t.Errorf("error in valid metric (%+v): %s", m5, err)
	}

}
