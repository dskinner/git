// Package git provides an incomplete pure Go implementation of Git core methods.
package git // import "dasa.cc/git"

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Init initializes a new git repository at the given path. Not recommended for use.
// Panics on error. Panics if path for git directory is not empty.
func Init(path string, bare bool) {
	check := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	isEmpty := func(x string) bool {
		f, err := os.Open(x)
		check(err)
		defer f.Close()
		_, err = f.Readdirnames(1)
		return err == io.EOF
	}

	if !bare {
		path = filepath.Join(path, ".git")
		check(os.MkdirAll(path, 0755))
	}

	if !isEmpty(path) {
		panic(fmt.Sprintf("directory not empty: %s", path))
	}

	check(os.MkdirAll(filepath.Join(path, "branches"), 0755))

	check(os.MkdirAll(filepath.Join(path, "hooks"), 0755))

	check(os.MkdirAll(filepath.Join(path, "info"), 0755))
	check(ioutil.WriteFile(filepath.Join(path, "info", "exclue"), []byte{}, 0644))

	check(os.MkdirAll(filepath.Join(path, "objects", "info"), 0755))
	check(os.MkdirAll(filepath.Join(path, "objects", "pack"), 0755))

	check(os.MkdirAll(filepath.Join(path, "refs", "heads"), 0755))
	check(os.MkdirAll(filepath.Join(path, "refs", "tags"), 0755))

	check(ioutil.WriteFile(filepath.Join(path, "HEAD"), []byte("ref: refs/heads/master"), 0644))

	config := []byte("[config]\n\trepositoryformatversion = 0\n\tfilemode = true")
	if bare {
		config = append(config, []byte("\n\tbare = true")...)
	}
	check(ioutil.WriteFile(filepath.Join(path, "config"), config, 0644))

	desc := []byte("Unnamed repository; edit this file 'description' to name the repository.")
	check(ioutil.WriteFile(filepath.Join(path, "description"), desc, 0644))
}
