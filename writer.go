package blast

import (
	"bytes"
	"io"
	"errors"
)

/*
 * Copyright (c) 2018 Josh Varga
 * Original C version: Copyright (c) Ladislav Zezula 2003
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
 * This code has been adapted to Go from Ladislav Zezula implode.c found in:
 * https://github.com/ladislav-zezula/StormLib/blob/master/src/pklib/implode.c
 * most of the comments are from the original source.
 *
 * Implode function of PKWARE Data Compression library
 */

const (
	// Binary represents the Binary compression mode
	Binary = 0
	// ASCII represents the ASCII compression mode
	ASCII  = 1

	// DictionarySize1024 represents a dictiony of size 1024 is used
	DictionarySize1024 = 1024
	// DictionarySize2048 represents a dictiony of size 2048 is used
	DictionarySize2048 = 2048
	// DictionarySize4096 represents a dictiony of size 4096 is used
	DictionarySize4096 = 4096
)

type tCmpStruct struct {
	distance    uint         // 0000: Backward distance of the currently found repetition, decreased by 1
	outBytes   uint          // 0004: # bytes available in outBuff
	outBits    uint          // 0008: # of bits available in the last out byte
	dsizeBits  uint          // 000C: Number of bits needed for dictionary size. 4 = 0x400, 5 = 0x800, 6 = 0x1000
	dsizeMask  uint          // 0010: Bit mask for dictionary. 0x0F = 0x400, 0x1F = 0x800, 0x3F = 0x1000
	cType      uint          // 0014: Compression type (ASCII or Binary)
	dsizeBytes uint          // 0018: Dictionary size in bytes
	distBits   []uint8       // 001C: Distance bits
	distCodes  []uint8       // 005C: Distance codes
	nChBits    [0x306]uint8  // 009C: Table of literal bit lengths to be put to the output stream
	nChCodes   [0x306]uint16 // 03A2: Table of literal codes to be put to the output stream
	offs09AE   uint16        // 09AE:

	//param     *uint8    // 09B0: User parameter
	readBuf  io.Reader // 9B4
	writeBuf io.Writer // 9B8

	offs09BC        [0x204]uint16 // 09BC:
	offs0DC4        uint32        // 0DC4:
	phashToIndex    [0x900]uint16 // 0DC8: Array of indexes (one for each PAIR_HASH) to the "pair_hash_offsets" table
	phashToIndexEnd uint16        // 1FC8: End marker for "phashToIndex" table
	outBuff         []uint8       // 1FCA: Compressed data
	workBuff        []uint8       // 27CC: Work buffer
	//  + DICT_OFFSET  => Dictionary
	//  + UNCMP_OFFSET => Uncompressed data
	phashOffs [0x2204]uint16 // 49D0: Table of offsets for each PAIR_HASH
}

func newTCmpStruct() *tCmpStruct {
	result := new(tCmpStruct)
	result.workBuff = make([]uint8, 0x2204)
	result.outBuff = make([]uint8, 0x802)
	result.distBits = make([]uint8, 0x40)
	result.distCodes = make([]uint8, 0x40)
	return result
}

const maxRepLength = 0x204 // The longest allowed repetition

var distBits = []uint8{
	0x02, 0x04, 0x04, 0x05, 0x05, 0x05, 0x05, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x06,
	0x06, 0x06, 0x06, 0x06, 0x06, 0x06, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
	0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,
	0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08,
}

var distCodes = []uint8{
	0x03, 0x0D, 0x05, 0x19, 0x09, 0x11, 0x01, 0x3E, 0x1E, 0x2E, 0x0E, 0x36, 0x16, 0x26, 0x06, 0x3A,
	0x1A, 0x2A, 0x0A, 0x32, 0x12, 0x22, 0x42, 0x02, 0x7C, 0x3C, 0x5C, 0x1C, 0x6C, 0x2C, 0x4C, 0x0C,
	0x74, 0x34, 0x54, 0x14, 0x64, 0x24, 0x44, 0x04, 0x78, 0x38, 0x58, 0x18, 0x68, 0x28, 0x48, 0x08,
	0xF0, 0x70, 0xB0, 0x30, 0xD0, 0x50, 0x90, 0x10, 0xE0, 0x60, 0xA0, 0x20, 0xC0, 0x40, 0x80, 0x00,
}

var exLenBits = []uint8{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
}

var lenBits = []uint8{
	0x03, 0x02, 0x03, 0x03, 0x04, 0x04, 0x04, 0x05, 0x05, 0x05, 0x05, 0x06, 0x06, 0x06, 0x07, 0x07,
}

