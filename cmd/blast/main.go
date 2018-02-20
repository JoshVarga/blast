package main

import (
	"flag"
	"github.com/JoshVarga/blast"
	"io"
	"io/ioutil"
	"log"
	"os"
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
	var blastReader io.ReadCloser
	blastReader, err = blast.NewReader(fileIn)
	if err != nil {
		log.Fatal(err)
	}
	decoded, err := ioutil.ReadAll(blastReader)
	if err != nil {
		log.Fatal(err)
	}
	fileIn.Close()
	err = ioutil.WriteFile(*outputFile, decoded, 0777)
	if err != nil {
		log.Fatal(err)
	}
}
