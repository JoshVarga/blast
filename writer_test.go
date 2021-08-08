package blast_test

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/JoshVarga/blast"
)

func TestSimpleCompress(t *testing.T) {
	expected := []byte{0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	_, err := w.Write([]byte("AIAIAIAIAIAIA"))
	if err != nil {
		t.Errorf("failed to write: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Errorf("failed to close: %v", err)
	}

	if !bytes.Equal(b.Bytes(), expected) {
		t.Errorf("found=%v : expected=%v", b, expected)
	}
}

func TestCompressDecompress(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	data := randomBytes(1000, 20)
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	_, err := w.Write(data)
	if err != nil {
		t.Errorf("error writing %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Errorf("error writing %v", err)
	}

	reader := bytes.NewBuffer(b.Bytes())
	blastReader, err := blast.NewReader(reader)
	if err != nil {
		t.Errorf("error reading %v", err)
	}
	decoded, err := ioutil.ReadAll(blastReader)
	if err != nil {
		t.Errorf("error decoding %v", err)
	}

	if !bytes.Equal(decoded, data) {
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