var lenCodes = []uint8{
	0x05, 0x03, 0x01, 0x06, 0x0A, 0x02, 0x0C, 0x14, 0x04, 0x18, 0x08, 0x30, 0x10, 0x20, 0x40, 0x00,
}

var chBitsAscs = []uint8{
	0x0B, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x08, 0x07, 0x0C, 0x0C, 0x07, 0x0C, 0x0C,
	0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0D, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C,
	0x04, 0x0A, 0x08, 0x0C, 0x0A, 0x0C, 0x0A, 0x08, 0x07, 0x07, 0x08, 0x09, 0x07, 0x06, 0x07, 0x08,
	0x07, 0x06, 0x07, 0x07, 0x07, 0x07, 0x08, 0x07, 0x07, 0x08, 0x08, 0x0C, 0x0B, 0x07, 0x09, 0x0B,
	0x0C, 0x06, 0x07, 0x06, 0x06, 0x05, 0x07, 0x08, 0x08, 0x06, 0x0B, 0x09, 0x06, 0x07, 0x06, 0x06,
	0x07, 0x0B, 0x06, 0x06, 0x06, 0x07, 0x09, 0x08, 0x09, 0x09, 0x0B, 0x08, 0x0B, 0x09, 0x0C, 0x08,
	0x0C, 0x05, 0x06, 0x06, 0x06, 0x05, 0x06, 0x06, 0x06, 0x05, 0x0B, 0x07, 0x05, 0x06, 0x05, 0x05,
	0x06, 0x0A, 0x05, 0x05, 0x05, 0x05, 0x08, 0x07, 0x08, 0x08, 0x0A, 0x0B, 0x0B, 0x0C, 0x0C, 0x0C,
	0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D,
	0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D,
	0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D,
	0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C,
	0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C,
	0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C, 0x0C,
	0x0D, 0x0C, 0x0D, 0x0D, 0x0D, 0x0C, 0x0D, 0x0D, 0x0D, 0x0C, 0x0D, 0x0D, 0x0D, 0x0D, 0x0C, 0x0D,
	0x0D, 0x0D, 0x0C, 0x0C, 0x0C, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D, 0x0D,
}

var chCodeAscs = []uint16{
	0x0490, 0x0FE0, 0x07E0, 0x0BE0, 0x03E0, 0x0DE0, 0x05E0, 0x09E0,
	0x01E0, 0x00B8, 0x0062, 0x0EE0, 0x06E0, 0x0022, 0x0AE0, 0x02E0,
	0x0CE0, 0x04E0, 0x08E0, 0x00E0, 0x0F60, 0x0760, 0x0B60, 0x0360,
	0x0D60, 0x0560, 0x1240, 0x0960, 0x0160, 0x0E60, 0x0660, 0x0A60,
	0x000F, 0x0250, 0x0038, 0x0260, 0x0050, 0x0C60, 0x0390, 0x00D8,
	0x0042, 0x0002, 0x0058, 0x01B0, 0x007C, 0x0029, 0x003C, 0x0098,
	0x005C, 0x0009, 0x001C, 0x006C, 0x002C, 0x004C, 0x0018, 0x000C,
	0x0074, 0x00E8, 0x0068, 0x0460, 0x0090, 0x0034, 0x00B0, 0x0710,
	0x0860, 0x0031, 0x0054, 0x0011, 0x0021, 0x0017, 0x0014, 0x00A8,
	0x0028, 0x0001, 0x0310, 0x0130, 0x003E, 0x0064, 0x001E, 0x002E,
	0x0024, 0x0510, 0x000E, 0x0036, 0x0016, 0x0044, 0x0030, 0x00C8,
	0x01D0, 0x00D0, 0x0110, 0x0048, 0x0610, 0x0150, 0x0060, 0x0088,
	0x0FA0, 0x0007, 0x0026, 0x0006, 0x003A, 0x001B, 0x001A, 0x002A,
	0x000A, 0x000B, 0x0210, 0x0004, 0x0013, 0x0032, 0x0003, 0x001D,
	0x0012, 0x0190, 0x000D, 0x0015, 0x0005, 0x0019, 0x0008, 0x0078,
	0x00F0, 0x0070, 0x0290, 0x0410, 0x0010, 0x07A0, 0x0BA0, 0x03A0,
	0x0240, 0x1C40, 0x0C40, 0x1440, 0x0440, 0x1840, 0x0840, 0x1040,
	0x0040, 0x1F80, 0x0F80, 0x1780, 0x0780, 0x1B80, 0x0B80, 0x1380,
	0x0380, 0x1D80, 0x0D80, 0x1580, 0x0580, 0x1980, 0x0980, 0x1180,
	0x0180, 0x1E80, 0x0E80, 0x1680, 0x0680, 0x1A80, 0x0A80, 0x1280,
	0x0280, 0x1C80, 0x0C80, 0x1480, 0x0480, 0x1880, 0x0880, 0x1080,
	0x0080, 0x1F00, 0x0F00, 0x1700, 0x0700, 0x1B00, 0x0B00, 0x1300,
	0x0DA0, 0x05A0, 0x09A0, 0x01A0, 0x0EA0, 0x06A0, 0x0AA0, 0x02A0,
	0x0CA0, 0x04A0, 0x08A0, 0x00A0, 0x0F20, 0x0720, 0x0B20, 0x0320,
	0x0D20, 0x0520, 0x0920, 0x0120, 0x0E20, 0x0620, 0x0A20, 0x0220,
	0x0C20, 0x0420, 0x0820, 0x0020, 0x0FC0, 0x07C0, 0x0BC0, 0x03C0,
	0x0DC0, 0x05C0, 0x09C0, 0x01C0, 0x0EC0, 0x06C0, 0x0AC0, 0x02C0,
	0x0CC0, 0x04C0, 0x08C0, 0x00C0, 0x0F40, 0x0740, 0x0B40, 0x0340,
	0x0300, 0x0D40, 0x1D00, 0x0D00, 0x1500, 0x0540, 0x0500, 0x1900,
	0x0900, 0x0940, 0x1100, 0x0100, 0x1E00, 0x0E00, 0x0140, 0x1600,
	0x0600, 0x1A00, 0x0E40, 0x0640, 0x0A40, 0x0A00, 0x1200, 0x0200,
	0x1C00, 0x0C00, 0x1400, 0x0400, 0x1800, 0x0800, 0x1000, 0x0000,
}

