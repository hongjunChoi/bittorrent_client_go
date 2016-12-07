package main

import (
	"fmt"
)

type Torrent struct {
	BitMap   []byte
	FileName string
	PeerList []*Peer
	InfoHash string
}

func (t *Torrent) initBitMap(u int64) {
	length := int(u / 8)
	if u%8 > 0 {
		length += 1
	}
	t.BitMap = make([]byte, length)
	fmt.Println(t.BitMap[0])
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}
