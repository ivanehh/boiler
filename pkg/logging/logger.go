package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	pkg "github.com/ivanehh/boiler/pkg"
)

const MaxStackSize int = 5

var (
	MaxFileSize int64 = 5096000
	logger      *DCSlogger
)

type LogConfiguration interface {
	MinLevel() slog.Level
	Dir() string
	MaxFileSize() int
}

type structuredError interface {
	error
	pkg.Mapable
}

type DCSlogger struct {
	sync.Mutex
	logReporter
	slogger *slog.Logger
	name    string
}

type attrSetter func() slog.Attr

// TODO: For gradeab we should be able to spawn multiple loggers
func NewDCSlogger(name string, lc LogConfiguration, slogAttrs ...attrSetter) *DCSlogger {
	rc := new([]LogRecord)
	writer := io.MultiWriter(os.Stdout, newLogFileWriter(filepath.Join(lc.Dir(), name), rc))
	handle := slog.NewJSONHandler(writer, &slog.HandlerOptions{AddSource: false, Level: lc.MinLevel()})
	logger = &DCSlogger{
		Mutex:       sync.Mutex{},
		name:        name,
		slogger:     slog.New(handle),
		logReporter: newLogReporter(name, lc, rc),
	}
	for _, sas := range slogAttrs {
		logger.slogger = logger.slogger.With(sas())
	}
	return &DCSlogger{
		Mutex:       sync.Mutex{},
		name:        name,
		slogger:     slog.New(handle),
		logReporter: newLogReporter(name, lc, rc),
	}
}

func Provide() *DCSlogger {
	if logger == nil {
		panic("logger provision requested but logger not instantiated")
	}
	return logger
}

func (l *DCSlogger) Name() string {
	return l.name
}

func SetWorkplace(plant string, wp string) attrSetter {
	return func() slog.Attr {
		return slog.Group("workplace", "plant", plant, "name", wp)
	}
}

func (l *DCSlogger) Debug(msg string, details ...any) {
	l.slogger.Debug(msg, "info", details)
}

func (l *DCSlogger) Info(msg string, details ...any) {
	l.slogger.Info(msg, "info", details)
}

func (l *DCSlogger) Warn(msg string, err ...any) {
	l.slogger.Warn(msg, "info", err)
}

func (l *DCSlogger) Error(msg string, err ...any) {
	l.slogger.Error(msg, "info", err)
}

func (l *DCSlogger) GetLogs(days int, sev string) LogReport {
	l.Lock()
	defer l.Unlock()
	return l.getLogs(days, l.name, sev)
}