// calculating hash of the current byte pair.
// Note that most exact byte pair hash would be buffer[0] + buffer[1] << 0x08,
// but even this way gives nice indication of equal byte pairs, with significantly
// smaller size of the array that holds numbers of those hashes
func getBytePairHash(buffer []uint8, offset uint) uint16 {
	return uint16(buffer[offset]*4) + uint16(buffer[offset+1]*5)
}

// Builds the "hash_to_index" table and "pair_hash_offsets" table.
// Every element of "hash_to_index" will contain lowest index to the
// "pair_hash_offsets" table, effectively giving offset of the first
// occurrence of the given PAIR_HASH in the input data.
func sortBuffer(pWork *tCmpStruct, bufferBegin uint, bufferEnd uint) {
	var phashToIndex int
	var bufferPtr uint
	var totalSum = uint16(0)
	var bytePairHash uint32 // Hash value of the byte pair
	var bytePairOffs uint16 // Offset of the byte pair, relative to "workBuff"

	// Zero the entire "phashToIndex" table
	for m := range pWork.phashToIndex {
		pWork.phashToIndex[m] = 0
	}
	// Step 1: Count amount of each PAIR_HASH in the input buffer
	// The table will look like this:
	//  offs 0x000: Number of occurrences of PAIR_HASH 0
	//  offs 0x001: Number of occurrences of PAIR_HASH 1
	//  ...
	//  offs 0x8F7: Number of occurrences of PAIR_HASH 0x8F7 (the highest hash value)
	for bufferPtr = bufferBegin; bufferPtr < bufferEnd; bufferPtr++ {
		pWork.phashToIndex[getBytePairHash(pWork.workBuff, bufferPtr)]++
	}
	// Step 2: Convert the table to the array of PAIR_HASH amounts.
	// Each element contains count of PAIR_HASHes that is less or equal
	// to element index
	// The table will look like this:
	//  offs 0x000: Number of occurrences of PAIR_HASH 0 or lower
	//  offs 0x001: Number of occurrences of PAIR_HASH 1 or lower
	//  ...
	//  offs 0x8F7: Number of occurences of PAIR_HASH 0x8F7 or lower
	for phashToIndex = 0; phashToIndex < len(pWork.phashToIndex); phashToIndex++ {
		totalSum = totalSum + pWork.phashToIndex[phashToIndex]
		pWork.phashToIndex[phashToIndex] = totalSum
	}

	// Step 3: Convert the table to the array of indexes.
	// Now, each element contains index to the first occurrence of given PAIR_HASH
	for bufferEnd--; bufferEnd >= bufferBegin; bufferEnd-- {
		bytePairHash = uint32(getBytePairHash(pWork.workBuff, bufferEnd))
		bytePairOffs = uint16(bufferEnd)

		pWork.phashToIndex[bytePairHash]--
		pWork.phashOffs[pWork.phashToIndex[bytePairHash]] = bytePairOffs
	}
}

