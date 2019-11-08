// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package handler

import (
	"errors"
	"os"
)

// ByteSize type for the log file size
type ByteSize float64

// Log file size constants
const (
	_           = iota
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)

// Handler is an interface to different log/logger handlers.
type Handler interface {
	// Writer interface
	Write(b []byte) (n int, err error)
	// Release any allocated resources
	Close() error
	// Return the handler's type name
	String() string
}

// NoopHandler is a dummy handler used for a new logger instance. Log to noop.
type NoopHandler struct {
}

// StdoutHandler writes to os.Stdout.
type StdoutHandler struct {
}

// StderrHandler writes to os.Stderr
type StderrHandler struct {
}

// Write a log message.
func (nh *NoopHandler) Write(b []byte) (n int, err error) {
	return 0, nil
}

// Close the handler.
func (nh *NoopHandler) Close() error {
	return nil
}

// String returns the handler name.
func (nh *NoopHandler) String() string {
	return "NoopHandler"
}

// Write a log message.
func (ch *StdoutHandler) Write(b []byte) (n int, err error) {
	n, err = os.Stdout.Write(b)
	if n < len(b) {
		return n, errors.New("Unable to write all bytes to stdout")
	}
	return n, err
}

// Close handler.
func (ch *StdoutHandler) Close() error {
	return nil
}

// String returns the handler name.
func (ch *StdoutHandler) String() string {
	return "StdoutHandler"
}

// Write a log message.
func (ch *StderrHandler) Write(b []byte) (n int, err error) {
	n, err = os.Stderr.Write(b)
	if n < len(b) {
		return n, errors.New("Unable to write all bytes to stdout")
	}
	return n, err
}

// Close handler.
func (ch *StderrHandler) Close() error {
	return nil
}

// String returns the handler name.
func (ch *StderrHandler) String() string {
	return "StderrHandler"
}
