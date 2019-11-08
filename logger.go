// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

// Package logger provides Logger4go which is a simple wrapper around go's log.Logger.
//
// There are four log handlers StdoutHandler, StderrHandler, FileHandler and SyslogHandler.
// A handler writes a log event/line to a specified destination, for example a file or stdout.
// Logger4go exposes log methods named after syslog's severity levels and also embedds 
// log.Logger to provide seemless access to its methods as well if needed.
//
// Supports:
//
//  - Writing to multiple handlers, e.g., log to console, file and syslog at the same time.
//  - Using more than one logger instance. Each with its own set of handler.
//  - Rotate the log file based on size, per day or number of rotated files with compression.
//  - Enable only specific severity levels to be written out.
//
// Example output:
// 	main 2013/06/21 08:21:44.680513  info  init called
// 	100m sprint 2013/06/21 08:21:44.680712  info  Started 100m sprint: Should take 10 seconds.
// 	Long jump 2013/06/21 08:21:44.680727  info  Started Long jump: Should take 6 seconds.
// 	High jump 2013/06/21 08:21:44.680748  info  Started High jump: Should take 3 seconds.
// 	High jump 2013/06/21 08:21:47.683402  info  Finished High jump
// 	Long jump 2013/06/21 08:21:50.683182  info  Finished Long jump
// 	100m sprint 2013/06/21 08:21:54.683871  info  Finished 100m sprint
// 	main 2013/06/21 08:22:14  debug    A debug message
// 	main 2013/06/21 08:22:14  info     An info message
// 	main 2013/06/21 08:22:14  notice   A notice message
// 	main 2013/06/21 08:22:14  warning  A warning message
// 	main 2013/06/21 08:22:14  err      An error message
// 	main 2013/06/21 08:22:14  crit     A critical message
// 	main 2013/06/21 08:22:14  alert    An alert message
// 	main 2013/06/21 08:22:14  emerge   An Emergeency message
//
// TODO:
//  - Structured logging support. Output format should be JSON
//  - Read settings from config file or env vars
package logger

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"sync"
	"strconv"

	"github.com/alyu/logger/handler"
)

// Logger4go embedds go's log.Logger as an anonymous field and
// so those methods are also exposed/accessable via Logger4go.
type Logger4go struct {
	name     string
	handlers []handler.Handler
	filter   SeverityFilter
	mutex    sync.Mutex
	*log.Logger
}

// Logger provides a default Logger4go instance that outputs to the console
var Logger *Logger4go

func init() {
	// Use stdout handler as default
	Logger = Get("main")
	Logger.AddStdoutHandler()

	// Add default stderr handler
	Get("err").AddStderrHandler()
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
	EmergSeverity SeverityFilter = 1 << iota
	AlertSeverity
	CritSeverity
	ErrSeverity
	WarningSeverity
	NoticeSeverity
	InfoSeverity
	DebugSeverity
	AllSeverity = EmergSeverity | AlertSeverity | CritSeverity | ErrSeverity | WarningSeverity | NoticeSeverity | InfoSeverity | DebugSeverity
)

// severity keywords
const (
	EmergString   = " emerg   "
	AlertString   = " alert   "
	CritString    = " crit    "
	ErrString     = " err     "
	WarningString = " warning "
	NoticeString  = " notice  "
	InfoString    = " info    "
	DebugString   = " debug   "
	AllString     = ""
)

func (s SeverityFilter) String() string {
	switch {
	case s == EmergSeverity:
		return EmergString
	case s == AlertSeverity:
		return AlertString
	case s == CritSeverity:
		return CritString
	case s == ErrSeverity:
		return ErrString
	case s == WarningSeverity:
		return WarningString
	case s == NoticeSeverity:
		return NoticeString
	case s == InfoSeverity:
		return InfoString
	case s == DebugSeverity:
		return DebugString
	case s == AllSeverity:
		return AllString
	default:
		return "SeverityFilter(" + strconv.FormatInt(int64(s), 10) + ")"
	}
}

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
		lg = newLogger(&handler.NoopHandler{}, name, prefix, flags)
		// create with a noop writer/handler
		lg.filter = AllSeverity
		mu.Lock()
		defer mu.Unlock()
		loggers4go[name] = lg
	}
	return lg
}

// AddStdoutHandler adds a logger that writes to stdout/console
func (l *Logger4go) AddStdoutHandler() (sh *handler.StdoutHandler, err error) {
	sh = &handler.StdoutHandler{}
	registerHandler(l, sh)

	return sh, nil
}

// AddStderrHandler adds a logger that writes to stderr/console
func (l *Logger4go) AddStderrHandler() (sh *handler.StderrHandler, err error) {
	sh = &handler.StderrHandler{}
	registerHandler(l, sh)

	return sh, nil
}

// AddStdFileHandler adds a file handler which rotates the log file 5 times with a maximum size of 1MB each
// starting with sequence no 1 and with compression and daily rotation disabled
func (l *Logger4go) AddStdFileHandler(filePath string) (fh *handler.FileHandler, err error) {

	fh, err = handler.NewStdFileHandler(filePath)
	if err != nil {
		return nil, err
	}
	registerHandler(l, fh)
	return fh, nil
}

