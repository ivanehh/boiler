package logging

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-sql/civil"
)

// TODO: A log reporter should be able to report from multiple workplaces for gradeab
type LogReport map[civil.Date]map[string][]LogRecord

type logReporter struct {
	folder   string
	stackRef *[]LogRecord
}

func newLogReporter(name string, lc LogConfiguration, recordStack *[]LogRecord) logReporter {
	return logReporter{
		folder:   filepath.Join(lc.Dir(), name),
		stackRef: recordStack,
	}
}

// TODO: Filter according to severity
func (wpl *logReporter) filterLogs(days int, sev string) []string {
	filteredLogs := make([]string, 0)
	// Filter the logs per input parameters
	for d := 0; d <= days; d++ {
		t := civil.DateOf(time.Now().AddDate(0, 0, -d)).String()
		logs, err := fs.Glob(os.DirFS(wpl.folder), fmt.Sprintf("%v*", t))
		if err != nil {
			return nil
		}
		filteredLogs = append(filteredLogs, logs...)
	}
	return filteredLogs
}

// GetLogs() returns the selected logs nested in a json strcture according to their date and type
func (wpl *logReporter) getLogs(days int, wp string, sev string) LogReport {
	lr := make(LogReport)
	filteredLogs := wpl.filterLogs(days, sev)
	// Extract the data from the filtered logs
	for _, log := range filteredLogs {
		content, err := os.ReadFile(filepath.Join(wpl.folder, log))
		if err != nil {
			continue
		}
		tmpWpL := make([]LogRecord, 1)
		err = json.Unmarshal(content, &tmpWpL)
		if err != nil {
			continue
		}
		t, err := time.Parse(time.DateOnly, strings.Split(log, "_")[0])
		if err != nil {
			continue
		}
		cDate := civil.DateOf(t)
		if _, ok := lr[cDate]; !ok {
			lr[cDate] = make(map[string][]LogRecord)
		}
		lr[civil.DateOf(t)][wp] = append(lr[civil.DateOf(t)][wp], tmpWpL...)

	}
	if len(*wpl.stackRef) > 0 {
		if _, ok := lr[civil.DateOf(time.Now())][wp]; ok {
			lr[civil.DateOf(time.Now())][wp] = append(lr[civil.DateOf(time.Now())][wp], *wpl.stackRef...)
		}
	}
	return lr
}
