// ggit provides a cli for dasa.cc/git functionality.
// This is not a suitable replacement for official git; useful for discovery.
package main

import (
	"log"
	"os"

	"dasa.cc/git"
)

var store git.Store

type Runner interface {
	Run()
}

var commands = map[string]func([]string) Runner{
	"cat-file":    NewCatFile,
	"hash-object": NewHashObject,
}

func main() {
	log.SetPrefix("ggit: ")
	log.SetFlags(0)

	defer func() {
		if r := recover(); r != nil {
			log.Fatal(r)
		}
	}()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Get working directory: %s", err)
	}
	store = git.DiskStore(git.Dir(wd))

	if len(os.Args) == 1 {
		log.Fatal("no arguments")
	}

	cmd, ok := commands[os.Args[1]]
	if !ok {
		log.Fatalf("command %q not found", os.Args[1])
	}
	cmd(os.Args[2:]).Run()
}
