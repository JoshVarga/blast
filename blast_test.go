package blast_test

import (
	"bytes"
	"github.com/JoshVarga/blast"
	"io/ioutil"
	"testing"
)

func TestSimpleCase(t *testing.T) {
	var testInput = []byte{
		0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}

	expected := "AIAIAIAIAIAIA"
	reader := bytes.NewBuffer(testInput)
	blastReader, _ := blast.NewReader(reader)
	decoded, err := ioutil.ReadAll(blastReader)
	if err != nil {
		t.Errorf("%v", err)
	}
	if string(decoded) != expected {
		t.Errorf("found=%v : expected=%v", string(decoded), expected)
	}
}

func TestInvalidHeader(t *testing.T) {
	var testInput = []byte{0x02, 0x04, 0x82}
	reader := bytes.NewBuffer(testInput)
	_, err := blast.NewReader(reader)
	if err != blast.ErrHeader {
		t.Error("failed to reject invalid header")
	}
}

func TestInvalidDictionary(t *testing.T) {
	var testInput = []byte{0x00, 0x03, 0x82}
	reader := bytes.NewBuffer(testInput)
	_, err := blast.NewReader(reader)
	if err != blast.ErrDictionary {
		t.Error("failed to reject invalid dictionary")
	}
}
