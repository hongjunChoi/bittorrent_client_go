package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

type Torrent struct {
	Name            string
	BlockOffsetMap  map[uint32]int64 //for each piece show unti where data is already downloaded (offset in bytes)
	BitMap          []byte           //map that shows whether piece at index is downloaded or not
	FileName        string
	PeerList        []*Peer
	InfoHash        string
	NumPieces       int
	PieceSize       int64
	PeerWorkMap     map[*Peer]([]*Block)
	PieceMap        map[uint32]*Piece
	BlockSize       uint32
	MetaInfo        *MetaInfo
	FileList        []*os.File
	TrackerUrl      string
	TrackerInterval int
	ClientId        string
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
	fmt.Println("=======  SENDING HAVING MSG TO PEERS ======")
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
	fmt.Println("===== HANDLE CHOKE  =======")
}

func (c *Client) handleUnchoke(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("==== handle Unchoke =====")
	for i := 0; i < 20; i++ {
		if i >= len(torrent.PeerWorkMap[peer]) {
			return
		}

		b := torrent.PeerWorkMap[peer][i]
		peer.sendRequestMessage(b)
	}
	peer.WorkMapLock.Lock()
	peer.CurrentBlock = 20
	peer.WorkMapLock.Unlock()

}

func (c *Client) handleInterested(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("===== HANDLE INTERESTED =======")
	conn := *peer.Connection
	_, err := conn.Write(createUnChokeMsg())
	if err != nil {
		fmt.Println("==== error in sending unchoke")
		fmt.Println(err)
	}
}

func (c *Client) handleNotInterested(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("===== HANDLE  NOT  INTERESTED =======")
}

func (c *Client) handleHave(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("===== HANDLE  HAVE   =======")
}

func (c *Client) handleBitfield(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("===== HANDLE  BITFIELD   =======")
}

func (c *Client) handleRequest(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("===== HANDLE  REQUEST   =======")
	indx := binary.BigEndian.Uint32(payload[:4])
	begin := binary.BigEndian.Uint32(payload[4:8])
	length := int64(binary.BigEndian.Uint32(payload[8:12]))

	fmt.Println("idx: ", indx, "begin: ", begin, "length: ", length)
	fileMap := torrent.PieceMap[indx].FileMap

	block := make([]byte, 0)

	for fIndx := 0; fIndx < len(fileMap); fIndx++ {
		fmt.Println("checking file ")
		file, err := os.Open(fileMap[fIndx].FileName)
		if err != nil {
			fmt.Println("error opening file: ", fileMap[fIndx].FileName)
		}
		start := fileMap[fIndx].startIndx
		end := fileMap[fIndx].endIndx
		if end-start >= length {
			data := make([]byte, length)
			_, err = file.ReadAt(data, start)
			block = append(block, data...)
			file.Close()
			fmt.Println("break")
			break
		} else {
			data := make([]byte, end-start)
			_, err = file.ReadAt(data, start)
			length = length - (end - start)
			block = append(block, data...)
			file.Close()
		}
	}

	// fmt.Println("payload block to send : ", block)

	peer.sendPieceMessage(indx, begin, block)
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
			pow = pow - 1
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

func (piece *Piece) freeBlockMemory() {
	for _, b := range piece.BlockMap {
		b.Data = make([]byte, 0)
	}
}

func (c *Client) handlePiece(peer *Peer, torrent *Torrent, payload []byte) {

	pieceIndex := binary.BigEndian.Uint32(payload[0:4])
	byteOffset := binary.BigEndian.Uint32(payload[4:8])
	data := payload[8:]
	fmt.Println("=====    RECEIVED   PIECE INDEX : ", pieceIndex, "  /  BLOCK OFFSET : ", byteOffset, "   =======")

	piece := torrent.PieceMap[pieceIndex]

	//GET CORRESPONDING BLOCK AND SET DATA
	block := torrent.PieceMap[pieceIndex].BlockMap[byteOffset]
	block.Data = data

	//UPDATE BITMAP OF PIECE
	bitMapByteIndx := int(byteOffset / BLOCKSIZE / 8)
	bitMapBitIndx := int(byteOffset/BLOCKSIZE) % 8

	piece.BitMapLock.Lock()
	byteValue := piece.BitMap[bitMapByteIndx]

	flipByteValue := setBit(int(byteValue), 7-uint(bitMapBitIndx))
	piece.BitMap[bitMapByteIndx] = byte(flipByteValue)
	piece.BitMapLock.Unlock()

	//DELETE BLOCK FROM SENDING QUEUE
	key := strconv.Itoa(int(pieceIndex)) + "_" + strconv.Itoa(int(byteOffset))
	peer.QueueLock.Lock()
	b := peer.PeerQueueMap[key]
	delete(peer.PeerQueueMap, key)
	peer.QueueLock.Unlock()

	if b == nil || uint32(b.Offset) != byteOffset {

		peer.WorkMapLock.Lock()
		if peer.CurrentBlock < len(torrent.PeerWorkMap[peer]) {
			peer.sendRequestMessage(torrent.PeerWorkMap[peer][peer.CurrentBlock])
			peer.CurrentBlock += 1
		}
		peer.WorkMapLock.Unlock()
		return
	}

	//CHECK IF PIECE IS FULL
	completeMap := createOnesBitMap(piece.NumBlocks)

	if bytes.Compare(completeMap, piece.BitMap) == 0 {
		fmt.Println("======= PIECE COMPLETE: ALL BLOCKS HAVE BEEN DOWNLOADED ======")

		pieceBuf := make([]byte, 0)
		for i := 0; i < piece.NumBlocks; i++ {
			pieceBuf = append(pieceBuf, piece.BlockMap[uint32(i*BLOCKSIZE)].Data...)
		}

		fileMap := piece.FileMap
		numFiles := len(piece.FileMap)
		start := int64(0)
		end := int64(0)
		fmt.Println("piece index :", piece.Index)

		for i := 0; i < numFiles; i++ {
			fmt.Println("----- writing to file", fileMap[i].FileName)
			fmt.Println("insert to ", fileMap[i].startIndx)
			if i != 0 {
				fmt.Println("finished downloading.. new file:  ", fileMap[i].FileName)
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
				fmt.Println("ERROR IN WRITING TO FILE AFTER DOWNLOAD ... ", err)
			}

			file.Write(pieceBuf[start:end])
			file.Close()
			start = end
		}

		torrent.sendHaving(piece)

		//IF ALL PEICES ARE DOWNLOADED THEN SEND COMPLETE MSG TO TRACKER
		downloadedMap := torrent.checkAlreadyDownloaded()
		allComplete := true
		for _, v := range downloadedMap {
			if !v {
				allComplete = false
				break
			}
		}

		if allComplete {
			fmt.Println("\n\n\n\n\n\n =========   download complete for torrent  ", torrent.Name, "    ================\n\n\n\n\n\n")
			torrent.sendComplete()
		}

		piece.freeBlockMemory()
	}

	//GET NEW BLOCK TO REQUEST
	peer.WorkMapLock.Lock()
	if peer.CurrentBlock < len(torrent.PeerWorkMap[peer]) {
		toRequest := torrent.PeerWorkMap[peer][peer.CurrentBlock]
		peer.sendRequestMessage(toRequest)
		peer.CurrentBlock += 1
	}

	peer.WorkMapLock.Unlock()

}

func (c *Client) handleCancel(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handlePort(peer *Peer, torrent *Torrent, payload []byte) {

}
