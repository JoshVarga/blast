# blast

Golang library for decompressing data in the PKWare Data Compression Library (DCL) compressed format,
otherwise known as "explode" for "imploded" data.

Based on blast.c in ZLIB written by Mark Adler

### Installation

	go get github.com/JoshVarga/blast

### Features

    Decompress ("explode") data that has been compressed using PKZIP "implode" method

### Example

```
func ExampleDecompress() {
	fileIn, err := os.Open("in.bin")
        ...
	blastReader, err = blast.NewReader(fileIn)
	...
	decoded, err := ioutil.ReadAll(blastReader)
	...
	err = ioutil.WriteFile("out.bin", decoded, 0777)
	fileIn.Close()
}
```
### License

Copyright (c) 2018 Josh Varga

Original C version: Copyright (C) 2003, 2012, 2013 Mark Adler,
version 1.3, 24 Aug 2013

This software is provided 'as-is', without any express or implied
warranty. In no event will the authors be held liable for any damages
arising from the use of this software.

Permission is granted to anyone to use this software for any purpose,
including commercial applications, and to alter it and redistribute it
freely, subject to the following restrictions:

1. The origin of this software must not be misrepresented; you must not
   claim that you wrote the original software. If you use this software
   in a product, an acknowledgment in the product documentation would be
   appreciated but is not required.
2. Altered source versions must be plainly marked as such, and must not be
   misrepresented as being the original software.
3. This notice may not be removed or altered from any source distribution.
