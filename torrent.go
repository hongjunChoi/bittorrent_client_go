package main

import (
	"fmt"
)

type Torrent struct {
	BitMap    []byte
	FileName  string
	PeerList  []*Peer
	InfoHash  string
	NumPieces int
	PieceSize int64
}

func (t *Torrent) initBitMap() {
	length := int(t.NumPieces / 8)
	if t.NumPieces%8 > 0 {
		length += 1
	}
	length = 51
	t.BitMap = make([]byte, length)
	fmt.Println(t.BitMap[0])
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}
