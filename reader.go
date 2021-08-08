/*
Package blast implements reading of PKWare Data Compression Library (DCL) compressed data,
otherwise known as "explode" for "imploded" data

The implementation provides functionality that decompresses during reading.

For example, to read compressed data from a buffer:

	r, err := blast.NewReader(&b)
	io.Copy(os.Stdout, r)
	r.Close()
*/
package blast

import (
	"bytes"
	"errors"
	"io"
)

/*
 * Copyright (c) 2018 Josh Varga
 * Original C version: Copyright (C) 2003, 2012, 2013 Mark Adler
 * version 1.3, 24 Aug 2013
 *
 * This software is provided 'as-is', without any express or implied
 * warranty. In no event will the authors be held liable for any damages
 * arising from the use of this software.
 *
 * Permission is granted to anyone to use this software for any purpose,
 * including commercial applications, and to alter it and redistribute it
 * freely, subject to the following restrictions:
 *
 * 1. The origin of this software must not be misrepresented; you must not
 *    claim that you wrote the original software. If you use this software
 *    in a product, an acknowledgment in the product documentation would be
 *    appreciated but is not required.
 * 2. Altered source versions must be plainly marked as such, and must not be
 *    misrepresented as being the original software.
 * 3. This notice may not be removed or altered from any source distribution.
 *
 * This code has been adapted to Go from Mark Adlers blast.c in ZLIB,
 * most of the comments are from the original source.
 *
 * blast() decompresses the PKWare Data Compression Library (DCL) compressed
 * format.  It provides the same functionality as the explode() function in
 * that library.  (Note: PKWare overused the "implode" verb, and the format
 * used by their library implode() function is completely different and
 * incompatible with the implode compression method supported by PKZIP.)

 * This decompressor is based on the excellent format description provided by
 * Ben Rudiak-Gould in comp.compression on August 13, 2001.  Interestingly, the
 * example Ben provided in the post is incorrect.  The distance 110001 should
 * instead be 111000.  When corrected, the example byte stream becomes:
 *
 *    00 04 82 24 25 8f 80 7f
 *
 * which decompresses to "AIAIAIAIAIAIA" (without the quotes).
 */

var (
	// ErrHeader is returned when reading data that has an invalid header.
	ErrHeader = errors.New("blast: invalid header")
	// ErrDictionary is returned when reading data that has an invalid dictionary.
	ErrDictionary = errors.New("blast: invalid dictionary")
	// ErrDistanceTooFar is returned when reading data that has an invalid repetition length.
	ErrDistanceTooFar = errors.New("distance is too far back")
	// ErrUnexpectedEOF means that EOF was encountered
	ErrUnexpectedEOF = errors.New("unexpected EOF")
)

const (
	maxBits       = 13   // maximum code length
	maxWindowSize = 4096 // maximum window size
)

// input and output state
type state struct {
	// input state
	reader  io.Reader // input function provided by user
	left    int       // available input at in
	in      []byte
	inIndex int
	bitbuf  int  // bit buffer
	bitcnt  uint // number of bits in bit buffer

	// output state
	writer io.Writer           // output function provided by user
	next   uint                // index of next write location in out[]
	first  bool                // true to check distances (for first 4K)
	out    [maxWindowSize]byte // output buffer and sliding window
}

/*
 * Return need bits from the input stream.  This always leaves less than
 * eight bits in the buffer.  bits() works properly for need == 0.
 *
 * Format notes:
 *
 * - Bits are stored in bytes from the least significant bit to the most
 *   significant bit.  Therefore bits are dropped from the bottom of the bit
 *   buffer, using shift right, and new bytes are appended to the top of the
 *   bit buffer, using shift left.
 */
func bits(s *state, need uint) (int, error) {
	var val int // bit accumulator
	var err error
	// load at least need bits into val
	val = s.bitbuf
	for s.bitcnt < need {
		if s.left == 0 {
			s.left, err = s.reader.Read(s.in)
			s.inIndex = 0
			if err != nil {
				return 0, err
			}
			if s.left == 0 {
				return 0, ErrUnexpectedEOF
			}
		}
		val |= int(uint(s.in[s.inIndex]) << s.bitcnt) // load eight bits
		s.inIndex++
		s.left--
		s.bitcnt += 8
	}

	// drop need bits and update buffer, always zero to seven bits left
	s.bitbuf = val >> need
	s.bitcnt -= need

	// return need bits, zeroing the bits above that
	return val & ((1 << need) - 1), nil
}

