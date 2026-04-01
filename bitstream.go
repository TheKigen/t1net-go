package t1net

import (
	"fmt"
	"sync"
)

type bitStreamReader struct {
	buf    []byte
	bitPos int
	bitLen int
}

func newBitStreamReader(data []byte) *bitStreamReader {
	return &bitStreamReader{
		buf:    data,
		bitLen: len(data) * 8,
	}
}

// ReadBits reads n bits (LSB first) into a uint32.
func (bs *bitStreamReader) ReadBits(n int) (uint32, error) {
	if bs.bitPos+n > bs.bitLen {
		return 0, fmt.Errorf("not enough bits: need %d, have %d", n, bs.bitLen-bs.bitPos)
	}
	var val uint32
	for i := 0; i < n; i++ {
		byteIdx := bs.bitPos / 8
		bitIdx := uint(bs.bitPos % 8)
		if (bs.buf[byteIdx] & (1 << bitIdx)) != 0 {
			val |= 1 << uint(i)
		}
		bs.bitPos++
	}
	return val, nil
}

// ReadUint16 reads 16 bits and returns a uint16.
func (bs *bitStreamReader) ReadUint16() (uint16, error) {
	val, err := bs.ReadBits(16)
	return uint16(val), err
}

// BitsRemaining returns how many bits are left to read.
func (bs *bitStreamReader) BitsRemaining() int {
	return bs.bitLen - bs.bitPos
}

type huffNode struct {
	pop    uint32
	index0 int16 // left branch (negative = leaf)
	index1 int16 // right branch (negative = leaf)
}

type huffLeaf struct {
	pop     uint32
	numBits uint8
	symbol  uint8
	code    uint32
}

type huffProcessor struct {
	nodes  []huffNode
	leaves [256]huffLeaf
}

const csgProbBoost = 1

var csmCharFreqs = [256]uint32{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 329, 21, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	2809, 68, 0, 27, 0, 58, 3, 62, 4, 7, 0, 0, 15, 65, 554, 3,
	394, 404, 189, 117, 30, 51, 27, 15, 34, 32, 80, 1, 142, 3, 142, 39,
	0, 144, 125, 44, 122, 275, 70, 135, 61, 127, 8, 12, 113, 246, 122, 36,
	185, 1, 149, 309, 335, 12, 11, 14, 54, 151, 0, 0, 2, 0, 0, 211,
	0, 2090, 344, 736, 993, 2872, 701, 605, 646, 1552, 328, 305, 1240, 735, 1533, 1713,
	562, 3, 1775, 1149, 1469, 979, 407, 553, 59, 279, 31, 0, 0, 0, 68, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

func isAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

type huffWrap struct {
	nodeIdx int
	leafIdx int
}

func (hw *huffWrap) getPop(hp *huffProcessor) uint32 {
	if hw.nodeIdx >= 0 {
		return hp.nodes[hw.nodeIdx].pop
	}
	return hp.leaves[hw.leafIdx].pop
}

func (hw *huffWrap) determineIndex(hp *huffProcessor) int16 {
	if hw.leafIdx >= 0 {
		return int16(-(hw.leafIdx + 1))
	}
	return int16(hw.nodeIdx)
}

var (
	globalHuff     *huffProcessor
	globalHuffOnce sync.Once
)

func getHuffProcessor() *huffProcessor {
	globalHuffOnce.Do(func() {
		hp := &huffProcessor{}
		hp.buildTables()
		globalHuff = hp
	})
	return globalHuff
}

func (hp *huffProcessor) buildTables() {
	for i := 0; i < 256; i++ {
		hp.leaves[i].symbol = uint8(i)
		boost := uint32(csgProbBoost)
		if isAlnum(uint8(i)) {
			boost += csgProbBoost
		}
		hp.leaves[i].pop = csmCharFreqs[i] + boost
	}

	hp.nodes = make([]huffNode, 1, 256)

	wraps := make([]huffWrap, 256)
	for i := 0; i < 256; i++ {
		wraps[i] = huffWrap{nodeIdx: -1, leafIdx: i}
	}
	currWraps := 256

	for currWraps > 1 {
		var min1, min2 uint32 = 0xfffffffe, 0xffffffff
		idx1, idx2 := -1, -1

		for i := 0; i < currWraps; i++ {
			pop := wraps[i].getPop(hp)
			if pop < min1 {
				min2 = min1
				idx2 = idx1
				min1 = pop
				idx1 = i
			} else if pop < min2 {
				min2 = pop
				idx2 = i
			}
		}

		hp.nodes = append(hp.nodes, huffNode{
			pop:    wraps[idx1].getPop(hp) + wraps[idx2].getPop(hp),
			index0: wraps[idx1].determineIndex(hp),
			index1: wraps[idx2].determineIndex(hp),
		})
		newNodeIdx := len(hp.nodes) - 1

		mergeIdx := idx1
		nukeIdx := idx2
		if idx1 > idx2 {
			mergeIdx = idx2
			nukeIdx = idx1
		}
		wraps[mergeIdx] = huffWrap{nodeIdx: newNodeIdx, leafIdx: -1}

		if nukeIdx != currWraps-1 {
			wraps[nukeIdx] = wraps[currWraps-1]
		}
		currWraps--
	}

	hp.nodes[0] = hp.nodes[wraps[0].nodeIdx]
	hp.generateCodes(0, 0, 0)
}

func (hp *huffProcessor) generateCodes(index int16, code uint32, depth uint8) {
	if index < 0 {
		leaf := &hp.leaves[-(index + 1)]
		leaf.code = code
		leaf.numBits = depth
		return
	}
	node := &hp.nodes[index]
	hp.generateCodes(node.index0, code, depth+1)
	hp.generateCodes(node.index1, code|(1<<uint(depth)), depth+1)
}

// ReadHuffString reads a Huffman-encoded string from the bitStream.
func (bs *bitStreamReader) ReadHuffString() (string, error) {
	hp := getHuffProcessor()

	flagBits, err := bs.ReadBits(1)
	if err != nil {
		return "", err
	}
	compressed := flagBits == 1

	lenBits, err := bs.ReadBits(8)
	if err != nil {
		return "", err
	}
	length := int(lenBits)

	if length == 0 {
		return "", nil
	}

	if compressed {
		buf := make([]byte, length)
		skip32 := false
		for i := 0; i < length; i++ {
			index := int16(0)
			for index >= 0 {
				bit, err := bs.ReadBits(1)
				if err != nil {
					return "", err
				}
				if bit == 1 {
					index = hp.nodes[index].index1
				} else {
					index = hp.nodes[index].index0
				}
			}
			sym := hp.leaves[-(index + 1)].symbol
			buf[i] = sym
			if i == 0 && sym >= 128 {
				i--
				length--
				skip32 = true
			}
		}
		if skip32 {
			if _, err := bs.ReadBits(32); err != nil {
				return "", err
			}
		}
		return string(buf[:length]), nil
	}

	// Uncompressed: read 8 bits per character
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		b, err := bs.ReadBits(8)
		if err != nil {
			return "", err
		}
		buf[i] = byte(b)
	}
	return string(buf), nil
}
