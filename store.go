package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Store is the interface that provides Reader and Writer methods for managing
// git objects from data storage.
type Store interface {
	// Object resolves hash to reader of underlying data. Implemenations must
	// resolve abbreviated hashes.
	Object(hash string) (io.Reader, error)

	// Reader initializes a new Reader by the given hash. Implementations need
	// to resolve abbreviated hashes and provide a proper io.Reader to Reader.
	Reader(hash string, options ...func(*Reader)) (*Reader, error)

	// Writer initializes a new Writer. Implementations must wrap Writer
	// so that Writer.Close() flushes content to storage.
	Writer() Writer
}

// DiskStore implements Store for git repositories on disk.
//
//  dir, _ := os.Getwd()
//  store := git.DiskStore(dir)
type DiskStore string

// Dir traverses tree backwards to locate git directory.
// Typically used to create DiskStore.
func Dir(x string) string {
	exists := func(args ...string) bool {
		for _, arg := range args {
			if _, err := os.Stat(arg); os.IsNotExist(err) {
				return false
			}
		}
		return true
	}

	d := x
	for {
		if exists(filepath.Join(d, ".git")) {
			return filepath.Join(d, ".git")
		}
		// TODO probably not a very good check
		if exists(filepath.Join(d, "config"), filepath.Join(d, "HEAD"), filepath.Join(d, "objects")) {
			return d // bare
		}
		x = filepath.Dir(d)
		if x == d || x == "/" || x == "." {
			panic("not a git repository")
		}
		d = x
	}
}

// Object resolves hash to reader of underlying data.
func (st DiskStore) Object(hash string) (io.Reader, error) {
	d := filepath.Join(string(st), "objects", hash[:2])
	s := filepath.Join(d, hash[2:])
	if f, err := os.Open(s); !os.IsNotExist(err) {
		return f, err
	}
	dir, err := os.Open(d)
	if err != nil {
		return nil, err
	}
	ns, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	var match string
	for _, e := range ns {
		if strings.HasPrefix(e, hash[2:]) {
			if match != "" {
				return nil, errors.New("ambigious hash " + hash)
			}
			match = e
		}
	}
	if match == "" {
		return nil, fmt.Errorf("object hash %s does not exist", hash)
	}
	return os.Open(filepath.Join(d, match))
}

// Reader returns a new Reader for the given object hash or error otherwise.
// The object's type and length are immediately available.
// Callers must call Reader.Close() when done.
func (st DiskStore) Reader(hash string, options ...func(*Reader)) (*Reader, error) {
	r, err := st.Object(hash)
	if err != nil {
		return nil, err
	}
	return NewReader(r, options...)
}

// Writer provides a new Writer that buffers data to a temporary file.
// Callers must call Writer.Close() to flush data to storage.
func (st DiskStore) Writer() Writer {
	tmp, err := ioutil.TempFile("", "gitdiskstore")
	if err != nil {
		panic(err)
	}
	return &diskCloser{NewWriter(tmp), st, tmp}
}

// TempStore provides a DiskStore in a temporary directory. Callers are responsible
// for removing the directory when done.
//
// TempStore may not provide a valid git repository. See Init source for layout.
//
//  store := git.TempStore()
//  defer os.RemoveAll(string(store))
func TempStore() DiskStore {
	name, err := ioutil.TempDir("", "gittempstore")
	if err != nil {
		panic(err)
	}
	Init(name, false)
	// since not bare, call Dir
	return DiskStore(Dir(name))
}

// diskCloser wraps a Writer delivered by DiskStore to finalize writing
// object to disk once Writer.Close() is called.
type diskCloser struct {
	Writer
	st DiskStore
	f  *os.File
}

func (g *diskCloser) Close() error {
	if err := g.Writer.Close(); err != nil {
		return err
	}
	hash := g.Writer.Hash()
	p := filepath.Join(string(g.st), "objects", hash[:2], hash[2:])
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	// TODO "invalid cross-device link" with this in random cases
	// even when not on a different partition
	// os.Rename(g.f.Name(), p)

	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if err != nil {
		return err
	}
	defer f.Close()

	g.f.Seek(0, 0)
	if _, err := io.Copy(f, g.f); err != nil {
		return err
	}

	g.f.Close()
	os.Remove(g.f.Name())

	return nil
}

// MemStore implements Store in-memory. No guarantees are ensured with thread safety
// as the implementation relies on the uniqueness of SHA1 hash, in so far as a hash
// collision could break concurrent access.
func MemStore() Store {
	m := make(map[string][]byte)
	return memStore(m)
}

type memStore map[string][]byte

func (st memStore) Object(hash string) (io.Reader, error) {
	if b, ok := st[hash]; ok {
		return bytes.NewReader(b), nil
	}
	var match string
	for k := range st {
		if strings.HasPrefix(k, hash) {
			if match != "" {
				return nil, errors.New("ambigious hash " + hash)
			}
			match = k
		}
	}
	if match == "" {
		return nil, fmt.Errorf("object hash %s does not exist", hash)
	}
	return bytes.NewReader(st[match]), nil
}

func (st memStore) Reader(hash string, options ...func(*Reader)) (*Reader, error) {
	r, err := st.Object(hash)
	if err != nil {
		return nil, err
	}
	return NewReader(r, options...)
}

func (st memStore) Writer() Writer {
	g := &memCloser{st: st}
	g.Writer = NewWriter(&g.buf)
	return g
}

type memCloser struct {
	Writer
	st  memStore
	buf bytes.Buffer
}

func (g *memCloser) Close() error {
	if err := g.Writer.Close(); err != nil {
		return err
	}
	hash := g.Writer.Hash()
	if _, ok := g.st[hash]; ok {
		return fmt.Errorf("object with hash %s exists", hash)
	}
	g.st[hash] = g.buf.Bytes()
	return nil
}
