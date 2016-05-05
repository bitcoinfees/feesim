package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
)

// DebugLog provides a logger which allows filtering of [DEBUG] log messages
type DebugLog struct {
	Logger *log.Logger
	out    io.Writer
	r      *io.PipeReader
	debug  bool
	mux    sync.Mutex
}

func NewDebugLog(out io.Writer, prefix string, flag int) *DebugLog {
	r, w := io.Pipe()
	logger := log.New(w, prefix, flag)
	l := &DebugLog{
		Logger: logger,
		out:    out,
		r:      r,
	}
	go l.filter("[DEBUG]")
	return l
}

func (l *DebugLog) SetDebug(d bool) {
	l.mux.Lock()
	defer l.mux.Unlock()
	l.debug = d
}

func (l *DebugLog) Debug() bool {
	l.mux.Lock()
	defer l.mux.Unlock()
	return l.debug
}

func (l *DebugLog) Close() {
	l.r.Close()
	if c, ok := l.out.(io.Closer); ok {
		c.Close()
	}
}

func (l *DebugLog) filter(debugPrefix string) {
	s := bufio.NewScanner(l.r)
	for s.Scan() {
		m := s.Text()
		if l.Debug() || !strings.Contains(m, debugPrefix) {
			fmt.Fprintln(l.out, m)
		}
	}
}