/*
 * Huffman code decoding tables.  count[1..maxBits] is the number of symbols of
 * each length, which for a canonical code are stepped through in order.
 * symbol[] are the symbol values in canonical order, where the number of
 * entries is the sum of the counts in count[].  The decoding process can be
 * seen in the function decode() below.
 */
type huffman struct {
	count  []int16 // number of symbols of each length
	symbol []int16 // canonically ordered symbols
}

/*
 * Decode a code from the stream s using huffman table h.  Return the symbol or
 * a negative value if there is an error.  If all of the lengths are zero, i.e.
 * an empty code, or if the code is incomplete and an invalid code is received,
 * then -9 is returned after reading maxBits bits.
 *
 * Format notes:
 *
 * - The codes as stored in the compressed data are bit-reversed relative to
 *   a simple integer ordering of codes of the same lengths.  Hence below the
 *   bits are pulled from the compressed data one at a time and used to
 *   build the code value reversed from what is in the stream in order to
 *   permit simple integer comparisons for decoding.
 *
 * - The first code for the shortest length is all ones.  Subsequent codes of
 *   the same length are simply integer decrements of the previous code.  When
 *   moving up a length, a one bit is appended to the code.  For a complete
 *   code, the last code of the longest length will be all zeros.  To support
 *   this ordering, the bits pulled during decoding are inverted to apply the
 *   more "natural" ordering starting with all zeros and incrementing.
 */
func decode(s *state, h *huffman) (int16, error) {
	length := uint(1)     // current number of bits in code
	code := 0             // length bits being decoded
	first := 0            // first code of length length
	var count int         // number of codes of length length
	index := 0            // index of first code of length length in symbol table
	bitBuffer := s.bitbuf // bits from stream
	left := s.bitcnt      // bits left in next or left to process
	nextIndex := 1
	for {
		for ; left != 0; left-- {
			code |= (bitBuffer & 1) ^ 1 // invert code
			bitBuffer >>= 1
			count = int(h.count[nextIndex])
			nextIndex++
			if code < first+count { // if length length, return symbol
				s.bitbuf = bitBuffer
				s.bitcnt = (s.bitcnt - length) & 7
				return h.symbol[index+(code-first)], nil
			}
			index += count // else update for next length
			first += count
			first <<= 1
			code <<= 1
			length++
		}
		left = (maxBits + 1) - length
		if left == 0 {
			break
		}
		if s.left == 0 {
			var err error
			s.left, err = s.reader.Read(s.in)
			s.inIndex = 0
			if err != nil {
				return -1, err
			}
			if s.left == 0 {
				return 0, ErrUnexpectedEOF
			}
		}
		bitBuffer = int(s.in[s.inIndex])
		s.inIndex++
		s.left--
		if left > 8 {
			left = 8
		}
	}
	return -9, nil // ran out of codes
}

/*
 * Given a list of repeated code lengths rep[0..n-1], where each byte is a
 * count (high four bits + 1) and a code length (low four bits), generate the
 * list of code lengths.  This compaction reduces the size of the object code.
 * Then given the list of code lengths length[0..n-1] representing a canonical
 * Huffman code for n symbols, construct the tables required to decode those
 * codes.  Those tables are the number of codes of each length, and the symbols
 * sorted by length, retaining their original order within each length.  The
 * return value is zero for a complete code set, negative for an over-
 * subscribed code set, and positive for an incomplete code set.  The tables
 * can be used if the return value is zero or positive, but they cannot be used
 * if the return value is negative.  If the return value is zero, it is not
 * possible for decode() using that table to return an error--any stream of
 * enough bits will resolve to a symbol.  If the return value is positive, then
 * it is possible for decode() using that table to return an error for received
 * codes past the end of the incomplete lengths.
 */
