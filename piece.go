package main

type Piece struct {
	Index    int
	BlockMap map[uint32]Block
	BitMap   []byte
}

type Block struct {
	Index int
	Data  []byte
}