// AddFileHandler adds a file handler with a specified max filesize, max number of rotations, file compression and daily rotation
func (l *Logger4go) AddFileHandler(filePath string, maxFileSize uint, maxRotation byte, isCompressFile, isDailyRotation bool) (fh *handler.FileHandler, err error) {

	fh, err = handler.NewFileHandler(filePath, maxFileSize, maxRotation, 1, isCompressFile, isDailyRotation)
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
func (l *Logger4go) AddSyslogHandler(protocol, ipaddr string, priority syslog.Priority, tag string) (sh *handler.SyslogHandler, err error) {
	sh, err = handler.NewSyslogHandler(protocol, ipaddr, priority, tag)
	if err != nil {
		return nil, err
	}
	registerHandler(l, sh)

	return sh, err
}

// AddHandler adds a custom handler which conforms to the Handler interface.
func (l *Logger4go) AddHandler(handler handler.Handler) {
	registerHandler(l, handler)
}

// RemoveHandler removes the handler from the logger.
func (l *Logger4go) RemoveHandler(handler handler.Handler) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i, h := range l.handlers {
		if h == handler {
			l.handlers = append(l.handlers[:i], l.handlers[i+1:]...)
			break
		}
	}
}

// Handlers returns a list of registered handlers
func (l *Logger4go) Handlers() []handler.Handler {
	return l.handlers
}

// Emergf log
func (l *Logger4go) Emergf(format string, v ...interface{}) {
	l.doPrintf(EmergSeverity, format, v...)
}

// Emerg log
func (l *Logger4go) Emerg(v ...interface{}) {
	l.doPrintf(EmergSeverity, "%s", v...)
}

// Emergf log
func Emergf(format string, v ...interface{}) {
	Logger.Emergf(format, v)
}

// Emerg log
func Emerg(v ...interface{}) {
	Logger.Emerg(v)
}

// Alertf log
func (l *Logger4go) Alertf(format string, v ...interface{}) {
	l.doPrintf(AlertSeverity, format, v...)
}

// Alert log
func (l *Logger4go) Alert(v ...interface{}) {
	l.doPrintf(AlertSeverity, "%s", v...)
}

// Alertf log
func Alertf(format string, v ...interface{}) {
	Logger.Alertf(format, v)
}

// Alert log
func Alert(v ...interface{}) {
	Logger.Alert(v)
}

// Critf log
func (l *Logger4go) Critf(format string, v ...interface{}) {
	l.doPrintf(CritSeverity, format, v...)
}

// Crit log
func (l *Logger4go) Crit(v ...interface{}) {
	l.doPrintf(CritSeverity, "%s", v...)
}

// Critf log
func Critf(format string, v ...interface{}) {
	Logger.Critf(format, v)
}

// Crit log
func Crit(v ...interface{}) {
	Logger.Crit(v)
}

// Errf log
func (l *Logger4go) Errf(format string, v ...interface{}) {
	l.doPrintf(ErrSeverity, format, v...)
}

// Err log
func (l *Logger4go) Err(v ...interface{}) {
	l.doPrintf(ErrSeverity, "%s", v...)
}

// Errf log
func Errf(format string, v ...interface{}) {
	Logger.Errf(format, v)
}

// Err log
func Err(v ...interface{}) {
	Logger.Err(v)
}

// Warningf log
func (l *Logger4go) Warningf(format string, v ...interface{}) {
	l.doPrintf(WarningSeverity, format, v...)
}

// Warning log
func (l *Logger4go) Warning(v ...interface{}) {
	l.doPrintf(WarningSeverity, "%s", v...)
}

// Warningf log
func Warningf(format string, v ...interface{}) {
	Logger.Warningf(format, v)
}

// Warning log
func Warning(v ...interface{}) {
	Logger.Warning(v)
}

// Warnf log
func (l *Logger4go) Warnf(format string, v ...interface{}) {
	l.doPrintf(WarningSeverity, format, v...)
}

// Warn log
func (l *Logger4go) Warn(v ...interface{}) {
	l.doPrintf(WarningSeverity, "%s", v...)
}

// Warnf log
func Warnf(format string, v ...interface{}) {
	Logger.Warnf(format, v)
}

//Warn log
func Warn(v ...interface{}) {
	Logger.Warn(v)
}

// Noticef log
func (l *Logger4go) Noticef(format string, v ...interface{}) {
	l.doPrintf(NoticeSeverity, format, v...)
}

// Notice log
func (l *Logger4go) Notice(v ...interface{}) {
	l.doPrintf(NoticeSeverity, "%s", v...)
}

// Noticef log
func Noticef(format string, v ...interface{}) {
	Logger.Noticef(format, v)
}

// Notice log
func Notice(v ...interface{}) {
	Logger.Notice(v)
}

// Infof log
func (l *Logger4go) Infof(format string, v ...interface{}) {
	l.doPrintf(InfoSeverity, format, v...)
}

// Info log
func (l *Logger4go) Info(v ...interface{}) {
	l.doPrintf(InfoSeverity, "%s", v...)
}

// Infof log
func Infof(format string, v ...interface{}) {
	Logger.Infof(format, v)
}

// Info log
func Info(v ...interface{}) {
	Logger.Info(v)
}

// Debugf log
func (l *Logger4go) Debugf(format string, v ...interface{}) {
	l.doPrintf(DebugSeverity, format, v...)
}

// Debug log
func (l *Logger4go) Debug(v ...interface{}) {
	l.doPrintf(DebugSeverity, "%s", v...)
}

// Debugf log
func Debugf(format string, v ...interface{}) {
	Logger.Debugf(format, v)
}

// Debug log
func Debug(v ...interface{}) {
	Logger.Debug(v)
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

func registerHandler(l *Logger4go, handler handler.Handler) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.handlers = append(l.handlers, handler)
	out := make([]io.Writer, 0)
	for _, h := range l.handlers {
		out = append(out, h)
	}
	l.Logger = log.New(io.MultiWriter(out...), l.Prefix(), l.Flags())
}
