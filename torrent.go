package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"
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
	MetaInfo       *MetaInfo
	FileList       []*os.File
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

func (torrent *Torrent) sendHaving(piece *Piece) {
	pieceIndex := piece.Index
	msg := createHaveMsg(pieceIndex)
	for _, peer := range torrent.PeerList {
		conn := *peer.Connection
		_, err := conn.Write(msg)
		if err != nil {
			fmt.Println("==== error in sending have message for seeding to peers =========")
			fmt.Println(err)
		}
	}

}

func (c *Client) handleChoke(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handleUnchoke(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("==== handle Unchoke =====")
	fmt.Println(len(torrent.PeerWorkMap[peer]))
	for i := 0; i < 25; i++ {
		b := torrent.PeerWorkMap[peer][i]
		peer.sendRequestMessage(b)
	}
	peer.CurrentBlock = 25
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
	completeMap := createOnesBitMap(piece.NumBlocks)
	// fmt.Println("pieceVal: ", piece.Index)
	// fmt.Println("offsetVal: ", byteOffset)
	// fmt.Println("bitMap: ", piece.BitMap)

	if bytes.Compare(completeMap, piece.BitMap) == 0 {
		fmt.Println("======= PIECE COMPLETE: ALL BLOCKS HAVE BEEN DOWNLOADED ======")

		pieceBuf := make([]byte, torrent.PieceSize)
		for i := 0; i < piece.NumBlocks; i++ {
			pieceBuf = append(pieceBuf, piece.BlockMap[uint32(i*BLOCKSIZE)].Data...)
		}
		fileMap := piece.FileMap
		numFiles := len(piece.FileMap)
		start := int64(0)
		end := int64(0)
		for i := 0; i < numFiles; i++ {
			if i != 0 {
				fmt.Println("finished downloading ", fileMap[i].FileName)
				fmt.Println(time.Now().Format(time.RFC850))
			}
			file, err := os.OpenFile(
				fileMap[i].FileName,
				os.O_WRONLY,
				0666,
			)
			end = start + fileMap[i].endIndx - fileMap[i].startIndx
			file.Seek(fileMap[i].startIndx, 0)
			if err != nil {
				fmt.Println(err)
			}
			file.Write(pieceBuf[start:end])
			defer file.Close()
			start = end
		}

		// Write bytes to file
		// file.Seek(0, 0)
		// byteSlice := []byte("Bytes!\n")
		// bytesWritten, err := file.Write(byteSlice)
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// fmt.Println("Wrote %d bytes.\n", bytesWritten)

		// file.Seek(20, 0)
		// bytesWritten, err = file.Write(byteSlice)
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// fmt.Println("Wrote %d bytes.\n", bytesWritten)
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

// func computePieceLocation(pieceIndex int, torrent *Torrent) {
// 	fileDictList := torrent.MetaInfo.Info.Files
// 	byteIndx := pieceIndex * BLOCKSIZE
// 	int count = 0
// 	for i := 0 ; i < len(fileDictList); i++ {
// 		if (fileDictList[i].Length)
// 	}
// }

func (c *Client) handleCancel(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handlePort(peer *Peer, torrent *Torrent, payload []byte) {

}
