/*
Copyright (c) 2013, Alex Yu <alex@alexyu.se>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of the <organization> nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Package logger provides Logger4go which is a simple wrapper around go's log.Logger.
//
// It provides three log handlers ConsoleHandler|FileHandler|SyslogHandler,
// wrapper methods named after syslog's severity levels and embedds log.Logger to provide
// seemless access to its methods as well if needed.
//
// Supports:
//
// 	- Write to multiple handlers, e.g., log to console, file and syslog at the same time.
// 	- Use more than one logger instance. Each with its own set of handlers.
// 	- Log file rotation (size of daily) and compression.
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
// 	main 2013/06/21 08:22:14 -alert- An alert message
// 	main 2013/06/21 08:22:14 -emerg- An emergency message
//
// TODO:
// 	- Custom header format
//	- Read settings from config file
package logger

import (
	"io"
	"log"
	"log/syslog"
)

// Logger4go embedds go's log.Logger as an anonymous field and
// so those methods are also exposed/accessable via Logger4go.
type Logger4go struct {
	name     string
	handlers []Handler
	filter   Filter
	*log.Logger
}

type Filter int

// log filters
const (
	EMERG Filter = 1 << iota
	ALERT
	CRIT
	ERR
	WARN
	NOTICE
	INFO
	DEBUG
	ALL = EMERG | ALERT | CRIT | ERR | WARN | NOTICE | INFO | DEBUG
)

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

// Std returns the standard logger instance with a console handler.
func Std() *Logger4go {
	return Get("std")
}

// Get returns a logger with the specified name and default log header flags.
// If it does not exist a new instance will be created.
func Get(name string) *Logger4go {
	return GetWithFlags(name, log.LstdFlags)
}

// GetWithFlags returns a logger with the specified name and log header flags.
// If it does exist a new instance will be created.
func GetWithFlags(name string, flags int) *Logger4go {
	lg, ok := loggers4go[name]
	if !ok {
		// create with a noop writer/handler
		lg = newLogger(&NoopHandler{}, name, name+" ", flags)
		lg.filter = ALL
		loggers4go[name] = lg
	}

	return lg
}

func (l *Logger4go) AddConsoleHandler() (ch *ConsoleHandler, err error) {
	ch = &ConsoleHandler{}
	saveHandler(l, ch)

	return ch, nil
}

// AddStdFileHandler adds a file handler which rotates the log file 5 times with a maximum size of 1MB each
// starting with sequence no 1 and with compression and daily rotation disabled
func (l *Logger4go) AddStdFileHandler(filePath string) (fh *FileHandler, err error) {

	fh, err = newStdFileHandler(filePath)
	if err != nil {
		return nil, err
	}
	saveHandler(l, fh)
	return fh, nil
}

// AddFileHandler adds a file handler with a specified max filesize, max number of rotations, a starting sequence no
// and if compression and daily rotation is enabled
func (l *Logger4go) AddFileHandler(filePath string, size uint, rotate byte, seq byte, compress, daily bool) (fh *FileHandler, err error) {

	fh, err = newFileHandler(filePath, size, rotate, seq, compress, daily)
	if err != nil {
		return nil, err
	}
	saveHandler(l, fh)
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
	saveHandler(l, sh)

	return sh, err
}

// AddHandler adds a custom handler which conforms to the Handler interface.
func (l *Logger4go) AddHandler(handler Handler) {
	saveHandler(l, handler)
}

func (l *Logger4go) Handlers() []Handler {
	return l.handlers
}

func (l *Logger4go) Emergf(format string, v ...interface{}) {
	l.doPrintf(EMERG, format, v...)
}

func (l *Logger4go) Emerg(v ...interface{}) {
	l.doPrintf(EMERG, "%s", v...)
}

func (l *Logger4go) Alertf(format string, v ...interface{}) {
	l.doPrintf(ALERT, format, v...)
}

func (l *Logger4go) Alert(v ...interface{}) {
	l.doPrintf(ALERT, "%s", v...)
}

func (l *Logger4go) Critf(format string, v ...interface{}) {
	l.doPrintf(CRIT, format, v...)
}

func (l *Logger4go) Crit(v ...interface{}) {
	l.doPrintf(CRIT, "%s", v...)
}

func (l *Logger4go) Errf(format string, v ...interface{}) {
	l.doPrintf(ERR, format, v...)
}

func (l *Logger4go) Err(v ...interface{}) {
	l.doPrintf(ERR, "%s", v...)
}

func (l *Logger4go) Warnf(format string, v ...interface{}) {
	l.doPrintf(WARN, format, v...)
}

func (l *Logger4go) Warn(v ...interface{}) {
	l.doPrintf(WARN, "%s", v...)
}

func (l *Logger4go) Noticef(format string, v ...interface{}) {
	l.doPrintf(NOTICE, format, v...)
}

func (l *Logger4go) Notice(v ...interface{}) {
	l.doPrintf(NOTICE, "%s", v...)
}

func (l *Logger4go) Infof(format string, v ...interface{}) {
	l.doPrintf(INFO, format, v...)
}

func (l *Logger4go) Info(v ...interface{}) {
	l.doPrintf(INFO, "%s", v...)
}

func (l *Logger4go) Debugf(format string, v ...interface{}) {
	l.doPrintf(DEBUG, format, v...)
}

func (l *Logger4go) Debug(v ...interface{}) {
	l.doPrintf(DEBUG, "%s", v...)
}

func (l *Logger4go) IsFilterSet(f Filter) bool {
	return f&l.filter == f
}

func (l *Logger4go) SetFilter(fFlags Filter) {
	l.filter = fFlags
}

func (l *Logger4go) Filter(fFlags Filter) Filter {
	return l.filter
}

func (l *Logger4go) Flags() int {
	return l.Logger.Flags()
}

func (l *Logger4go) SetFlags(flag int) {
	l.Logger.SetFlags(flag)
}

func (l *Logger4go) Prefix() string {
	return l.Logger.Prefix()
}

func (l *Logger4go) SetPrefix(prefix string) {
	l.Logger.SetPrefix(prefix)
}

func (l *Logger4go) SetOutput(out io.Writer) {

	l.Logger = log.New(out, l.Logger.Prefix(), l.Logger.Flags())
}

//
// Private
//
var loggers4go = make(map[string]*Logger4go)

var severity = []string{"-emerg-", "-alert-", "-crit-", "-err-", "-warn-", "-notice-", "-info-", "-debug-", ""}

func init() {
	Get("std").AddConsoleHandler()
}

func (l *Logger4go) doPrintf(f Filter, format string, v ...interface{}) {
	if l.IsFilterSet(f) {
		i := DEBUG
		switch f {
		default:
			i = 7
		case EMERG:
			i = 0
		case ALERT:
			i = 1
		case CRIT:
			i = 2
		case ERR:
			i = 3
		case WARN:
			i = 4
		case NOTICE:
			i = 5
		case INFO:
			i = 6
		case DEBUG:
			i = 7
		}
		l.Printf(severity[i]+" "+format, v...)
	}
}

func newLogger(out io.Writer, name string, prefix string, flags int) *Logger4go {
	return &Logger4go{name: name, Logger: log.New(out, prefix, flags)}
}

func saveHandler(l *Logger4go, handler Handler) {
	l.handlers = append(l.handlers, handler)
	out := make([]io.Writer, 0)
	for _, h := range l.handlers {
		out = append(out, h)
	}
	l.Logger = log.New(io.MultiWriter(out...), l.Prefix(), l.Flags())
}
