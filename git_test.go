// Package git provides a limited set of git core methods.
package git

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	command func(name string, arg ...string) *exec.Cmd
	store   DiskStore
)

func run(cmd *exec.Cmd) (string, error) {
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", data)
	}
	return string(data), err
}

func assertRun(t *testing.T, cmd *exec.Cmd) string {
	x, err := run(cmd)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func assertWrite(t *testing.T, cmd *exec.Cmd, r io.Reader) string {
	wc, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(wc, r)
	wc.Close()

	x, err := run(cmd)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestMain(m *testing.M) {
	var (
		exitFuncs []func()

		exitFunc = func(fn func()) {
			exitFuncs = append(exitFuncs, fn)
		}

		exit = func(code int) {
			for _, fn := range exitFuncs {
				fn()
			}
			os.Exit(code)
		}

		fatal = func(v ...interface{}) {
			log.Println(v...)
			exit(1)
		}
	)

	defer func() {
		if r := recover(); r != nil {
			fatal(r)
		}
	}()

	log.SetPrefix("testing: ")
	log.SetFlags(0)

	//
	cmdDir, err := ioutil.TempDir("", "testing")
	if err != nil {
		fatal(err)
	}
	exitFunc(func() {
		if err := os.RemoveAll(cmdDir); err != nil {
			log.Println(err)
		}
	})

	command = func(name string, arg ...string) *exec.Cmd {
		cmd := exec.Command(name, arg...)
		cmd.Dir = cmdDir
		return cmd
	}

	if _, err := run(command("git", "init")); err != nil {
		fatal(err)
	}

	store = DiskStore(Dir(cmdDir))

	exit(m.Run())
}

func TestWriter(t *testing.T) {
	data := []byte("hello,\nworld")

	w := store.Writer()
	w.WriteHeader(Blob, len(data))
	w.Write(data)
	w.Close()

	hash := w.Hash()

	out := assertRun(t, command("git", "cat-file", "-t", hash))
	out = strings.TrimSpace(out)
	if out != Blob.String() {
		t.Fatalf("git cat-file -t %s => %q, want %q", hash[:8], out, Blob)
	}

	out = assertRun(t, command("git", "cat-file", "-p", hash))
	out = strings.TrimSpace(out)
	if out != string(data) {
		t.Fatalf("git cat-file -p %s => %q, want %q", hash[:8], out, data)
	}
}

func TestReader(t *testing.T) {
	data := []byte("hello, world\n")

	cmd := command("git", "hash-object", "-t", "blob", "-w", "--stdin")
	hash := assertWrite(t, cmd, bytes.NewReader(data))
	hash = strings.TrimSpace(hash)

	r, err := store.Reader(hash)
	if err != nil {
		t.Fatal(err)
	}

	if r.Type() != Blob {
		t.Fatalf("Reader.Type() => %#v, want Blob", r.Type())
	}
	if r.Len() != len(data) {
		t.Fatalf("Reader.Len() => %v, want %v", r.Len(), len(data))
	}

	b := new(bytes.Buffer)
	io.Copy(b, r)
	r.Close()

	if !bytes.Equal(b.Bytes(), data) {
		t.Fatalf("Buffer.Bytes() => %q, want %q", string(b.Bytes()), string(data))
	}
}

func TestTree(t *testing.T) {
	// init
	dir := filepath.Join(string(store), "..")
	ioutil.WriteFile(filepath.Join(dir, "foo.txt"), []byte("foo"), 0644)
	os.MkdirAll(filepath.Join(dir, "foo", "bar"), 0755)
	ioutil.WriteFile(filepath.Join(dir, "foo", "bar", "bar.txt"), []byte("bar"), 0644)
	assertRun(t, command("git", "add", "-A", "."))
	assertRun(t, command("git", "commit", "-m", "foobar"))

	hash := strings.TrimSpace(assertRun(t, command("git", "rev-parse", "HEAD")))
	commit := strings.TrimSpace(assertRun(t, command("git", "cat-file", "-p", hash)))
	tree := strings.Split(strings.Split(commit, "\n")[0], " ")[1]

	want := assertRun(t, command("git", "cat-file", "-p", tree))

	// test reader
	r, err := store.Reader(tree, PrettyReader)
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	io.Copy(b, r)
	r.Close()

	if b.String() != want {
		t.Fatalf("Buffer.Bytes() => %q, want %q", b.String(), want)
	}

	// test writer
	tmp, err := ioutil.TempFile("", "gittest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	w := NewWriter(tmp)
	if _, err := w.WriteHeader(Tree, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b.Bytes()); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	w.Close()

	orig, err := command("git", "cat-file", "tree", tree).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	tmp.Seek(0, 0)
	raw, err := NewReader(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Type() != Tree {
		t.Fatal("expect tree")
	}
	if raw.Len() != 65 {
		t.Fatal("expect 65", raw.Len())
	}
	dat := new(bytes.Buffer)
	io.Copy(dat, raw)
	raw.Close()
	tmp.Close()

	if !bytes.Equal(dat.Bytes(), orig) {
		t.Fatalf("bytes.Equal have %v want %v", dat.Bytes(), orig)
	}

	if w.Hash() != tree {
		t.Fatalf("Writer.Hash() => %q, want %q", w.Hash(), tree)
	}
}
