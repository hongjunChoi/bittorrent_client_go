package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type Torrent struct {
	BlockOffsetMap map[uint32]int64 //for each piece show unti where data is already downloaded (offset in bytes)
	BitMap         []byte           //map that shows whether piece at index is downloaded or not
	FileName       string
	PeerList       []*Peer
	InfoHash       string
	NumPieces      int
	PieceSize      int64
	PeerWorkMap    map[*Peer]([]*Block)
	PieceMap       map[uint32]*Piece
	BlockSize      uint32
	FileTrial      *os.File
}

func (t *Torrent) initBitMap() {
	t.BitMap = createZerosBitMap(t.NumPieces)
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}

func getBit(n uint8, pos int) uint8 {
	return (n >> uint(pos)) & 1
}

func (c *Client) handleChoke(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleUnchoke(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("==== handle Unchoke =====")
	fmt.Println(len(torrent.PeerWorkMap[peer]))
	for i := 0; i < 1; i++ {
		b := torrent.PeerWorkMap[peer][i]
		peer.sendRequestMessage(b)
	}
	peer.CurrentBlock = 1
}

func (c *Client) handleInterested(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleNotInterested(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleHave(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleBitfield(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleRequest(peer *Peer, torrent *Torrent, payload []byte) {

}
func createOnesBitMap(bits int) []byte {
	length := int(bits / 8)
	trailing := bits % 8
	if trailing > 0 {
		length += 1
	}
	bitMap := make([]byte, length)
	for i := 0; i < length; i++ {
		bitMap[i] = byte(255)
	}

	if trailing > 0 {
		pow := 7
		byteVal := 0
		for i := 0; i < trailing; i++ {
			byteVal += int(math.Pow(2, float64(pow)))
		}
		bitMap[length-1] = byte(byteVal)
	}
	fmt.Println("-------")
	fmt.Println(bitMap)
	return bitMap
}

func createZerosBitMap(bits int) []byte {
	length := int(bits / 8)
	trailing := bits % 8
	if trailing > 0 {
		length += 1
	}
	bitMap := make([]byte, length)
	return bitMap
}

func (c *Client) handlePiece(peer *Peer, torrent *Torrent, payload []byte) {

	pieceIndex := binary.BigEndian.Uint32(payload[0:4])
	byteOffset := binary.BigEndian.Uint32(payload[4:8])
	data := payload[8:]

	piece := torrent.PieceMap[pieceIndex]

	//GET CORRESPONDING BLOCK AND SET DATA
	block := torrent.PieceMap[pieceIndex].BlockMap[byteOffset]
	block.Data = data

	//UPDATE BITMAP OF PIECE

	bitMapByteIndx := int(byteOffset / BLOCKSIZE / 8)
	bitMapBitIndx := int(byteOffset/BLOCKSIZE) % 8

	byteValue := piece.BitMap[bitMapByteIndx]
	flipByteValue := setBit(int(byteValue), uint(bitMapBitIndx))
	piece.BitMap[bitMapByteIndx] = byte(flipByteValue)

	//CHECK IF PIECE IS FULL
	fmt.Println("----------")
	fmt.Println(piece.NumBlocks)
	completeMap := createOnesBitMap(piece.NumBlocks)
	fmt.Println("pieceVal: ", piece.Index)
	fmt.Println("offsetVal: ", byteOffset)
	fmt.Println("bitMap: ", piece.BitMap)

	if bytes.Compare(completeMap, piece.BitMap) == 0 {
		fmt.Println("======= PIECE COMPLETE: ALL BLOCKS HAVE BEEN DOWNLOADED ======")
		// f, err := os.OpenFile(filename, os.O_APPEND, 0666)

		// n, err := f.WriteString(text)

		// f.Close()
	}

	// REMOVE BLOCK FROM BLOCk QUEUE
	b := peer.BlockQueue.Dequeue()
	if uint32(b.(*Block).Offset) != byteOffset {
		fmt.Println("======= WEIRD POPING FROM BLOCK QUEUE =========")
	}

	//GET NEW BLOCK TO REQUEST
	toRequest := torrent.PeerWorkMap[peer][peer.CurrentBlock]
	peer.CurrentBlock += 1
	peer.sendRequestMessage(toRequest)
	//ADD TO C

	//UPDATE THE BITMAP IFF THE ENTIRE PIECE HAS BEEN DOWNLOADED

	//DOWNLOAD THE DATA AND SAVE

	//STORE FILE PATH => SAVE TO MAPPING OF TORRENT : FILES : PIECES

	//CALL NEXT BLOCK IN QUEUE FOR MORE DOWNLOAD
}

func (c *Client) handleCancel(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handlePort(peer *Peer, torrent *Torrent, payload []byte) {

}
