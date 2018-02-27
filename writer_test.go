package blast_test

import (
	"bytes"
	"github.com/JoshVarga/blast"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"
)

func TestSimpleCompress(t *testing.T) {
	expected := []byte{0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	w.Write([]byte("AIAIAIAIAIAIA"))
	w.Close()
	if bytes.Compare(b.Bytes(), expected) != 0 {
		t.Errorf("found=%v : expected=%v", b, expected)
	}
}

func TestCompressDecompress(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	data := randomBytes(1000, 20)
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	w.Write(data)
	w.Close()

	reader := bytes.NewBuffer(b.Bytes())
	blastReader, _ := blast.NewReader(reader)
	decoded, _ := ioutil.ReadAll(blastReader)

	if bytes.Compare(decoded, data) != 0 {
		t.Errorf("found=%v\nexpected=%v", decoded, data)
	}
}

func randomBytes(length, unique int) []uint8 {
	b := make([]uint8, length)
	for i := range b {
		b[i] = uint8(rand.Intn(unique))
	}
	return b
}
