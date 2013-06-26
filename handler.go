// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package logger

import (
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type ByteSize float64

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

// A Handler writes log records.
type Handler interface {
	// Writer interface
	Write(b []byte) (n int, err error)
	// Release any allocated resources
	Close() error
	// Return the handler's type name
	String() string
}

// Dummy handler used for a new logger instance. Log to noop.
type NoopHandler struct {
}

// Write to os.Stdout
type ConsoleHandler struct {
}

// Write to file.
type FileHandler struct {
	filePath string
	written  uint // bytes written
	rotate   byte // how many logs to keep around, 0 disabled
	size     uint // rotate at size
	seq      byte // next rotated log
	compress bool // compress rotated log
	daily    bool
	out      *os.File
	mutex    sync.Mutex
}

// Write to syslog.
type SyslogHandler struct {
	Out *syslog.Writer
}

func (nh *NoopHandler) Write(b []byte) (n int, err error) { return 0, nil }
func (nh *NoopHandler) Close() error                      { return nil }
func (nh *NoopHandler) String() string                    { return "NoopHandler" }

func (ch *ConsoleHandler) Write(b []byte) (n int, err error) {
	n, err = os.Stdout.Write(b)
	if n < len(b) {
		return n, errors.New("Unable to write all bytes to stdout")
	}
	return n, err
}

func (ch *ConsoleHandler) Close() error {
	return nil
}

func (ch *ConsoleHandler) String() string {
	return "ConsoleHandler"
}

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

func (sh *SyslogHandler) Close() error {
	return sh.Out.Close()
}

func (sh *SyslogHandler) String() string {
	return "SyslogHandler"
}

func (fh *FileHandler) Write(b []byte) (n int, err error) {
	n, err = fh.out.Write(b)
	if err != nil {
		return n, err
	}

	if n < len(b) {
		return n, errors.New("Unable to write all bytes to " + fh.filePath)
	}

	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.written += uint(n)
	if !fh.daily && fh.rotate > 0 && fh.size > 0 && fh.written >= fh.size {
		f, err := fh.rotateLog()
		if err != nil {
			return n, err
		}
		fh.written = 0
		fh.out = f
	}
	return n, err
}

func (fh *FileHandler) Close() error {
	if fh.out != nil {
		return fh.Close()
	}
	return nil
}

func (fh *FileHandler) Rotate() byte {
	return fh.rotate
}

func (fh *FileHandler) SetRotate(rotate byte) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.rotate = rotate
}

func (fh *FileHandler) Size() uint {
	return fh.size
}

func (fh *FileHandler) SetSize(size uint) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.size = size
}

func (fh *FileHandler) Compress() bool {
	return fh.compress
}

func (fh *FileHandler) SetCompress(compress bool) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.compress = compress
}

func (fh *FileHandler) Seq() byte {
	return fh.seq
}

func (fh *FileHandler) SetSeq(seq byte) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.seq = seq
}

func (fh *FileHandler) Daily() bool {
	return fh.daily
}

func (fh *FileHandler) SetDaily(daily bool) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	if !fh.daily && daily {
		go fh.rotateDaily()
	}
	fh.daily = daily
}

func (fh *FileHandler) String() string {
	return "FileHandler"
}

//
// Private
//
const (
	def_rotate = 5
	def_size   = 1 * MB
	def_seq    = 1
)

func newStdFileHandler(filePath string) (*FileHandler, error) {

	return newFileHandler(filePath, uint(def_size), def_rotate, def_seq, false, false)
}

func newFileHandler(filePath string, size uint, rotate byte, seq byte, compress, daily bool) (*FileHandler, error) {

	fh := &FileHandler{filePath: filePath, size: size, rotate: rotate, seq: seq, compress: compress, daily: daily}
	f, err := fh.rotateLog()
	if err != nil {
		return nil, err
	}

	fh.out = f
	if fh.daily {
		go fh.rotateDaily()
	}
	return fh, nil
}

func newSyslogHandler(protocol, ipaddr string, priority syslog.Priority, tag string) (sh *SyslogHandler, err error) {
	sh = &SyslogHandler{}

	sh.Out, err = syslog.Dial(protocol, ipaddr, priority, tag)
	if err != nil {
		return nil, err
	}

	return sh, nil
}

func (fh *FileHandler) rotateLog() (f *os.File, err error) {
	if fh.out != nil {
		fh.out.Close()
	}

	rotatefile := fh.filePath + "." + strconv.Itoa(int(fh.seq))

	err = os.Rename(fh.filePath, rotatefile)
	if err != nil {
		if !os.IsNotExist(err) { // fine if the file does not exists
			return nil, err
		}
	}

	fh.seq++
	if fh.seq > fh.rotate {
		fh.seq = 1
	}

	if fh.compress && fh.rotate > 0 {
		if _, err := os.Stat(rotatefile); err == nil {
			go compress(rotatefile)
		}
	}

	f, err = os.OpenFile(fh.filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fh *FileHandler) rotateDaily() {
	for {
		h, m, s := time.Now().Clock()
		d := time.Duration((24-h)*3600-m*60-1*s) * time.Second
		t := time.NewTimer(d)
		select {
		case <-t.C:
			f, err := fh.rotateLog()
			if err != nil {
				fmt.Errorf("Failed to rotate log daily: %v", err)
			}
			fh.written = 0
			fh.out = f
		}
		if !fh.daily {
			break
		}
	}
}

func compress(filePath string) {
	err := exec.Command("gzip", "-f", filePath).Run()
	if err != nil {
		fmt.Errorf("%v", err)
	}
}
