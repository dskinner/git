package git

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Store is the interface that provides Reader and Writer methods for managing
// git objects from data storage.
type Store interface {
	// Reader initializes a new Reader by the given hash. Implementations need
	// only to provide a proper io.Reader to Reader.
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

func (st DiskStore) objectPath(hash string) string {
	return filepath.Join(string(st), "objects", hash[:2], hash[2:])
}

// Reader returns a new Reader for the given object hash or error otherwise.
// The object's type and length are immediately available.
// Callers must call Reader.Close() when done.
func (st DiskStore) Reader(hash string, options ...func(*Reader)) (*Reader, error) {
	f, err := os.Open(st.objectPath(hash))
	if err != nil {
		return nil, err
	}
	return NewReader(f, options...)
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
	p := g.st.objectPath(g.Writer.Hash())
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
