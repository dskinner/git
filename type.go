package git

import (
	"bytes"
	"fmt"
)

// Type represents a git object type.
type Type int

func (t Type) String() string {
	switch t {
	case Blob:
		return "blob"
	case Tree:
		return "tree"
	case Commit:
		return "commit"
	}
	panic(fmt.Sprintf("missing type: %#v", t))
}

// Header returns nul terminated header string for git object.
func (t Type) Header(length int) []byte {
	return []byte(fmt.Sprintf("%s %v\x00", t, length))
}

// ParseType parses object type from bytes.
func ParseType(q []byte) Type {
	if bytes.Equal(q, []byte("blob")) {
		return Blob
	}
	if bytes.Equal(q, []byte("tree")) {
		return Tree
	}
	if bytes.Equal(q, []byte("commit")) {
		return Commit
	}
	panic(fmt.Sprintf("missing type: %q", q))
}

// Git Object Types
const (
	Blob Type = iota
	Tree
	Commit
)
