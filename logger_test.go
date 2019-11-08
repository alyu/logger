package logger

import (
	"fmt"
	"log"
	"log/syslog"
	"testing"
	"time"
)

var lg *Logger4go

func TestDefaultLogger(t *testing.T) {
	Logger.Emergf("The severity is %v", "emerge")
	Logger.Alertf("The severity is %v", "alert")
	Logger.Critf("The severity is %v", "crit")
	Logger.Errf("The severity is %v", "err")
	Logger.Warningf("The severity is %v", "warning")
	Logger.Noticef("The severity is %v", "notice")
	Logger.Infof("The severity is %v", "info")
	Logger.Debugf("%+v", Logger)
}

func TestInitNewLogger(t *testing.T) {
	lg = GetWithFlags("testing", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	lg.Info("This log event should not be written out")
}

func TestStdout(t *testing.T) {
	lg.AddStdoutHandler()
	lg.Info("This log event should be written to stdout")
}

func TestStderr(t *testing.T) {
	h, _ := lg.AddStderrHandler()
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

func TestStructureLog(t * testing.T) {
	st := struct {
		A string
		B int
		C uint16
	}{
		"A string",
		10,
		100,
	}

	lg.Infof("Test structure: %+v", st)
}

func TestRegularLog(t *testing.T) {
	lg.Println("This is the regular log line")
}

func TestFilter(t *testing.T) {
	lg.Debug("Setting filter to Info|Crit")
	lg.SetFilter(InfoSeverity | CritSeverity)
	lg.Emerg("This should not be written out")
	lg.Alert("This should not be written out")

	startThreads()
	time.Sleep(10e3* time.Millisecond)
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