func construct(h *huffman, rep []byte) int {
	var n = len(rep)
	var symbol int              // current symbol when stepping through length[]
	var hCountLength int        // current length when stepping through h.count[]
	var left int                // number of possible codes left of current length
	var offs [maxBits + 1]int16 // offsets in symbol table for each length
	var length [256]int16       // code lengths

	// convert compact repeat counts into symbol bit length list
	symbol = 0
	i := 0
	for ; n != 0; n-- {
		hCountLength = int(rep[i])
		i++
		left = (hCountLength >> 4) + 1
		hCountLength &= 15
		for ; left != 0; left-- {
			length[symbol] = int16(hCountLength)
			symbol++
		}
	}
	n = symbol

	// count number of codes of each length
	for hCountLength = 0; hCountLength <= maxBits; hCountLength++ {
		h.count[hCountLength] = 0
	}
	for symbol = 0; symbol < n; symbol++ {
		h.count[length[symbol]]++ // assumes lengths are within bounds
	}
	if h.count[0] == int16(n) { // no codes!
		return 0 // complete, but decode() will fail
	}
	// check for an over-subscribed or incomplete set of lengths
	left = 1 // one possible code of zero length
	for hCountLength = 1; hCountLength <= maxBits; hCountLength++ {
		left <<= 1                         // one more bit, double codes left
		left -= int(h.count[hCountLength]) // deduct count from possible codes
		if left < 0 {
			return left // over-subscribed--return negative
		}
	} // left > 0 means incomplete

	// generate offsets into symbol table for each length for sorting
	offs[1] = 0
	for hCountLength = 1; hCountLength < maxBits; hCountLength++ {
		offs[hCountLength+1] = offs[hCountLength] + h.count[hCountLength]
	}
	// put symbols in table sorted by length, by symbol order within each length
	for symbol = 0; symbol < n; symbol++ {
		if length[symbol] != 0 {
			h.symbol[offs[length[symbol]]] = int16(symbol)
			offs[length[symbol]]++
		}
	}
	// return zero for complete set, positive for incomplete set
	return left
}

/*
 * Decode PKWare Compression Library stream.
 *
 * Format notes:
 *
 * - First byte is 0 if literals are uncoded or 1 if they are coded.  Second
 *   byte is 4, 5, or 6 for the number of extra bits in the distance code.
 *   This is the base-2 logarithm of the dictionary size minus six.
 *
 * - Compressed data is a combination of literals and length/distance pairs
 *   terminated by an end code.  Literals are either Huffman coded or
 *   uncoded bytes.  A length/distance pair is a coded length followed by a
 *   coded distance to represent a string that occurs earlier in the
 *   uncompressed data that occurs again at the current location.
 *
 * - A bit preceding a literal or length/distance pair indicates which comes
 *   next, 0 for literals, 1 for length/distance.
 *
 * - If literals are uncoded, then the next eight bits are the literal, in the
 *   normal bit order in the stream, i.e. no bit-reversal is needed. Similarly,
 *   no bit reversal is needed for either the length extra bits or the distance
 *   extra bits.
 *
 * - Literal bytes are simply written to the output.  A length/distance pair is
 *   an instruction to copy previously uncompressed bytes to the output.  The
 *   copy is from distance bytes back in the output stream, copying for length
 *   bytes.
 *
 * - Distances pointing before the beginning of the output data are not
 *   permitted.
 *
 * - Overlapped copies, where the length is greater than the distance, are
 *   allowed and common.  For example, a distance of one and a length of 518
 *   simply copies the last byte 518 times.  A distance of four and a length of
 *   twelve copies the last four bytes three times.  A simple forward copy
 *   ignoring whether the length is greater than the distance or not implements
 *   this correctly.
 */
