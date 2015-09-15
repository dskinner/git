package git

import "testing"

var header []byte

func BenchmarkTypeHeader(b *testing.B) {
	for n := 0; n < b.N; n++ {
		header = Blob.Header(20048)
	}
}
