package dcsLogger_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"dcs-lib/pkg/logging"
)

var lconfig = logConfig{
	level:       slog.LevelDebug,
	dir:         "/home/terzivan/tmp/",
	maxFileSize: 5,
}

type logConfig struct {
	level       slog.Level
	dir         string
	maxFileSize int
}

func (lc logConfig) MinLevel() slog.Level {
	return lc.level
}

func (lc logConfig) Dir() string {
	return lc.dir
}

func (lc logConfig) MaxFileSize() int {
	return lc.maxFileSize
}

func TestNewDCSlogger(t *testing.T) {
	lconfig := logConfig{
		level:       slog.LevelDebug,
		dir:         "/home/terzivan/tmp/",
		maxFileSize: 5,
	}
	logging.NewDCSlogger("test", lconfig)
	l := logging.Provide()
	if l.Name() != "test" {
		t.Fatal("logger name mismatch")
	}
}

// TODO: Set automated pass/fail criteria
func TestAttrSetter(t *testing.T) {
	logging.NewDCSlogger("test", lconfig, logging.SetWorkplace("3500", "M1"))
	l := logging.Provide()
	l.Info("this is a test message")
}

func TestLogFileWrite(t *testing.T) {
	logging.NewDCSlogger("test", lconfig)
	l := logging.Provide()
	for i := 0; i < logging.MaxStackSize+1; i++ {
		l.Info("test_message", fmt.Sprintf("message_number: %v", i))
	}
	entries, err := os.ReadDir(filepath.Join(lconfig.dir, "test"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal(fmt.Errorf("no appropriate entries found in %v", filepath.Join(lconfig.dir, "test")))
	}
}

func TestLogReporter(t *testing.T) {
	logging.NewDCSlogger("test", lconfig)
	l := logging.Provide()
	lr := l.GetLogs(0, "test")
	if len(lr) == 0 {
		t.Fatalf("acquired log report is of length 0;report:%+v", lr)
	}
	fmt.Printf("%+v", lr)
}
