package main

import (
	"strconv"
	"sync"
)

type Piece struct {
	Index      int
	PieceSize  int64
	BlockMap   map[uint32]*Block
	BitMap     []byte
	NumBlocks  int
	FileMap    []*File
	BitMapLock sync.RWMutex
}

type Block struct {
	Offset     int
	Data       []byte
	PieceIndex int
	Size       int
}

type File struct {
	FileName  string
	startIndx int64
	endIndx   int64
}

func (b *Block) createBlockKey() string {
	return strconv.Itoa(b.PieceIndex) + "_" + strconv.Itoa(b.Offset)
}