func flushBuf(pWork *tCmpStruct) {
	var saveCh1 uint8
	var saveCh2 uint8
	pWork.writeBuf.Write(pWork.outBuff[0:0x800])

	saveCh1 = pWork.outBuff[0x800]
	saveCh2 = pWork.outBuff[pWork.outBytes]
	pWork.outBytes -= 0x800
	if pWork.outBytes != 0 {
		pWork.outBuff[0] = saveCh1
	}
	if pWork.outBits != 0 {
		pWork.outBuff[pWork.outBytes] = saveCh2
	}
}

func outputBits(pWork *tCmpStruct, nBits uint16, bitBuff uint) {
	var outBits uint

	// If more than 8 bits to output, do recursion
	if nBits > 8 {
		outputBits(pWork, 8, bitBuff)
		bitBuff >>= 8
		nBits -= 8
	}

	// Add bits to the last out byte in outBuff;
	outBits = pWork.outBits
	pWork.outBuff[pWork.outBytes] |= uint8(bitBuff << outBits)
	pWork.outBits += uint(nBits)

	// If 8 or more bits, increment number of bytes
	if pWork.outBits > 8 {
		pWork.outBytes++
		bitBuff >>= 8 - outBits

		pWork.outBuff[pWork.outBytes] = uint8(bitBuff)
		pWork.outBits &= 7
	} else {
		pWork.outBits &= 7
		if pWork.outBits == 0 {
			pWork.outBytes++
		}
	}

	// If there is enough compressed bytes, flush them
	if pWork.outBytes >= 0x800 {
		flushBuf(pWork)
	}
}

