// Copyright (c) 2013 - Alex Yu <alex@alexyu.se>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

package logger

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// FileHandler writes to file.
type FileHandler struct {
	filePath string
	written  uint // bytes written
	rotate   byte // how many log files to rotate between
	size     uint // rotate at file size
	seq      byte // next rotated log filename sequence
	compress bool // compress rotated logs
	daily    bool // rotate daily
	out      *os.File
	mutex    sync.Mutex
}

// Write log message to file and rotate the file if necessary.
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

// Close handler
func (fh *FileHandler) Close() error {
	if fh.out != nil {
		return fh.Close()
	}
	return nil
}

// Rotate returns how many log files to rotate between.
func (fh *FileHandler) Rotate() byte {
	return fh.rotate
}

// SetRotate sets the number of log files to rotate between.
func (fh *FileHandler) SetRotate(rotate byte) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.rotate = rotate
}

// Size returns the max log file size.
func (fh *FileHandler) Size() uint {
	return fh.size
}

// SetSize sets the max log file size.
func (fh *FileHandler) SetSize(size uint) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.size = size
}

// Compress returns true if file compression is set for the rotated log file.
func (fh *FileHandler) Compress() bool {
	return fh.compress
}

// SetCompress sets whether file compression should be used for the rotated log file.
func (fh *FileHandler) SetCompress(compress bool) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.compress = compress
}

// Seq returns the next log file sequence number for the rotated log file.
func (fh *FileHandler) Seq() byte {
	return fh.seq
}

// SetSeq sets the log file sequence number for the next rotated log file.
func (fh *FileHandler) SetSeq(seq byte) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	fh.seq = seq
}

// Daily returns whether the log file rotates daily.
func (fh *FileHandler) Daily() bool {
	return fh.daily
}

// SetDaily sets whether the log file should rotate daily.
func (fh *FileHandler) SetDaily(daily bool) {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()

	if !fh.daily && daily {
		go fh.rotateDaily()
	}
	fh.daily = daily
}

// String returns the handler name.
func (fh *FileHandler) String() string {
	return "FileHandler"
}

// DefRotatation and DefFileSize sets the default number of rotated files and the max size per log file.
const (
	DefRotatation = 5
	DefFileSize   = uint(1 * MB)
	defStartSeq   = 1
)

func newStdFileHandler(filePath string) (*FileHandler, error) {
	return newFileHandler(filePath, DefFileSize, DefRotatation, defStartSeq, false, false)
}

func newFileHandler(filePath string, maxFileSize uint, maxRotation byte, startSeq byte, compress bool, daily bool) (*FileHandler, error) {
	fh := &FileHandler{filePath: filePath, size: maxFileSize, rotate: maxRotation, seq: startSeq, compress: compress, daily: daily}
	// find a free log file sequence no
	fh.findSequence()
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

func (fh *FileHandler) findSequence() {
	// Find a free rotated log file sequence no
	fileName := "%v.%d"
	if (fh.compress) {
		fileName = "%v.%d.gz"
	}

	rotateFile := fmt.Sprintf(fileName, fh.filePath, fh.seq)
	for {
		if _, err := os.Stat(rotateFile); os.IsNotExist(err) {
			// found seq no, file does not exist
			break
		}
		fh.seq++
		rotateFile = fmt.Sprintf(fileName, fh.filePath, fh.seq)
	}
}

func (fh *FileHandler) rotateLog() (f *os.File, err error) {
	// close log file
	if fh.out != nil {
		// ignore err
		fh.out.Close()
	}

	if fh.rotate > 0 {
		if fh.seq > fh.rotate {
			fh.seq = 1
		}

		rotateFileName := fmt.Sprintf("%v.%d", fh.filePath, fh.seq)
		if _, err := os.Stat(fh.filePath); !os.IsNotExist(err) {
			// rename/move only if it exist
			err := os.Rename(fh.filePath, rotateFileName)
			if err != nil {
				return nil, err
			}

			if fh.compress {
				if _, err := os.Stat(rotateFileName); !os.IsNotExist(err) {
					go compress(rotateFileName)
				}
			}
			fh.seq++
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
				_ = fmt.Errorf("Failed to rotate log daily: %v", err)
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
		_ = fmt.Errorf("%v", err)
	}
}
