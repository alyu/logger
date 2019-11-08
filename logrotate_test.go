// +build logrotate

package logger

import (
	"log"
	"testing"
	"time"

	"github.com/alyu/logger/handler"
)

func TestFileHandlerWithLogration(t *testing.T) {
	lg := GetWithFlags("testing", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	// add a file handler which rotates 5 files with a maximum size of 5KB starting with sequence no 1,
	// daily midnight rotation disabled and with compress logs enabled
	_, err := lg.AddFileHandler("/tmp/logger2.log", uint(5*handler.KB), 5, true, false)
	if err != nil {
		t.Logf("Unable to add file handler: %v", err)
	}
}

func TestLogRotate(t *testing.T) {
	lg := Get("testing")
	lg.AddStdoutHandler()

	lg.Info("Setting filter to include all levels")
	lg.SetFilter(AllSeverity)

	for i := 0; i < 10e3; i++ {
		lg.Debug("A debug message")
		lg.Info("An info message")
		lg.Notice("A notice message")
		lg.Warn("A warning message")
		lg.Err("An error messagessage")
		lg.Crit("A critical message")
		lg.Alert("An alert message")
		lg.Emerg("An emergency message")

		lg.Debugf("A %s debug message", "formattated")
		lg.Infof("An %s info message", "formatted")
		lg.Noticef("A %s notice message", "formatted")
		time.Sleep(5e3 * time.Millisecond)
	}
}
