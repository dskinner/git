package git

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
)

// Writer writes git object format for blobs, trees, and commits.
//
// TODO short writes on tree objects are likely to fail.
type Writer interface {
	// Write writes p to the underlying Writer. Write returns an error
	// if caller has not first called WriteHeader.
	//
	// Close flushes written data. This does not close the original writer.
	io.WriteCloser

	// WriteHeader must be called before writing any data. If you don't know
	// the size of data to be written, pass a negative integer; size is always
	// ignored for tree types. In such cases, an intermediary file is used
	// to determine size.
	WriteHeader(t Type, size int) (int, error)

	// Hash returns sha1 sum of data written.
	Hash() string
}

type writer struct {
	io.Writer
	zw *zlib.Writer
	hh hash.Hash

	// used in case size is unknown
	tmp *os.File
	tw  *treeWriter
	t   Type

	wroteHeader bool
	err         error
	finalize    func() error
}

// NewWriter returns a new Writer that writes to staging.
func NewWriter(staging io.Writer) Writer {
	// TODO need to insert treeWriter here, before zlib.NewWriter, also not sure about hh ???
	// actually, maybe after WriteHeader is done.
	g := &writer{
		zw: zlib.NewWriter(staging),
		hh: sha1.New(),
	}
	g.Writer = io.MultiWriter(g.zw, g.hh)
	return g
}

func (g *writer) WriteHeader(t Type, s int) (n int, err error) {
	if g.wroteHeader {
		return 0, errors.New("Header already written.")
	}
	g.t = t
	g.wroteHeader = true
	if t == Tree || s < 0 {
		g.tmp, err = ioutil.TempFile("", "gitwriter")
		g.tw = &treeWriter{
			Writer: g.tmp,
			rbuf:   new(bytes.Buffer),
			wbuf:   new(bytes.Buffer),
			sum:    make([]byte, 20),
		}
	} else {
		n, err = g.Write(t.Header(s))
	}
	return
}

func (g *writer) Write(p []byte) (int, error) {
	if !g.wroteHeader {
		return 0, errors.New("Must call WriteHeader before calling Write.")
	}
	if g.tw != nil {
		return g.tw.Write(p)
	}
	return g.Writer.Write(p)
}

func (g *writer) Close() error {
	if g.tw != nil {
		defer g.tmp.Close()
		defer os.Remove(g.tmp.Name())

		fi, err := g.tmp.Stat()
		if err != nil {
			return err
		}
		size := int(fi.Size())
		_, err = g.tmp.Seek(0, 0)
		if err != nil {
			return err
		}
		_, err = g.Writer.Write(g.t.Header(size))
		if err != nil {
			return err
		}
		_, err = io.Copy(g.Writer, g.tmp)
		if err != nil {
			return err
		}
	}

	if err := g.zw.Close(); err != nil {
		return err
	}
	if g.finalize != nil {
		return g.finalize()
	}
	return nil
}

func (g *writer) Hash() string {
	return fmt.Sprintf("%x", g.hh.Sum(nil))
}

// treeWriter handles PrettyReader formatted tree stream.
// TODO this could use some work.
type treeWriter struct {
	io.Writer

	rbuf *bytes.Buffer
	wbuf *bytes.Buffer

	// len 20
	sum []byte
}

// TODO this is going to bomb on a short read
func (g *treeWriter) Write(p []byte) (n int, err error) {
	var mode, h, name []byte

	g.rbuf.Write(p)
	r := bufio.NewReader(g.rbuf)

	for {
		mode, err = r.ReadBytes(' ')
		if err != nil {
			break
		}
		if mode[0] == '0' {
			mode = mode[1:]
		}

		// discard type
		_, err = r.ReadBytes(' ')
		if err != nil {
			break
		}

		h, err = r.ReadBytes('\t')
		if err != nil {
			break
		}
		h = h[:len(h)-1]
		_, err = hex.Decode(g.sum, h)
		if err != nil {
			break
		}

		name, err = r.ReadBytes('\n')
		if err != nil {
			break
		}
		name[len(name)-1] = '\x00'

		g.wbuf.Write(mode)
		g.wbuf.Write(name)
		g.wbuf.Write(g.sum)
	}

	n, _ = g.Writer.Write(g.wbuf.Next(len(p)))

	return n, err
}
