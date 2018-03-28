// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

// Package logger provides Logger4go which is a simple wrapper around go's log.Logger.
//
// It provides three log handlers ConsoleHandler|FileHandler|SyslogHandler,
// wrapper methods named after syslog's severity levels and embedds log.Logger to provide
// seemless access to its methods as well if needed.
//
// Supports:
//
//  - Write to multiple handlers, e.g., log to console, file and syslog at the same time.
//  - Use more than one logger instance. Each with its own set of handlers.
//  - Log file rotation (size of daily) and compression.
//  - Filter out severity levels.
//
// Example output:
// 	main 2013/06/21 08:21:44.680513 -info- init called
// 	100m sprint 2013/06/21 08:21:44.680712 -info- Started 100m sprint: Should take 10 seconds.
// 	Long jump 2013/06/21 08:21:44.680727 -info- Started Long jump: Should take 6 seconds.
// 	High jump 2013/06/21 08:21:44.680748 -info- Started High jump: Should take 3 seconds.
// 	High jump 2013/06/21 08:21:47.683402 -info- Finished High jump
// 	Long jump 2013/06/21 08:21:50.683182 -info- Finished Long jump
// 	100m sprint 2013/06/21 08:21:54.683871 -info- Finished 100m sprint
// 	main 2013/06/21 08:22:14 -debug- A debug message
// 	main 2013/06/21 08:22:14 -info- An info message
// 	main 2013/06/21 08:22:14 -notice- A notice message
// 	main 2013/06/21 08:22:14 -warn- A warning message
// 	main 2013/06/21 08:22:14 -err- An error message
// 	main 2013/06/21 08:22:14 -crit- A critical message
// 	main 2013/06/21 08:22:14 -Alert- An alert message
// 	main 2013/06/21 08:22:14 -Emerge- An Emergeency message
//
// TODO:
// 	- Custom header format
//	- Read settings from config file
package logger

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"sync"
	"strconv"
)

// Logger4go embedds go's log.Logger as an anonymous field and
// so those methods are also exposed/accessable via Logger4go.
type Logger4go struct {
	name     string
	handlers []Handler
	filter   SeverityFilter
	mutex    sync.Mutex
	*log.Logger
}

// Logger provides a default Logger4go instance that output to the console
var Logger *Logger4go

func init() {
	Logger = Get("")
	Logger.AddConsoleHandler()

	// Set Std logger
	Get("main").AddConsoleHandler()

	// Set Err logger
	Get("err").AddErrConsoleHandler()
}

// Def returns the default logger instance with a console handler with no prefix.
func Def() *Logger4go {
	return Logger
}

// Stdout returns a standard logger instance with a stdout console handler using prefix 'main'.
func Stdout() *Logger4go {
	return Get("main")
}

// Stderr returns the standard logger instance with a stderr console handler using prefix 'err'
func Stderr() *Logger4go {
	return Get("err")
}

// SeverityFilter represents a severity level to filter
// go:generate stringer -type=SeverityFilter
type SeverityFilter int

// severity levels
const (
	Emerg SeverityFilter = 1 << iota
	Alert
	Crit
	Err
	Warning
	Notice
	Info
	Debug
	All = Emerg | Alert | Crit | Err | Warning | Notice | Info | Debug
)

// severity keywords
var (
	EmergString   = "-emerg-"
	AlertString   = "-alert-"
	CritString    = "-crit-"
	ErrString     = "-err-"
	WarningString = "-warning-"
	NoticeString  = "-notice-"
	InfoString    = "-info-"
	DebugString   = "-debug-"
	AllString     = ""
)

func (s SeverityFilter) String() string {
	switch {
	case s == 1:
		return EmergString
	case s == 2:
		return AlertString
	case s == 4:
		return CritString
	case s == 8:
		return ErrString
	case s == 16:
		return WarningString
	case s == 32:
		return NoticeString
	case s == 64:
		return InfoString
	case s == 128:
		return DebugString
	case s == 255:
		return AllString
	default:
		return "SeverityFilter(" + strconv.FormatInt(int64(s), 10) + ")"
	}
}

