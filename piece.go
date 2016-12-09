package main

type Piece struct {
	Index     int
	BlockMap  map[uint32]*Block
	BitMap    []byte
	NumBlocks int
	FileMap   []*File
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
