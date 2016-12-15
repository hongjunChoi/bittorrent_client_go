package main

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
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
	FileList        []*File
	TrackerUrl      string
	TrackerInterval int
	ClientId        string
	WorkList        *list.List
	WorkListLock    sync.RWMutex
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
	fmt.Println("===== HANDLE CHOKE  =======")
	peer.RemoteChoking = true
}

func (c *Client) handleUnchoke(peer *Peer, torrent *Torrent, payload []byte) {
	fmt.Println("==== handle Unchoke =====")
	peer.RemoteChoking = false

	for i := 0; i < 20; i++ {
		if !peer.RemoteChoking && peer.SelfInterested {
			blockToRequest := torrent.getNextBlock(peer)
			if blockToRequest != nil {
				peer.sendRequestMessage(blockToRequest)
			}
		}
	}

}

func (torrent *Torrent) getNextBlock(peer *Peer) *Block {
	workList := torrent.WorkList

	torrent.WorkListLock.Lock()
	defer torrent.WorkListLock.Unlock()

	fmt.Println("==== left =======")
	fmt.Println(workList.Len())
	fmt.Println("===========")

	for e := workList.Front(); e != nil; e = e.Next() {
		nextBlock := (e.Value).(*Block)
		pieceIndex := nextBlock.PieceIndex
		bitShiftIndex := 7 - (pieceIndex % 8)
		bitmapIndex := pieceIndex / 8

		if getBit(peer.RemoteBitMap[bitmapIndex], int(bitShiftIndex)) == 1 {
			torrent.WorkList.Remove(e)

			return nextBlock
		}
	}

	return nil

}

func (c *Client) handleInterested(peer *Peer, torrent *Torrent, payload []byte) {
	peer.RemoteInterested = true
	conn := *peer.Connection
	_, err := conn.Write(createUnChokeMsg())
	if err != nil {
		fmt.Println("==== error in sending unchoke  err : ", err, "   ==========")
	}
}

func (c *Client) handleNotInterested(peer *Peer, torrent *Torrent, payload []byte) {
	peer.RemoteInterested = false
	fmt.Println("===== HANDLE  NOT  INTERESTED =======")
}

func (c *Client) handleHave(peer *Peer, torrent *Torrent, payload []byte) {
	// fmt.Println("===== HANDLE  HAVE   =======")
}

func (c *Client) handleBitfield(peer *Peer, torrent *Torrent, payload []byte) {
	// fmt.Println("===== HANDLE  BITFIELD   =======")
}

func (c *Client) handleRequest(peer *Peer, torrent *Torrent, payload []byte) {
	// fmt.Println("===== HANDLE  REQUEST   =======")
	indx := binary.BigEndian.Uint32(payload[:4])
	begin := int64(binary.BigEndian.Uint32(payload[4:8]))
	length := int64(binary.BigEndian.Uint32(payload[8:12]))
	fileMap := torrent.PieceMap[indx].FileMap

	block := make([]byte, 0)
	fmt.Println("start looking for piece", indx, begin, length )
	for fIndx := 0; fIndx < len(fileMap); fIndx++ {
		fmt.Println("looking at file ", fIndx)
		file, err := os.Open(fileMap[fIndx].FileName)
		if err != nil {
			fmt.Println("error opening file: ", fileMap[fIndx].FileName)
		}
		start := fileMap[fIndx].startIndx
		end := fileMap[fIndx].endIndx
		if end - start + begin >= length {
			file.Seek(start + begin, 0)
			data := make([]byte, length)
			_, err = file.Read(data)
			block = append(block, data...)
			file.Close()
			fmt.Println("inside entire file")
			break
		} else {
			data := make([]byte, end - start + begin)
			file.Seek(start + begin, 0)
			_, err = file.Read(data)
			length = length - (end - start)
			block = append(block, data...)
			begin = 0
			fmt.Println("to next file")
			file.Close()
		}
	}
	fmt.Println("===== SENDING PIECE INDEX: ", indx, "BLOCK OFFSET: ", begin)
	// fmt.Println(block)
	peer.sendPieceMessage(indx, uint32(begin), block)
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
	fmt.Println("recved data...")
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

		if !peer.RemoteChoking && peer.SelfInterested {
			blockToRequest := torrent.getNextBlock(peer)
			if blockToRequest != nil {
				peer.sendRequestMessage(blockToRequest)
			} else {
				fmt.Println("NIL")
			}
		}
		return
	}

	//CHECK IF PIECE IS FULL
	completeMap := createOnesBitMap(piece.NumBlocks)

	if bytes.Compare(completeMap, piece.BitMap) == 0 {

		pieceBuf := make([]byte, 0)
		for i := 0; i < piece.NumBlocks; i++ {
			pieceBuf = append(pieceBuf, piece.BlockMap[uint32(i*BLOCKSIZE)].Data...)
		}
		// ================================
		//TODO: checking SHA1 HASH
		sha1Hash := torrent.MetaInfo.Info.Pieces[piece.Index*20 : (piece.Index+1)*20]
		isHashTrue := checkHash(pieceBuf, sha1Hash)
		fmt.Println("\n\n\n\n\n\n\n==== SHA1 HASH FOR DOWNLOADED DATA FOR PIECE WITH INDEX : ", piece.Index, "  IS  :", isHashTrue, "   ======\n\n\n\n\n\n\n")

		// ================================
		fileMap := piece.FileMap
		numFiles := len(piece.FileMap)
		start := int64(0)
		end := int64(0)

		for i := 0; i < numFiles; i++ {

			file, err := os.OpenFile(
				fileMap[i].FileName,
				os.O_WRONLY,
				0666,
			)
			end = start + fileMap[i].endIndx - fileMap[i].startIndx
			file.Seek(fileMap[i].startIndx, 0)
			if err != nil {
				fmt.Println("ERROR IN WRITING TO FILE AFTER DOWNLOAD ... ", err)
				file.Close()
				return
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
	if !peer.RemoteChoking && peer.SelfInterested {
		blockToRequest := torrent.getNextBlock(peer)
		if blockToRequest != nil {
			peer.sendRequestMessage(blockToRequest)
		}
	}

}

func (c *Client) handleCancel(peer *Peer, torrent *Torrent, payload []byte) {

}

func (c *Client) handlePort(peer *Peer, torrent *Torrent, payload []byte) {

}
