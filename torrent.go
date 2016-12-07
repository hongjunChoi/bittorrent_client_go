package main

import (
	"encoding/binary"
	"fmt"
)

type Torrent struct {
	BlockOffsetMap map[uint32]int64 //for each piece show unti where data is already downloaded (offset in bytes)
	BitMap         []byte           //map that shows whether piece at index is downloaded or not
	FileName       string
	PeerList       []*Peer
	InfoHash       string
	NumPieces      int
	PieceSize      int64
	PeerWorkMap    map[*Peer]([]*Piece)
}

func (t *Torrent) initBitMap() {
	length := int(t.NumPieces / 8)
	if t.NumPieces%8 > 0 {
		length += 1
	}
	t.BitMap = make([]byte, length)
	fmt.Println(t.BitMap[0])
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}

func (c *Client) handleChoke(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleUnchoke(peer *Peer, torrent *Torrent, payload []byte) {

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

func (c *Client) handlePiece(peer *Peer, torrent *Torrent, payload []byte) {
	pieceIndex := binary.BigEndian.Uint32(payload[0:4])
	byteOffset := binary.BigEndian.Uint32(payload[4:8])
	data := payload[8:]

	//UPDATE THE OFFSET BYTE BY AMOUNT OF DATA RECVED
	torrent.BlockOffsetMap[pieceIndex] = int64(byteOffset) + int64(len(data))

	//UPDATE THE BITMAP IFF THE ENTIRE PIECE HAS BEEN DOWNLOADED

	//DOWNLOAD THE DATA AND SAVE

	//STORE FILE PATH => SAVE TO MAPPING OF TORRENT : FILES : PIECES

	//CALL NEXT BLOCK IN QUEUE FOR MORE DOWNLOAD
}

func (c *Client) handleCancel(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handlePort(peer *Peer, torrent *Torrent, payload []byte) {

}
