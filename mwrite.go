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

type Tee struct {
	sync.RWMutex
	buf    chan []byte
	spouts map[chan []byte]io.WriteCloser
	closed bool
}

func NewTee(buflen int) *Tee {
	h := Tee{
		RWMutex: sync.RWMutex{},
		buf:     make(chan []byte, buflen),
		spouts:  make(map[chan []byte]io.WriteCloser),
		closed:  false,
	}
	go func() {
		for bs := range h.buf {
			h.RLock()
			for ch, w := range h.spouts {
				if w == nil {
					ch <- bs
				} else if _, err := w.Write(bs); err != nil {
					h.RUnlock()
					h.DeleteWriter(w)
					h.RLock()
				}
			}
			h.RUnlock()
		}
		for _, w := range h.spouts {
			h.RLock()
			w.Close()
			h.RUnlock()
		}
		h.spouts = nil
	}()
	return &h
}

func (t *Tee) Write(bs []byte) (int, error) {
	if t.closed {
		return 0, ErrClosed
	}
	t.buf <- bs
	return len(bs), nil
}

func (t *Tee) Close() error {
	t.Lock()
	defer t.Unlock()
	if t.closed {
		return ErrClosed
	}
	close(t.buf)
	t.closed = true
	return nil
}

func (t *Tee) AddSpout(sp *spout) {
	t.Lock()
	defer t.Unlock()
	t.spouts[sp.buf] = sp
}

func (h *Tee) AddBufferedWriters(buflen int, wcs ...io.WriteCloser) {
	for _, wc := range wcs {
		sp := newSpout(buflen, wc)
		h.AddSpout(sp)
	}
}

func (h *Tee) AddBufferedChannel(buflen int) chan []byte {
	sp := newSpout(buflen, nil)
	h.AddSpout(sp)
	return sp.buf
}

func (h *Tee) DeleteWriter(dwc io.WriteCloser) {
	h.Lock()
	defer h.Unlock()
	for ch, wc := range h.spouts {
		if wc == dwc {
			delete(h.spouts, ch)
			close(ch)
		}
	}
}

func (h *Tee) DeleteChan(dch chan []byte) {
	h.Lock()
	defer h.Unlock()
	delete(h.spouts, dch)
}
