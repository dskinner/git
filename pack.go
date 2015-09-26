package git

import "io"

// PackStore implements Store for packfiles in git repositories.
func PackStore() Store { return &packStore{} }

type packStore struct{}

func (st *packStore) Object(hash string) (io.Reader, error) {
	panic("Not implemented")
}

func (st *packStore) Reader(hash string, options ...func(*Reader)) (*Reader, error) {
	panic("Not implemented")
}

func (st *packStore) Writer() Writer {
	panic("Not implemented")
}

type packCloser struct{}

func (g *packCloser) Close() error {
	panic("Not implemented")
}
