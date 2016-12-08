package main

type Piece struct {
	Index     int
	BlockMap  map[uint32]*Block
	BitMap    []byte
	NumBlocks int
}

type Block struct {
	Offset     int
	Data       []byte
	PieceIndex int
	Size       int
}
