package git

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/hex"
	"io"
	"strconv"
)

// PrettyReader decodes sha1 sum of references in tree objects
// and parses reference types. This has no effect on other types.
//
//  NewReader(r, PrettyReader)
func PrettyReader(g *Reader) { g.pretty = true }

// Reader reads git object format for blobs, trees, and commits.
//
// TODO short reads on tree objects are likely to fail.
type Reader struct {
	io.Reader

	pretty bool

	zr  io.ReadCloser
	t   Type
	n   int
	err error
}

// NewReader returns Reader for r. Most users will want to call store.Reader(r).
func NewReader(r io.Reader, options ...func(*Reader)) (*Reader, error) {
	g := new(Reader)
	for _, opt := range options {
		opt(g)
	}
	if err := g.Reset(r); err != nil {
		return nil, err
	}
	return g, nil
}

// Type returns type of object to be read.
func (g *Reader) Type() Type { return g.t }

// Len returns the length of object's content to be read.
func (g *Reader) Len() int { return g.n }

// Close does not close the original reader passed in.
func (g *Reader) Close() error { return g.zr.Close() }

// Reset clears the state of the Reader g such that it is equivalent to its
// initial state from NewReader, but instead reading from r. Any options
// previously set are retained.
func (g *Reader) Reset(r io.Reader) error {
	var err error
	if g.zr == nil {
		if g.zr, err = zlib.NewReader(r); err != nil {
			return err
		}
	} else if err = g.zr.(flate.Resetter).Reset(r, nil); err != nil {
		return err
	}

	// 28 byte limit means reader can't support content larger than 18,500 petabytes.
	b := bufio.NewReader(io.LimitReader(g.zr, 28))
	g.Reader = io.MultiReader(b, g.zr)

	// read header
	t, err := b.ReadBytes(' ')
	if err != nil {
		return err
	}
	g.t = ParseType(t[:len(t)-1])

	n, err := b.ReadBytes('\x00')
	if err != nil {
		return err
	}
	g.n, err = strconv.Atoi(string(n[:len(n)-1]))

	// trees are different
	if g.pretty && g.t == Tree {
		g.Reader = &treeReader{
			Reader: bufio.NewReaderSize(g.Reader, 20),
			hash:   make([]byte, 40),
		}
	}

	return err
}

type treeReader struct {
	// init with min size 20 to peek sha1
	*bufio.Reader

	buf bytes.Buffer

	// init with length 40
	hash []byte
}

func (g *treeReader) Read(p []byte) (n int, err error) {
	var mode, name, sum []byte

	// each iteration reads a single line as follows:
	// [mode] [name]\x00[[20]byte]
	//
	// output is as follows (where type is determined by mode[0]):
	// [mode] [type] [hexenc]\t[name]
	for g.buf.Len() <= len(p) {
		mode, err = g.Reader.ReadBytes(' ')
		if err != nil {
			break
		}

		switch mode[0] {
		case '1':
			g.buf.Write(mode)
			g.buf.Write([]byte("blob "))
		case '4':
			g.buf.WriteRune('0')
			g.buf.Write(mode)
			g.buf.Write([]byte("tree "))
		default:
			panic("unrecognized mode")
		}

		// TODO custom error about malformed tree for below

		name, err = g.Reader.ReadBytes('\x00')
		if err != nil {
			break
		}
		name = name[:len(name)-1]

		sum, err = g.Reader.Peek(20)
		if err != nil {
			break
		}
		g.Reader.Discard(20)

		hex.Encode(g.hash, sum)
		g.buf.Write(g.hash)
		g.buf.WriteRune('\t')
		g.buf.Write(name)
		g.buf.WriteRune('\n')
	}

	n = copy(p, g.buf.Next(len(p)))

	return n, err
}