// This function searches for a repetition
// (a previous occurence of the current byte sequence)
// Returns length of the repetition, and stores the backward distance
// to pWork structure.
func findRep(pWork *tCmpStruct, workBuffOffset uint) uint {
	var (
		phashToIndex     uint // Pointer into pWork.phashToIndex table
		phashOffs        uint // Pointer to the table containing offsets of each PAIR_HASH
		repetitionLimit  int  // An eventual repetition must be at position below this pointer
		prevRepetition uint // Pointer to the previous occurrence of the current PAIR_HASH
		prevRepEnd     uint // End of the previous repetition
		inputDataPtr   uint
		phashOffsIndex uint                  // Index to the table with PAIR_HASH positions
		minPhashOffs   uint16                // The lowest allowed hash offset
		offsInRep      uint16                // Offset within found repetition
		equalByteCount uint                  // Number of bytes that are equal to the previous occurence
		repLength                        = uint(1) // Length of the found repetition
		repLength2     uint                  // Secondary repetition
		preLastByte    uint8                 // Last but one byte from a repetition
		diVal          uint16
	)
	// Calculate the previous position of the PAIR_HASH
	phashToIndex = uint(getBytePairHash(pWork.workBuff, workBuffOffset))
	minPhashOffs = uint16(workBuffOffset - pWork.dsizeBytes + 1)
	phashOffsIndex = uint(pWork.phashToIndex[phashToIndex])

	// If the PAIR_HASH offset is below the limit, find a next one
	phashOffs = phashOffsIndex
	if pWork.phashOffs[phashOffs] < minPhashOffs {
		for pWork.phashOffs[phashOffs] < minPhashOffs {
			phashOffsIndex++
			phashOffs++
		}
		pWork.phashToIndex[phashToIndex] = uint16(phashOffsIndex)
	}

	// Get the first location of the PAIR_HASH,
	// and thus the first eventual location of byte repetition
	phashOffs = phashOffsIndex
	prevRepetition = uint(pWork.phashOffs[phashOffs]) // offset to pWork.workBuff
	repetitionLimit = int(workBuffOffset) - 1

	// If the current PAIR_HASH was not encountered before,
	// we haven't found a repetition.
	if int(prevRepetition) >= repetitionLimit {
		return 0
	}
	// We have found a match of a PAIR_HASH. Now we have to make sure
	// that it is also a byte match, because PAIR_HASH is not unique.
	// We compare the bytes and count the length of the repetition
	inputDataPtr = workBuffOffset
	for {
		// If the first byte of the repetition and the so-far-last byte
		// of the repetition are equal, we will compare the blocks.
		if pWork.workBuff[inputDataPtr] == pWork.workBuff[prevRepetition] && pWork.workBuff[inputDataPtr+repLength-1] == pWork.workBuff[prevRepetition+repLength-1] {
			// Skip the current byte
			prevRepetition++
			inputDataPtr++
			equalByteCount = 2

			// Now count how many more bytes are equal
			for equalByteCount < maxRepLength {
				prevRepetition++
				inputDataPtr++

				// Are the bytes different ?
				if pWork.workBuff[prevRepetition] != pWork.workBuff[inputDataPtr] {
					break
				}
				equalByteCount++
			}

			// If we found a repetition of at least the same length, take it.
			// If there are multiple repetitions in the input buffer, this will
			// make sure that we find the most recent one, which in turn allows
			// us to store backward length in less amount of bits
			inputDataPtr = workBuffOffset
			if equalByteCount >= repLength {
				// Calculate the backward distance of the repetition.
				// Note that the distance is stored as decremented by 1
				pWork.distance = uint(workBuffOffset - prevRepetition + equalByteCount - 1)

				// Repetitions longer than 10 bytes will be stored in more bits,
				// so they need a bit different handling
				repLength = equalByteCount
				if repLength > 10 {
					break
				}
			}
		}

		// Move forward in the table of PAIR_HASH repetitions.
		// There might be a more recent occurrence of the same repetition.
		phashOffsIndex++
		phashOffs++
		prevRepetition = uint(pWork.phashOffs[phashOffs])

		// If the next repetition is beyond the minimum allowed repetition, we are done.
		if int(prevRepetition) >= repetitionLimit {
			// A repetition must have at least 2 bytes, otherwise it's not worth it
			if repLength >= 2 {
				return repLength
			}
			return 0
		}
	}

	// If the repetition has max length of 0x204 bytes, we can't go any fuhrter
	if equalByteCount == maxRepLength {
		pWork.distance--
		return equalByteCount
	}

	// Check for possibility of a repetition that occurs at more recent position
	phashOffs = phashOffsIndex
	if int(pWork.phashOffs[phashOffs+1]) >= repetitionLimit {
		return repLength
	}

	// The following part checks if there isn't a longer repetition at
	// a latter offset, that would lead to better compression.
	//
	// Example of data that can trigger this optimization:
	//
	//   "EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEQQQQQQQQQQQQ"
	//   "XYZ"
	//   "EEEEEEEEEEEEEEEEQQQQQQQQQQQQ";
	//
	// Description of data in this buffer
	//   [0x00] Single byte "E"
	//   [0x01] Single byte "E"
	//   [0x02] Repeat 0x1E bytes from [0x00]
	//   [0x20] Single byte "X"
	//   [0x21] Single byte "Y"
	//   [0x22] Single byte "Z"
	//   [0x23] 17 possible previous repetitions of length at least 0x10 bytes:
	//          - Repetition of 0x10 bytes from [0x00] "EEEEEEEEEEEEEEEE"
	//          - Repetition of 0x10 bytes from [0x01] "EEEEEEEEEEEEEEEE"
	//          - Repetition of 0x10 bytes from [0x02] "EEEEEEEEEEEEEEEE"
	//          ...
	//          - Repetition of 0x10 bytes from [0x0F] "EEEEEEEEEEEEEEEE"
	//          - Repetition of 0x1C bytes from [0x10] "EEEEEEEEEEEEEEEEQQQQQQQQQQQQ"
	//          The last repetition is the best one.
	//

	pWork.offs09BC[0] = 0xFFFF
	pWork.offs09BC[1] = 0x0000
	diVal = 0

	// Note: I failed to figure out what does the table "offs09BC" mean.
	// If anyone has an idea, let me know to zezula_at_volny_dot_cz
	for offsInRep = 1; uint(offsInRep) < repLength; {
		if pWork.workBuff[workBuffOffset+uint(offsInRep)] != pWork.workBuff[workBuffOffset+uint(diVal)] {
			diVal = pWork.offs09BC[diVal]
			if diVal != 0xFFFF {
				continue
			}
		}
		offsInRep++
		diVal++
		pWork.offs09BC[offsInRep] = diVal
	}

	//
	// Now go through all the repetitions from the first found one
	// to the current input data, and check if any of them migh be
	// a start of a greater sequence match.
	//

	prevRepetition = uint(pWork.phashOffs[phashOffs])
	prevRepEnd = prevRepetition + repLength
	repLength2 = repLength

	for {
		repLength2 = uint(pWork.offs09BC[repLength2])
		if repLength2 == 0xFFFF {
			repLength2 = 0
		}
		// Get the pointer to the previous repetition
		phashOffs = phashOffsIndex

		// Skip those repetitions that don't reach the end
		// of the first found repetition
		for ok := true; ok; ok = prevRepetition+repLength2 < prevRepEnd {
			phashOffs++
			phashOffsIndex++
			prevRepetition = uint(pWork.phashOffs[phashOffs])
			if int(prevRepetition) >= repetitionLimit {
				return repLength
			}
		}

		// Verify if the last but one byte from the repetition matches
		// the last but one byte from the input data.
		// If not, find a next repetition
		preLastByte = pWork.workBuff[workBuffOffset+repLength-2]
		if preLastByte == pWork.workBuff[prevRepetition+repLength-2] {
			// If the new repetition reaches beyond the end
			// of previously found repetition, reset the repetition length to zero.
			if prevRepetition+repLength2 != prevRepEnd {
				prevRepEnd = prevRepetition
				repLength2 = 0
			}
		} else {
			phashOffs = phashOffsIndex
			for ok := true; ok; ok = pWork.workBuff[prevRepetition+repLength-2] != preLastByte ||
				pWork.workBuff[prevRepetition] != pWork.workBuff[workBuffOffset] {
				phashOffs++
				phashOffsIndex++
				prevRepetition = uint(pWork.phashOffs[phashOffs])
				if int(prevRepetition) >= repetitionLimit {
					return repLength
				}
			}

			// Reset the length of the repetition to 2 bytes only
			prevRepEnd = prevRepetition + 2
			repLength2 = 2
		}

		// Find out how many more characters are equal to the first repetition.
		for prevRepEnd == workBuffOffset+repLength2 {
			repLength2++
			if repLength2 >= 0x204 {
				break
			}
			prevRepEnd++
		}

		// Is the newly found repetition at least as long as the previous one ?
		if repLength2 >= repLength {
			// Calculate the distance of the new repetition
			pWork.distance = uint(workBuffOffset - prevRepetition - 1)
			repLength = repLength2
			if repLength == 0x204 {
				return repLength
			}

			// Update the additional elements in the "offs09BC" table
			// to reflect new rep length
			for uint(offsInRep) < repLength2 {
				if pWork.workBuff[workBuffOffset+uint(offsInRep)] != pWork.workBuff[workBuffOffset+uint(diVal)] {
					diVal = pWork.offs09BC[diVal]
					if diVal != 0xFFFF {
						continue
					}
				}
				diVal++
				offsInRep++
				pWork.offs09BC[offsInRep] = diVal
			}
		}
	}
}

