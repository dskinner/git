package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"dasa.cc/git"
)

type HashObject struct {
	fset *flag.FlagSet

	flagStdin *bool
	flagWrite *bool
	flagType  *string
}

func NewHashObject(args []string) Runner {
	r := &HashObject{}
	r.fset = flag.NewFlagSet("hash-object", flag.ContinueOnError)
	r.flagStdin = r.fset.Bool("stdin", false, "read from stdin")
	r.flagWrite = r.fset.Bool("w", false, "write object")
	r.flagType = r.fset.String("t", "blob", "object type")
	r.fset.Parse(args)
	return r
}

func (cmd *HashObject) Run() {
	log.SetPrefix("ggit hash-object: ")

	name := cmd.fset.Arg(0)
	if name == "" && !*cmd.flagStdin {
		log.Fatal("nothing to hash")
	}

	check := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}

	var (
		w git.Writer
		n int
		r io.Reader
	)

	if *cmd.flagWrite {
		w = store.Writer()
	} else {
		tmp, err := ioutil.TempFile("", "ggithashobject")
		defer os.Remove(tmp.Name())
		check(err)
		w = git.NewWriter(tmp)
	}

	if *cmd.flagStdin {
		tmp, err := ioutil.TempFile("", "ggithashobjectstdin")
		check(err)
		defer os.Remove(tmp.Name())
		_, err = io.Copy(tmp, os.Stdin)
		check(err)
		name = tmp.Name()
		tmp.Close()
	}

	if name != "" {
		f, err := os.Open(name)
		check(err)
		fi, err := f.Stat()
		check(err)
		n = int(fi.Size())
		r = f
	}

	_, err := w.WriteHeader(git.ParseType([]byte(*cmd.flagType)), n)
	check(err)
	_, err = io.Copy(w, r)
	check(err)
	check(w.Close())
	fmt.Println(w.Hash())
}
