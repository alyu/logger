// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package handler

import (
	"errors"
	"log/syslog"
)

// SyslogHandler writes to syslog.
type SyslogHandler struct {
	Out *syslog.Writer
}

// Write log message.
func (sh *SyslogHandler) Write(b []byte) (n int, err error) {
	n, err = sh.Out.Write(b)
	if err != nil {
		return n, err
	}

	if n < len(b) {
		return n, errors.New("Unable to write all bytes to syslog")
	}

	return n, err
}

// Close handler.
func (sh *SyslogHandler) Close() error {
	return sh.Out.Close()
}

// String returns the handler name.
func (sh *SyslogHandler) String() string {
	return "SyslogHandler"
}

// NewSyslogHandler returns a handler for syslog
func NewSyslogHandler(protocol, ipaddr string, priority syslog.Priority, tag string) (sh *SyslogHandler, err error) {
	sh = &SyslogHandler{}

	sh.Out, err = syslog.Dial(protocol, ipaddr, priority, tag)
	if err != nil {
		return nil, err
	}

	return sh, nil
}