func writeCmpData(pWork *tCmpStruct) {
	var inputDataEndIndex uint // Pointer to the end of the input data
	var workBuffOffset = pWork.dsizeBytes + 0x204
	var inputDataEnded = false // If 1, then all data from the input stream have been already loaded
	var saveRepLength uint     // Saved length of current repetition
	var saveDistance = uint(0) // Saved distance of current repetition
	var repLength uint         // Length of the found repetition
	var phase = uint(0)        //

	// Store the compression type and dictionary size
	pWork.outBuff[0] = uint8(pWork.cType)
	pWork.outBuff[1] = uint8(pWork.dsizeBits)
	pWork.outBytes = 2

	// Reset output buffer to zero
	for m := range pWork.outBuff {
		if m > 1 {
			pWork.outBuff[m] = 0
		}
	}
	pWork.outBits = 0

	for !inputDataEnded {
		var bytesToLoad = uint(0x1000)
		var totalLoaded = int(0)
		var bytesLoaded int

		// Load the bytes from the input stream, up to 0x1000 bytes
		for bytesToLoad != 0 {
			input := make([]byte, bytesToLoad)
			bytesLoaded, _ = pWork.readBuf.Read(input)
			copy(pWork.workBuff[pWork.dsizeBytes+0x204+uint(totalLoaded):pWork.dsizeBytes+0x204+uint(totalLoaded)+uint(bytesToLoad)], input)
			if bytesLoaded == 0 {
				if totalLoaded == 0 && phase == 0 {
					goto __Exit
				}
				inputDataEnded = true
				break
			} else {
				bytesToLoad -= uint(bytesLoaded)
				totalLoaded += bytesLoaded
			}
		}

		inputDataEndIndex = pWork.dsizeBytes + uint(totalLoaded)
		if inputDataEnded {
			inputDataEndIndex = inputDataEndIndex + uint(0x204)
		}
		//
		// Warning: The end of the buffer passed to "sortBuffer" is actually 2 bytes beyond
		// valid data. It is questionable if this is actually a bug or not,
		// but it might cause the compressed data output to be dependent on random bytes
		// that are in the buffer.
		// To prevent that, the calling application must always zero the compression
		// buffer before passing it to "implode"
		//

		// Search the PAIR_HASHes of the loaded blocks. Also, include
		// previously compressed data, if any.
		switch phase {
		case 0:
			sortBuffer(pWork, workBuffOffset, inputDataEndIndex+1)
			phase++
			if pWork.dsizeBytes != 0x1000 {
				phase++
			}
			break

		case 1:
			sortBuffer(pWork, workBuffOffset-pWork.dsizeBytes+0x204, inputDataEndIndex+1)
			phase++
			break

		default:
			sortBuffer(pWork, workBuffOffset-pWork.dsizeBytes, inputDataEndIndex+1)
			break
		}

		// Perform the compression of the current block
		for workBuffOffset < inputDataEndIndex {
			// Find if the current byte sequence wasn't there before.
			repLength = findRep(pWork, workBuffOffset)
			for repLength != 0 {
				// If we found repetition of 2 bytes, that is 0x100 or fuhrter back,
				// don't bother. Storing the distance of 0x100 bytes would actually
				// take more space than storing the 2 bytes as-is.
				if repLength == 2 && pWork.distance >= 0x100 {
					break
				}
				// When we are at the end of the input data, we cannot allow
				// the repetition to go past the end of the input data.
				if inputDataEnded && workBuffOffset+repLength > inputDataEndIndex {
					// Shorten the repetition length so that it only covers valid data
					repLength = uint(inputDataEndIndex - workBuffOffset)
					if repLength < 2 {
						break
					}
					// If we got repetition of 2 bytes, that is 0x100 or more backward, don't bother
					if repLength == 2 && pWork.distance >= 0x100 {
						break
					}
					goto __FlushRepetition
				}

				if repLength >= 8 || workBuffOffset+1 >= inputDataEndIndex {
					goto __FlushRepetition
				}
				// Try to find better repetition 1 byte later.
				// Example: "ARROCKFORT" "AROCKFORT"
				// When "input_data" points to the second string, findRep
				// returns the occurence of "AR". But there is longer repetition "ROCKFORT",
				// beginning 1 byte after.
				saveRepLength = repLength
				saveDistance = pWork.distance
				repLength = findRep(pWork, workBuffOffset+1)

				// Only use the new repetition if it's length is greater than the previous one
				if repLength > saveRepLength {
					// If the new repetition if only 1 byte better
					// and the previous distance is less than 0x80 bytes, use the previous repetition
					if repLength > saveRepLength+1 || saveDistance > 0x80 {
						// Flush one byte, so that input_data will point to the secondary repetition
						outputBits(pWork, uint16(pWork.nChBits[pWork.workBuff[workBuffOffset]]), uint(pWork.nChCodes[pWork.workBuff[workBuffOffset]]))
						workBuffOffset++
						continue
					}
				}

				// Revert to the previous repetition
				repLength = saveRepLength
				pWork.distance = saveDistance

			__FlushRepetition:

				outputBits(pWork, uint16(pWork.nChBits[repLength+0xFE]), uint(pWork.nChCodes[repLength+0xFE]))
				if repLength == 2 {
					outputBits(pWork, uint16(pWork.distBits[pWork.distance>>2]), uint(pWork.distCodes[pWork.distance>>2]))
					outputBits(pWork, 2, pWork.distance&3)
				} else {
					outputBits(pWork, uint16(pWork.distBits[pWork.distance>>pWork.dsizeBits]),
						uint(pWork.distCodes[pWork.distance>>pWork.dsizeBits]))
					outputBits(pWork, uint16(pWork.dsizeBits), pWork.dsizeMask&pWork.distance)
				}

				// Move the begin of the input data by the length of the repetition
				workBuffOffset += repLength
				goto _00402252
			}

			// If there was no previous repetition for the current position in the input data,
			// just output the 9-bit literal for the one character
			outputBits(pWork, uint16(pWork.nChBits[pWork.workBuff[workBuffOffset]]), uint(pWork.nChCodes[pWork.workBuff[workBuffOffset]]))
			workBuffOffset++
		_00402252:
		}

		if !inputDataEnded {
			workBuffOffset -= 0x1000
			copy(pWork.workBuff[0:pWork.dsizeBytes+0x204], pWork.workBuff[0x1000:0x1000+pWork.dsizeBytes+0x204])
		}
	}

__Exit:

	// Write the termination literal
	outputBits(pWork, uint16(pWork.nChBits[0x305]), uint(pWork.nChCodes[0x305]))
	if pWork.outBits != 0 {
		pWork.outBytes++
	}
	pWork.writeBuf.Write(pWork.outBuff[:pWork.outBytes])
	return
}

