package blast_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/JoshVarga/blast"
)

func ExampleNewWriter() {
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	count, err := w.Write([]byte("AIAIAIAIAIAIA"))
	if err != nil {
		fmt.Print("failed to write")
	}
	if count != 13 {
		fmt.Printf("incorrect number of bytes written: %v", count)
	}
	err = w.Close()
	fmt.Print(b.Bytes())
	if err != nil {
	}
	// Output: [0 4 130 36 37 143 128 127]
}

func ExampleNewReader() {
	buff := []byte{0, 4, 130, 36, 37, 143, 128, 127}
	b := bytes.NewReader(buff)
	r, err := blast.NewReader(b)
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(os.Stdout, r)
	// Output: AIAIAIAIAIAIA
	if err != nil {
	}
	err = r.Close()
	if err != nil {
	}
}
