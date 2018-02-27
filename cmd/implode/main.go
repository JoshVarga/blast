package main

import (
	"flag"
	"github.com/JoshVarga/blast"
	"io/ioutil"
	"log"
	"os"
	"bytes"
)

func main() {
	inputFile := flag.String("i", "", "input file")
	outputFile := flag.String("o", "", "output file")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}
	fileIn, err := os.Open(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	var b bytes.Buffer
	w := blast.NewWriter(&b, blast.Binary, blast.DictionarySize1024)
	decoded, err := ioutil.ReadAll(fileIn)
	if err != nil {
		log.Fatal(err)
	}
	fileIn.Close()
	_, err = w.Write(decoded)
	if err != nil {
		log.Fatal(err)
	}
	w.Close()
	err = ioutil.WriteFile(*outputFile, b.Bytes(), 0777)
	if err != nil {
		log.Fatal(err)
	}
}