func decompress(s *state) error {
	var lit int        // true if literals are coded
	var dict int       // log2(dictionary size) - 6
	var symbol int16   // decoded symbol, extra bits for distance
	var copyLength int // length for copy
	var dist uint      // distance for copy

	var copy int                                                         // copy counter
	literalCode := huffman{make([]int16, maxBits+1), make([]int16, 256)} // length code
	lengthCode := huffman{make([]int16, maxBits+1), make([]int16, 16)}   // length code
	distanceCode := huffman{make([]int16, maxBits+1), make([]int16, 64)} // distance code
	// bit lengths of literal codes
	var literalBitLength = []byte{
		11, 124, 8, 7, 28, 7, 188, 13, 76, 4, 10, 8, 12, 10, 12, 10, 8, 23, 8,
		9, 7, 6, 7, 8, 7, 6, 55, 8, 23, 24, 12, 11, 7, 9, 11, 12, 6, 7, 22, 5,
		7, 24, 6, 11, 9, 6, 7, 22, 7, 11, 38, 7, 9, 8, 25, 11, 8, 11, 9, 12,
		8, 12, 5, 38, 5, 38, 5, 11, 7, 5, 6, 21, 6, 10, 53, 8, 7, 24, 10, 27,
		44, 253, 253, 253, 252, 252, 252, 13, 12, 45, 12, 45, 12, 61, 12, 45,
		44, 173}
	// bit lengths of length codes 0..15
	var lengthBitLength = []byte{2, 35, 36, 53, 38, 23}
	// bit lengths of distance codes 0..63
	var distanceBitLength = []byte{2, 20, 53, 230, 247, 151, 248}
	var base = []int16{ // base for length codes
		3, 2, 4, 5, 6, 7, 8, 9, 10, 12, 16, 24, 40, 72, 136, 264}
	var extra = []int8{ // extra bits for length codes
		0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}

	// set up decoding tables (once--might not be thread-safe)
	construct(&literalCode, literalBitLength)
	construct(&lengthCode, lengthBitLength)
	construct(&distanceCode, distanceBitLength)
	var err error
	// read header
	lit, err = bits(s, 8)
	if err != nil {
		return err
	}
	if lit > 1 {
		return ErrHeader
	}
	dict, err = bits(s, 8)
	if err != nil {
		return err
	}
	if dict < 4 || dict > 6 {
		return ErrDictionary
	}
	// decode literals and length/distance pairs
	for {
		bitVal, err := bits(s, 1)
		if err != nil {
			return err
		}
		if bitVal != 0 {
			// get length
			symbol, err = decode(s, &lengthCode)
			if err != nil {
				return err
			}
			bitVal, err = bits(s, uint(extra[symbol]))
			if err != nil {
				return err
			}
			copyLength = int(base[symbol]) + bitVal
			if copyLength == 519 {
				break // end code
			}
			// get distance
			if copyLength == 2 {
				symbol = 2
			} else {
				symbol = int16(dict)
			}
			var decodeVal int16
			decodeVal, err = decode(s, &distanceCode)
			if err != nil {
				return err
			}

			dist = uint(decodeVal) << uint(symbol)
			bitVal, err = bits(s, uint(symbol))
			if err != nil {
				return err
			}
			dist += uint(bitVal)
			dist++
			if s.first && dist > s.next {
				return ErrDistanceTooFar // distance too far back
			}
			// copy length bytes from distance bytes back
			for ok := true; ok; ok = copyLength != 0 {
				to := s.next
				from := s.next - dist
				copy = maxWindowSize
				if s.next < dist {
					from += uint(copy)
					copy = int(dist)
				}
				copy -= int(s.next)
				if copy > copyLength {
					copy = copyLength
				}
				copyLength -= copy
				s.next += uint(copy)
				for ; copy != 0; copy-- {
					s.out[to] = s.out[from]
					to++
					from++
				}
				if s.next == maxWindowSize {
					_, err := s.writer.Write(s.out[:s.next])
					if err != nil {
						return err
					}
					s.next = 0
					s.first = false
				}
			}
		} else {
			// get literal and write it
			if lit != 0 {
				symbol, err = decode(s, &literalCode)
				if err != nil {
					return err
				}
			} else {
				var bitsVal int
				bitsVal, err = bits(s, 8)
				symbol = int16(bitsVal)
				if err != nil {
					return err
				}
			}
			s.out[s.next] = byte(symbol)
			s.next++
			if s.next == maxWindowSize {
				_, err := s.writer.Write(s.out[:s.next])
				if err != nil {
					return err
				}
				s.next = 0
				s.first = false
			}
		}
	}
	return nil
}

func blast(r io.Reader, w io.Writer, left *uint) error {
	var s state // input/output state
	// initialize input state
	s.reader = r
	if left != nil && *left != 0 {
		s.left = int(*left)
	} else {
		s.left = 0
	}
	s.bitbuf = 0
	s.bitcnt = 0
	s.in = make([]byte, 16384)
	// initialize output state
	s.writer = w
	s.next = 0
	s.first = true
	err := decompress(&s)
	if err != nil {
		return err
	}
	// return unused input
	if left != nil {
		*left = uint(s.left)
	}
	// write any leftover output and update the error code if needed
	if s.next != 0 {
		_, err = s.writer.Write(s.out[:s.next])
		if err != nil {
			return err
		}
	}
	return nil
}

type reader struct {
	data      []byte
	readIndex int64
}

// NewReader creates a new ReadCloser.
// Reads from the returned ReadCloser read and decompress data from r.
// It is the caller's responsibility to call Close on the ReadCloser when done.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	var writer bytes.Buffer
	err := blast(r, &writer, nil)
	if err != nil {
		return nil, err
	}
	blastReader := new(reader)
	blastReader.data = writer.Bytes()
	blastReader.readIndex = 0
	return blastReader, nil
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.readIndex >= int64(len(r.data)) {
		err = io.EOF
		return
	}
	n = copy(p, r.data[r.readIndex:])
	r.readIndex += int64(n)
	return
}

func (r *reader) Close() error {
	return nil
}
