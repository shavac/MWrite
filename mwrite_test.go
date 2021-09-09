package MWrite

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func makeTempFiles(n int) ([]io.WriteCloser, error) {
	ws := []io.WriteCloser{}
	for i := 0; i < n; i++ {
		f, err := ioutil.TempFile(os.TempDir(), "mwtest")
		if err != nil {
			return nil, err
		}
		ws = append(ws, f)
	}
	return ws, nil
}
func TestClose(t *testing.T) {
	ws, err := makeTempFiles(3)
	if err != nil {
		t.FailNow()
	}
	te := NewTee(256)
	te.AddBufferedWriters(1024, ws...)
	bch := te.AddBufferedChannel(1024)
	bs := []byte("01234567890abcdefghijklmnopqrstuvwxyz")
	go func() {
		for i := 0; i < 100000; i++ {
			te.Write(bs)
		}
		te.Close()
	}()
	<-bch
	time.Sleep(time.Second)
}
