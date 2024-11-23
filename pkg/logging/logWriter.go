package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-sql/civil"
)

// NOTE: Maybe this should be an interface with a JSONable requirement?
type LogRecord map[string]any

type logFileWriter struct {
	sync.Mutex
	stack  *[]LogRecord
	folder string
}

func newLogFileWriter(logDir string, recordStack *[]LogRecord) *logFileWriter {
	os.MkdirAll(logDir, 0o700)
	return &logFileWriter{
		Mutex:  sync.Mutex{},
		stack:  recordStack,
		folder: logDir,
	}
}

func (esw *logFileWriter) Write(b []byte) (int, error) {
	var err error
	record := &LogRecord{}
	esw.Lock()
	defer esw.Unlock()
	err = json.Unmarshal(b, record)
	if err != nil {
		return 0, err
	}
	*esw.stack = append(*esw.stack, *record)
	if len(*esw.stack) >= MaxStackSize {
		err = esw.flush()
		if err != nil {
			return 0, err
		}
	}
	return len(*esw.stack), nil
}

// flush writes appends esw.stack to an appropriate file; it then resets the stack to length 0
func (esw *logFileWriter) flush() error {
	prevLogs := make([]LogRecord, 0)
	lf, err := esw.findLatestFile()
	if err != nil {
		return err
	}
	defer func() error {
		err = lf.Close()
		return err
	}()
	content, err := io.ReadAll(lf)
	if err != nil {
		return err
	}
	if len(content) > 0 {
		err = json.Unmarshal(content, &prevLogs)
		if err != nil {
			return err
		}
	}
	prevLogs = append(prevLogs, *esw.stack...)
	stack, err := json.Marshal(prevLogs)
	if err != nil {
		return err
	}
	lf.Truncate(0)
	lf.Seek(0, 0)
	_, err = lf.Write(stack)
	if err != nil {
		return err
	}
	*esw.stack = (*esw.stack)[:0]
	return nil
}

func (esw *logFileWriter) findLatestFile() (*os.File, error) {
	var latestFile fs.FileInfo
	var lMod time.Time
	entries, err := fs.Glob(os.DirFS(esw.folder), fmt.Sprintf("%v*", civil.DateOf(time.Now())))
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		for _, entry := range entries {
			finfo, err := os.Stat(filepath.Join(esw.folder, entry))
			if err != nil {
				return nil, err
			}
			if finfo.ModTime().After(lMod) {
				latestFile = finfo
				lMod = finfo.ModTime()
			}
		}
		if latestFile.Size() >= MaxFileSize {
			return os.OpenFile(
				path.Join(
					esw.folder,
					civil.DateOf(time.Now()).String()+fmt.Sprintf("_%v", len(entries))+".log.json",
				),
				os.O_CREATE|os.O_RDWR,
				0o644,
			)
		}
	}
	return os.OpenFile(
		path.Join(
			esw.folder,
			civil.DateOf(time.Now()).String()+fmt.Sprintf("_%v", 0)+".log.json",
		),
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
}
