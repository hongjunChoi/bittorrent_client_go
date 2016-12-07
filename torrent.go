package main

type Torrent struct {
	BitMap   []byte
	FileName string
	PeerList []*Peer
	InfoHash string
}

func (t *Torrent) initBitMap(u int64) {
	// t.BitMap = 0
}