// from go's log package
const (
	// Bits or'ed together to control what's printed. There is no control over the
	// order they appear (the order listed here) or the format they present (as
	// described in the comments).  A colon appears after these items:
	//	2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
	Ldate         = 1 << iota     // the date: 2009/01/23
	Ltime                         // the time: 01:23:23
	Lmicroseconds                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                     // full file name and line number: /a/b/c/d.go:23
	Lshortfile                    // final file name element and line number: d.go:23. overrides Llongfile
	LstdFlags     = Ldate | Ltime // initial values for the standard logger
)


// Get returns a logger with the specified name and default log header flags.
// If it does not exist a new instance will be created.
func Get(name string) *Logger4go {
	return GetWithFlags(name, log.LstdFlags)
}

// GetWithFlags returns a logger with the specified name and log header flags.
// If it does exist a new instance will be created.
func GetWithFlags(name string, flags int) *Logger4go {
	mu.RLock()
	lg, ok := loggers4go[name]
	mu.RUnlock()
	if !ok {
		prefix := name + " "
		if name == "" {
			prefix = ""
		}
		// create with a noop writer/handler
		lg = newLogger(&NoopHandler{}, name, prefix, flags)
		lg.filter = All
		mu.Lock()
		defer mu.Unlock()
		loggers4go[name] = lg
	}
	return lg
}

// AddConsoleHandler adds a logger that writes to stdout/console
func (l *Logger4go) AddConsoleHandler() (ch *ConsoleHandler, err error) {
	ch = &ConsoleHandler{}
	registerHandler(l, ch)

	return ch, nil
}

// AddErrConsoleHandler adds a logger that writes to stderr/console
func (l *Logger4go) AddErrConsoleHandler() (ch *ErrConsoleHandler, err error) {
	ch = &ErrConsoleHandler{}
	registerHandler(l, ch)

	return ch, nil
}

// AddStdFileHandler adds a file handler which rotates the log file 5 times with a maximum size of 1MB each
// starting with sequence no 1 and with compression and daily rotation disabled
func (l *Logger4go) AddStdFileHandler(filePath string) (fh *FileHandler, err error) {

	fh, err = newStdFileHandler(filePath)
	if err != nil {
		return nil, err
	}
	registerHandler(l, fh)
	return fh, nil
}

// AddFileHandler adds a file handler with a specified max filesize, max number of rotations, file compression and daily rotation
func (l *Logger4go) AddFileHandler(filePath string, maxFileSize uint, maxRotation byte, isCompressFile, isDailyRotation bool) (fh *FileHandler, err error) {

	fh, err = newFileHandler(filePath, maxFileSize, maxRotation, 1, isCompressFile, isDailyRotation)
	if err != nil {
		return nil, err
	}
	registerHandler(l, fh)
	return fh, nil
}

// AddSyslogHandler adds a syslog handler with the specified network procotol tcp|udp, a syslog daemon ip address,
// a log/syslog priority flag (syslog severity + facility, see syslog godoc) and a tag/prefix.
// The syslog daemon on localhost will be used if protocol and ipaddr is "".
//
// AddSyslogHandler returns a SyslogHandler which can be used to directly access the SyslogHandler.out (syslog.Writer) instance
// which can be used to write messages with a specific syslog severity and bypassing what the logger instance is set to use.
// No default header is written when going via the syslog.Writer instance.
func (l *Logger4go) AddSyslogHandler(protocol, ipaddr string, priority syslog.Priority, tag string) (sh *SyslogHandler, err error) {
	sh, err = newSyslogHandler(protocol, ipaddr, priority, tag)
	if err != nil {
		return nil, err
	}
	registerHandler(l, sh)

	return sh, err
}

// AddHandler adds a custom handler which conforms to the Handler interface.
func (l *Logger4go) AddHandler(handler Handler) {
	registerHandler(l, handler)
}

// Handlers returns a list of registered handlers
func (l *Logger4go) Handlers() []Handler {
	return l.handlers
}

// Emergf log
func (l *Logger4go) Emergf(format string, v ...interface{}) {
	l.doPrintf(Emerg, format, v...)
}

