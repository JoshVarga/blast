package blast_test

import (
	"bytes"
	"fmt"
	"github.com/JoshVarga/blast"
	"io"
	"os"
)

func ExampleNewWriter() {
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	w.Write([]byte("AIAIAIAIAIAIA"))
	w.Close()
	fmt.Println(b.Bytes())
	// Output: [0 4 130 36 37 143 128 127]
}

func ExampleNewReader() {
	buff := []byte{0, 4, 130, 36, 37, 143, 128, 127}
	b := bytes.NewReader(buff)
	r, err := blast.NewReader(b)
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, r)
	// Output: AIAIAIAIAIAIA
	r.Close()
}
