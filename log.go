//
// log.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the GPLv3 license.
//
// based on: github.com/prometheus/common/log

package main

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"runtime"
	"sort"
	"strings"
)

type logger struct {
	entry *logrus.Entry
}

// sourced adds a source field to the logger that contains
// the file name and line where the logging happened.
func (l *logger) sourced() *logrus.Entry {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<???>"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		file = file[slash+1:]
	}
	return l.entry.WithField("source", fmt.Sprintf("%s:%d", file, line))
}

var baseLogger = logrus.New()
var log = logger{logrus.NewEntry(baseLogger)}

// InitializeLogger set log level and optional log filename
func InitializeLogger(level, filename string) {

	lev, err := logrus.ParseLevel(level)
	if err != nil {
		panic("invalid log level: " + err.Error())
	}

	baseLogger.Level = lev

	if filename == "" {
		if o, ok := baseLogger.Out.(*os.File); ok && terminal.IsTerminal(int(o.Fd())) {
			baseLogger.Formatter = &logrus.TextFormatter{}
		} else {
			baseLogger.Formatter = &simpleFormatter{}
		}
	} else {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
		if err != nil {
			panic("failed to use '" + filename + "' for logging: " + err.Error())
		}
		baseLogger.Out = file
		baseLogger.Formatter = &logrus.TextFormatter{}
	}
}

// Debug logs a message at level Debug on the standard logger.
func (l *logger) Debug(args ...interface{}) {
	log.sourced().Debug(args...)
}

// Debugln logs a message at level Debug on the standard logger.
func (l *logger) Debugln(args ...interface{}) {
	log.sourced().Debugln(args...)
}

// Debugf logs a message at level Debug on the standard logger.
func (l *logger) Debugf(format string, args ...interface{}) {
	log.sourced().Debugf(format, args...)
}

// Info logs a message at level Info on the standard logger.
func (l *logger) Info(args ...interface{}) {
	log.sourced().Info(args...)
}

// Infoln logs a message at level Info on the standard logger.
func (l *logger) Infoln(args ...interface{}) {
	log.sourced().Infoln(args...)
}

// Infof logs a message at level Info on the standard logger.
func (l *logger) Infof(format string, args ...interface{}) {
	log.sourced().Infof(format, args...)
}

// Warn logs a message at level Warn on the standard logger.
func (l *logger) Warn(args ...interface{}) {
	log.sourced().Warn(args...)
}

// Warnln logs a message at level Warn on the standard logger.
func (l *logger) Warnln(args ...interface{}) {
	log.sourced().Warnln(args...)
}

// Warnf logs a message at level Warn on the standard logger.
func (l *logger) Warnf(format string, args ...interface{}) {
	log.sourced().Warnf(format, args...)
}

// Error logs a message at level Error on the standard logger.
func (l *logger) Error(args ...interface{}) {
	log.sourced().Error(args...)
}

// Errorln logs a message at level Error on the standard logger.
func (l *logger) Errorln(args ...interface{}) {
	log.sourced().Errorln(args...)
}

// Errorf logs a message at level Error on the standard logger.
func (l *logger) Errorf(format string, args ...interface{}) {
	log.sourced().Errorf(format, args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func (l *logger) Fatal(args ...interface{}) {
	log.sourced().Fatal(args...)
}

// Fatalln logs a message at level Fatal on the standard logger.
func (l *logger) Fatalln(args ...interface{}) {
	log.sourced().Fatalln(args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func (l *logger) Fatalf(format string, args ...interface{}) {
	log.sourced().Fatalf(format, args...)
}

func (l *logger) With(key string, value interface{}) logger {
	return logger{l.entry.WithField(key, value)}
}

type simpleFormatter struct {
}

func (s *simpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	b.WriteString(entry.Level.String())
	for i := 7 - b.Len(); i > 0; i-- {
		b.WriteByte(' ')
	}

	if entry.Message != "" {
		b.WriteByte(' ')
		b.WriteString(fmt.Sprintf("%q", entry.Message))
		for i := 80 - b.Len(); i > 0; i-- {
			b.WriteByte(' ')
		}
	}
	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprintf("%q", entry.Data[key]))
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