var (
	// ErrInvalidDictSize is returned when writing data and an invalid dictionary size is specified.
	ErrInvalidDictSize = errors.New("blast: invalid dictionary size")
	// ErrInvalidMode is returned when writing data and an invalid implode mode was given.
	ErrInvalidMode = errors.New("blast: invalid implode mode")
)

func implode(r io.Reader, w io.Writer, workBuf *tCmpStruct, implodeType uint, dSize uint) error {
	var pWork = workBuf
	var nChCode uint
	var nCount uint
	var i uint
	var nCount2 int

	// Fill the work buffer information
	// Note: The caller must zero the "workBuf" before passing it to implode
	pWork.readBuf = r
	pWork.writeBuf = w
	pWork.dsizeBytes = dSize
	pWork.cType = implodeType
	//pWork.param = param
	pWork.dsizeBits = 4
	pWork.dsizeMask = 0x0F

	// Test dictionary size
	switch dSize {
	case DictionarySize4096: // 0x1000 bytes
		pWork.dsizeBits++
		pWork.dsizeMask |= 0x20
		// No break here !!!
		pWork.dsizeBits++
		pWork.dsizeMask |= 0x10
		break
	case DictionarySize2048: // 0x800 bytes
		pWork.dsizeBits++
		pWork.dsizeMask |= 0x10
		// No break here !!!
		break
	case DictionarySize1024: // 0x400
		break

	default:
		return ErrInvalidDictSize
	}

	// Test the compression type
	switch implodeType {
	case Binary: // We will compress data with binary compression type
		nChCode = 0
		for nCount = 0; nCount < 0x100; nCount++ {
			pWork.nChBits[nCount] = 9
			pWork.nChCodes[nCount] = uint16(nChCode)
			nChCode = (nChCode & 0x0000FFFF) + 2
		}
		break

	case ASCII: // We will compress data with ASCII compression type
		for nCount = 0; nCount < 0x100; nCount++ {
			pWork.nChBits[nCount] = uint8(chBitsAscs[nCount] + 1)
			pWork.nChCodes[nCount] = uint16(chCodeAscs[nCount] * 2)
		}
		break

	default:
		return ErrInvalidMode
	}

	for i = 0; i < 0x10; i++ {
		if 1<<exLenBits[i] != 0 {
			for nCount2 = 0; nCount2 < (1 << exLenBits[i]); nCount2++ {
				pWork.nChBits[nCount] = uint8(exLenBits[i] + lenBits[i] + 1)
				pWork.nChCodes[nCount] = uint16((uint16(nCount2) << uint16(lenBits[i]+1)) | uint16((uint16(lenCodes[i])&0x00FF)*2) | 1)
				nCount++
			}
		}
	}

	// Copy the distance codes and distance bits and perform the compression
	copy(pWork.distCodes, distCodes)
	copy(pWork.distBits, distBits)
	writeCmpData(pWork)
	return nil
}

// A Writer takes data written to it and writes the compressed
// form of that data to an underlying writer (see NewWriter).
type Writer struct {
	w           io.Writer
	compressor  *tCmpStruct
	implodeType uint
	dictSize    uint
	data        []uint8
	err         error
}

// NewWriter creates a new Writer.
// Writes to the returned Writer are compressed and written to w.
//
// It is the caller's responsibility to call Close on the WriteCloser when done.
// Writes may be buffered and not flushed until Close.
func NewWriter(w io.Writer, implodeType uint, dictSize uint) *Writer {
	compressor := newTCmpStruct()
	writer := new(Writer)
	writer.w = w
	writer.implodeType = implodeType
	writer.dictSize = dictSize
	writer.compressor = compressor
	writer.data = make([]byte, 0)
	return writer
}

// Write writes a compressed form of p to the underlying io.Writer. The
// compressed bytes are not necessarily flushed until the Writer is closed.
func (w *Writer) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

// Close flushes and closes the writer.
func (w *Writer) Close() error {
	return implode(bytes.NewBuffer(w.data), w.w, w.compressor, w.implodeType, w.dictSize)
}