// Emerg log
func (l *Logger4go) Emerg(v ...interface{}) {
	l.doPrintf(Emerg, "%s", v...)
}

// Alertf log
func (l *Logger4go) Alertf(format string, v ...interface{}) {
	l.doPrintf(Alert, format, v...)
}

// Alert log
func (l *Logger4go) Alert(v ...interface{}) {
	l.doPrintf(Alert, "%s", v...)
}

// Critf log
func (l *Logger4go) Critf(format string, v ...interface{}) {
	l.doPrintf(Crit, format, v...)
}

// Crit log
func (l *Logger4go) Crit(v ...interface{}) {
	l.doPrintf(Crit, "%s", v...)
}

// Errf log
func (l *Logger4go) Errf(format string, v ...interface{}) {
	l.doPrintf(Err, format, v...)
}

// Err log
func (l *Logger4go) Err(v ...interface{}) {
	l.doPrintf(Err, "%s", v...)
}

// Warningf log
func (l *Logger4go) Warningf(format string, v ...interface{}) {
	l.doPrintf(Warning, format, v...)
}

// Warning log
func (l *Logger4go) Warning(v ...interface{}) {
	l.doPrintf(Warning, "%s", v...)
}

// Warnf log
func (l *Logger4go) Warnf(format string, v ...interface{}) {
	l.doPrintf(Warning, format, v...)
}

// Warn log
func (l *Logger4go) Warn(v ...interface{}) {
	l.doPrintf(Warning, "%s", v...)
}

// Noticef log
func (l *Logger4go) Noticef(format string, v ...interface{}) {
	l.doPrintf(Notice, format, v...)
}

// Notice log
func (l *Logger4go) Notice(v ...interface{}) {
	l.doPrintf(Notice, "%s", v...)
}

// Infof log
func (l *Logger4go) Infof(format string, v ...interface{}) {
	l.doPrintf(Info, format, v...)
}

// Info log
func (l *Logger4go) Info(v ...interface{}) {
	l.doPrintf(Info, "%s", v...)
}

// Debugf log
func (l *Logger4go) Debugf(format string, v ...interface{}) {
	l.doPrintf(Debug, format, v...)
}

// Debug log
func (l *Logger4go) Debug(v ...interface{}) {
	l.doPrintf(Debug, "%s", v...)
}

// IsFilterSet returns true if the severity filter is set
func (l *Logger4go) IsFilterSet(f SeverityFilter) bool {
	return f&l.filter == f
}

// SetFilter sets a severity filter
func (l *Logger4go) SetFilter(f SeverityFilter) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.filter = f
}

// Flags returns the current set of logger flags
func (l *Logger4go) Flags() int {
	return l.Logger.Flags()
}

// SetFlags sets a logger flag
func (l *Logger4go) SetFlags(flag int) {
	l.Logger.SetFlags(flag)
}

// Prefix returns the logger prefix
func (l *Logger4go) Prefix() string {
	return l.Logger.Prefix()
}

// SetPrefix sets the logger prefix
func (l *Logger4go) SetPrefix(prefix string) {
	l.Logger.SetPrefix(prefix)
}

// SetOutput sets a writer
func (l *Logger4go) SetOutput(out io.Writer) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.Logger = log.New(out, l.Logger.Prefix(), l.Logger.Flags())
}

//
// Private
//
var mu = &sync.RWMutex{}
var loggers4go = make(map[string]*Logger4go)

func (l *Logger4go) doPrintf(f SeverityFilter, format string, v ...interface{}) {
	if l.IsFilterSet(f) {
		l.Printf(fmt.Sprintf("%s ", f) + format, v...)
	}
}

func newLogger(out io.Writer, name string, prefix string, flags int) *Logger4go {
	return &Logger4go{name: name, Logger: log.New(out, prefix, flags)}
}

func registerHandler(l *Logger4go, handler Handler) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.handlers = append(l.handlers, handler)
	out := make([]io.Writer, 0)
	for _, h := range l.handlers {
		out = append(out, h)
	}
	l.Logger = log.New(io.MultiWriter(out...), l.Prefix(), l.Flags())
}
