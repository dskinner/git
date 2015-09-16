# git [![GoDoc](https://godoc.org/dasa.cc/git?status.svg)](https://godoc.org/dasa.cc/git)

Package git provides an incomplete pure Go implementation of Git core methods.

## Example

### Code:

```go
store := git.TempStore()
defer os.RemoveAll(string(store))

buf := new(bytes.Buffer)

// blob
bdata := []byte("hello, world")

bw := store.Writer()
bw.WriteHeader(git.Blob, len(bdata))
bw.Write(bdata)
bw.Close()

br, _ := store.Reader(bw.Hash())
io.Copy(buf, br)
br.Close()

buf.WriteRune('\n')

// tree
tdata := []byte(fmt.Sprintf("100644 blob %s\t%s\n", bw.Hash(), "hello.txt"))

tw := store.Writer()
tw.WriteHeader(git.Tree, -1)
tw.Write(tdata)
tw.Close()

tr, _ := store.Reader(tw.Hash(), git.PrettyReader)
io.Copy(buf, tr)
tr.Close()

fmt.Println(strings.Replace(buf.String(), "\t", " ", -1))
```

### Output:

```
hello, world
100644 blob 8c01d89ae06311834ee4b1fab2f0414d35f01102 hello.txt
```

## Caveats

* Currently limited to dangling objects
* Reader and Writer for tree objects will likely fail on short reads and large content. Straight-forward to fix.
