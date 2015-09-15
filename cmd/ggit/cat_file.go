package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"dasa.cc/git"
)

type CatFile struct {
	fset *flag.FlagSet

	flagType  *bool
	flagSize  *bool
	flagPrint *bool
}

func NewCatFile(args []string) Runner {
	r := &CatFile{}
	r.fset = flag.NewFlagSet("cat-file", flag.ContinueOnError)
	r.flagType = r.fset.Bool("t", false, "display object type")
	r.flagSize = r.fset.Bool("s", false, "display object size")
	r.flagPrint = r.fset.Bool("p", false, "display object content")
	r.fset.Parse(args)
	return r
}

func (cmd *CatFile) Run() {
	log.SetPrefix("ggit cat-file: ")
	hash := cmd.fset.Arg(0)
	if hash == "" {
		log.Fatal("no hash given")
	}
	r, err := store.Reader(hash, git.PrettyReader)
	if err != nil {
		log.Fatalf("Reader(%s): %s", hash, err)
	}
	if *cmd.flagType {
		fmt.Println(r.Type())
	}
	if *cmd.flagSize {
		fmt.Println(r.Len())
	}
	if *cmd.flagPrint {
		if _, err := io.Copy(os.Stdout, r); err != nil {
			log.Fatalf("Write stdout: %s", err)
		}
	}
}
