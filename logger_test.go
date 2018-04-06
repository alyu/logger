package logger

import (
	"fmt"
	"log/syslog"
	"testing"
	"time"
)

var lg *Logger4go

func TestDefaultLogger(t *testing.T) {
	Logger.Infof("%v is the default logger", "Logger")
	Logger.Debugf("%+v", Logger)
}

func TestInitNewLogger(t *testing.T) {
	lg = GetWithFlags("testing", Ldate|Ltime|Lmicroseconds)
	lg.Info("This log event should not be written out")
}

func TestStdout(t *testing.T) {
	lg.AddConsoleHandler()
	lg.Info("This log event should be written to stdout")
}

func TestStderr(t *testing.T) {
	h, _ := lg.AddErrConsoleHandler()
	lg.Info("This log event should be written to stderr")
	lg.RemoveHandler(h)
}

func TestFileHandler(t *testing.T) {
	_, err := lg.AddStdFileHandler("/tmp/logger.log")
	if err != nil {
		t.Error("Unable to open /tmp/logger.log")
	}
	lg.Alert("This log event should be on the console/stdout and log file")
}

func TestFileHandlerWithLogration(t *testing.T) {
	// add a file handler which rotates 5 files with a maximum size of 5KB starting with sequence no 1,
	// daily midnight rotation disabled and with compress logs enabled
	_, err := lg.AddFileHandler("/tmp/logger2.log", uint(5*KB), 5, true, false)
	if err != nil {
		t.Logf("Unable to add file handler: %v", err)
	}
}

func TestFileHandlerWithErr(t *testing.T) {
	_, err := lg.AddStdFileHandler("/abc/logger.log")
	if err != nil {
		t.Logf("Unable to add file handler: %v", err)
	}
}

func TestSyslogHandler(t *testing.T) {
	sh, err := lg.AddSyslogHandler("", "", syslog.LOG_INFO|syslog.LOG_LOCAL0, "logger")
	if err != nil {
		t.Fatal("Unable to connect to syslog daemon")
	}
	lg.Info("This should be on console, log file and syslog")
	err = sh.Out.Err("This syslog record should be recored with severity Error")
	if err != nil {
		t.Error("Unable to log to syslog")
	}
}

func TestFilter(t *testing.T) {
	lg.Debug("Setting filter to Info|Crit")
	lg.SetFilter(InfoSeverity | CritSeverity)
	lg.Emerg("This should not be written out")

	startThreads()
}

func TestLogRotate(t *testing.T) {
	lg.Info("Setting filter to All")
	lg.SetFilter(All)

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

func simulateEvent(name string, timeInSecs int64) {
	// sleep for a while to simulate time consumed by event
	lg.Info(name + ":Started   " + name + ": Should take" + fmt.Sprintf(" %d ", timeInSecs) + "seconds.")
	time.Sleep(time.Duration(timeInSecs * 1e9))
	lg.Crit(name + ":Finished " + name)
}

func startThreads() {
	go simulateEvent("100m sprint", 10) //start 100m sprint, it should take         10 seconds
	go simulateEvent("Long jump", 6)    //start long jump, it       should take 6 seconds
	go simulateEvent("High jump", 3)    //start Highh jump, it should take 3 seconds
}
