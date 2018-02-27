# blast

Golang library for both compressing and decompressing data in the PKWare Data Compression Library (DCL) compressed format,
otherwise known as "explode" and "implode" which differ from PKZIP.

Based on:

blast.c in ZLIB written by Mark Adler
https://github.com/madler/zlib/blob/master/contrib/blast/blast.c

implode.c Ladislav Zezula 2003
https://github.com/ladislav-zezula/StormLib/blob/master/src/pklib/implode.c


### Installation

	go get github.com/JoshVarga/blast

### Features
    Compress ("implode") data using the PKWARE DCL "implode" method
    Decompress ("explode") data that has been compressed using PKWARE DCL "implode" method

### Example

```
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
```
### License

Copyright (c) 2018 Josh Varga

Explode:
Original C version: Copyright (C) 2003, 2012, 2013 Mark Adler,
version 1.3, 24 Aug 2013

Implode:
Original C version: Copyright (c) Ladislav Zezula 2003

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
