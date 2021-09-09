package MWrite

import (
	"errors"
	"io"
	"sync"
)

var ErrClosed = errors.New("closed")

type spout struct {
	buf    chan []byte
	target io.WriteCloser
	closed bool
}

// Writer can be nil
func newSpout(buflen int, wc io.WriteCloser) *spout {
	v := &spout{
		buf:    make(chan []byte, buflen),
		target: wc,
		closed: false,
	}
	go func() {
		if v.target == nil {
			return
		}
		for b := range v.buf {
			if _, err := v.target.Write(b); err != nil {
				break
			}
		}
		v.target.Close()
		v.closed = true
	}()
	return v
}

func (v spout) Write(buf []byte) (int, error) {
	if v.closed {
		return 0, ErrClosed
	}
	v.buf <- buf
	return len(buf), nil
}

func (v *spout) Close() error {
	if v.closed {
		return ErrClosed
	}
	close(v.buf)
	v.closed = true
	return nil
}

type tee struct {
	sync.RWMutex
	buf    chan []byte
	spouts map[chan []byte]io.WriteCloser
	closed bool
}

func NewTee(buflen int) *tee {
	h := tee{
		RWMutex: sync.RWMutex{},
		buf:     make(chan []byte, buflen),
		spouts:  make(map[chan []byte]io.WriteCloser),
		closed:  false,
	}
	go func() {
		for bs := range h.buf {
			h.RLock()
			for ch, v := range h.spouts {
				if _, err := v.Write(bs); err != nil {
					h.Lock()
					delete(h.spouts, ch)
					h.Unlock()
				}
			}
			h.RUnlock()
		}
		for _, v := range h.spouts {
			h.RLock()
			v.Close()
			h.RUnlock()
		}
		h.spouts = nil
	}()
	return &h
}

func (t *tee) Write(bs []byte) (int, error) {
	if t.closed {
		return 0, ErrClosed
	}
	t.buf <- bs
	return len(bs), nil
}

func (h *tee) Close() error {
	h.Lock()
	defer h.Unlock()
	if h.closed {
		return ErrClosed
	}
	close(h.buf)
	h.closed = true
	return nil
}

func (h *tee) AddSpout(sp *spout) {
	h.Lock()
	defer h.Unlock()
	h.spouts[sp.buf] = sp
}

func (h *tee) AddBufferedWriters(buflen int, wcs ...io.WriteCloser) {
	for _, wc := range wcs {
		sp := newSpout(buflen, wc)
		h.AddSpout(sp)
	}
}

func (h *tee) AddBufferedChannel(buflen int) chan []byte {
	sp := newSpout(buflen, nil)
	h.AddSpout(sp)
	return sp.buf
}
