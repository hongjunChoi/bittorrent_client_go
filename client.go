package main

import (
	"./bencode-go"
	"./datastructure"
	"./shell"
	"bytes"
	"container/list"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"

	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	CHOKE          = 0
	UNCHOKE        = 1
	INTERESTED     = 2
	NOT_INTERESTED = 3
	HAVE           = 4
	BITFIELD       = 5
	REQUEST        = 6
	PIECE          = 7
	CANCEL         = 8
	PORT           = 9
	BLOCKSIZE      = 2048
)

type Peer struct {
	PeerChannel      chan bool
	RemoteBitMap     []byte
	SelfChoking      bool
	SelfInterested   bool
	RemoteChoking    bool
	RemoteInterested bool
	RemotePeerId     string
	RemotePeerIP     string
	RemotePeerPort   uint16
	Connection       *net.Conn
	BlockQueue       *lane.Queue
	PeerQueueMap     (map[string](*Block))
	QueueLock        sync.RWMutex
	WorkMapLock      sync.RWMutex
	CurrentBlock     int
}

type Client struct {
	Id          string  // self peer id
	Peers       []*Peer //MAP of remote peer id : peer data
	TorrentList []*Torrent
	FunctionMap map[int]func(*Peer, *Torrent, []byte)
}

func main() {
	client := createClient()
	go client.startListeningToSeed()

	clientCommands := map[string]shell.Command{
		"add":    shell.Command{client.addTorrent, "torrent add <torrent_file>", "add torrent and start downloading / seeding", 2},
		"remove": shell.Command{client.removeTorrent, "torrent remove <torrend_id> ", "remove torrent ", 2},
		"detail": shell.Command{client.showTorrentDetail, "torrent detail <torrent_id> ", "show detail of torrent ", 2},
		"list":   shell.Command{client.listTorrent, "torrent list", "list all torrents ", 0},
	}

	var s shell.Shell
	s.Done = make(chan bool)
	go s.Interact(clientCommands)

	for {
		select {

		case <-s.Done:
			fmt.Println("quiting...")
			// close everything here ...
			for i, torrent := range client.TorrentList {
				for _, peer := range torrent.PeerList {
					peer.closePeerConnection()
				}
				client.TorrentList = append(client.TorrentList[:i], client.TorrentList[i+1:]...)
			}
			return
		}
	}
}

func (c *Client) listTorrent(arg []string) {
	fmt.Println("\tid,     name,     done,    files,    peers,   uploaded,   downloaded")
	fmt.Println("======================================================================")

	for i, torrent := range c.TorrentList {
		downloadedMap := torrent.checkAlreadyDownloaded()
		downloaded := true
		downloadedBytes := int64(0)
		uploadedBytes := int64(0)
		for i, b := range downloadedMap {
			if !b {
				downloaded = false
			} else {
				downloadedBytes += torrent.PieceMap[uint32(i)].PieceSize
			}
		}

		fmt.Println("\t", i, "\t", torrent.Name, "\t", downloaded, "\t", len(torrent.FileList), "\t", len(torrent.PeerList), "\t", uploadedBytes, "\t", downloadedBytes)

	}

}

func (c *Client) removeTorrent(arg []string) {
	id, err := strconv.Atoi(arg[2])
	if err != nil {
		fmt.Println("Incorrect argument. err : ", err)
		return
	}

	if id >= len(c.TorrentList) {
		fmt.Println("Torrent with given ID does not exit..")
		return
	}

	torrent := c.TorrentList[id]
	for _, peer := range torrent.PeerList {
		peer.closePeerConnection()
	}

	c.TorrentList = append(c.TorrentList[:id], c.TorrentList[id+1:]...)
}

func (c *Client) showTorrentDetail(arg []string) {
	id, err := strconv.Atoi(arg[2])
	if err != nil {
		fmt.Println("Incorrect argument..")
		return
	}

	if id >= len(c.TorrentList) {
		fmt.Println("Torrent with given ID does not exit..")
		return
	}

	torrent := c.TorrentList[id]
	totalBytes := torrent.getTotalSize()
	downloadedMap := torrent.checkAlreadyDownloaded()
	downloadedBytes := int64(0)

	for i, p := range torrent.PieceMap {
		if downloadedMap[int(i)] {
			downloadedBytes += p.PieceSize
		}
	}
	fmt.Println("\n\n\n=======   GENERAL INFO    =======")
	fmt.Println("\t torrent name : ", torrent.Name)
	fmt.Println("\t client local peer ID : ", c.Id)
	fmt.Println("\t total bytes : ", totalBytes)
	fmt.Println("\t downloaded bytes : ", downloadedBytes)
	fmt.Println("\t left bytes : ", totalBytes-downloadedBytes)
	fmt.Println("\t number of pieces : ", torrent.NumPieces)
	fmt.Println("\t number of files : ", len(torrent.FileList))
	fmt.Println("\t tracker url : ", torrent.TrackerUrl)
	fmt.Println("\t tracker interval : ", torrent.TrackerInterval)

	fmt.Println("\n\n\n=======   FILE INFO    =======")
	for i, _ := range torrent.FileList {
		file := torrent.FileList[i]
		fmt.Println("\t file name : ", file.FileName)
	}

	fmt.Println("\n\n\n======= Piece Info ========")
	for i := 0; i < torrent.NumPieces; i++ {
		piece := torrent.PieceMap[uint32(i)]
		fmt.Println("\n\npiece index : ", piece.Index)
		fmt.Println("\t piece size : ", piece.PieceSize)
		fmt.Println("\t piece block bitmap : ", piece.BitMap)
		fmt.Println("\t piece downloaded or not : ", downloadedMap[i])
	}

	fmt.Println("\n\n\n======= Peer Info ==========")
	for _, peer := range torrent.PeerList {
		fmt.Println("\n\npeer id : ", peer.RemotePeerId)
		fmt.Println("\t peer bitmap : ", peer.RemoteBitMap)
	}

}

func (c *Client) startListeningToSeed() {
	// Start listening to port 8888 for TCP connection
	listener, err := net.Listen("tcp", ":6881")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		listener.Close()
		fmt.Println("Listener closed")
	}()

	for {
		// Get net.TCPConn object
		conn, err := listener.Accept()

		fmt.Println("======== NEW CONNECTION SEEDING ====== \n\n")
		if err != nil {
			fmt.Println(err)
			break
		}

		go c.handleConnection(conn)
	}
}

func (c *Client) handleConnection(conn net.Conn) {
	fmt.Println("Handling new connection...")

	var t *Torrent
	p := new(Peer)

	p.Connection = &conn
	readBuffer := make([]byte, 0)
	for {
		// fmt.Println("waiting for peer request...")
		buf := make([]byte, 1024)
		numBytes, err := conn.Read(buf)
		if err != nil {
			fmt.Println("read error from peer in seeding :", err)
			return
		}

		// fmt.Println("------ received from seeding thread", conn.RemoteAddr(), buf[:numBytes])
		buf = buf[:numBytes]
		if numBytes == 49+int(buf[0]) {

			// fmt.Println("maybe handshake")
			pstrlen := int(buf[0])
			// pstr := string(buf[1 : pstrlen+1])
			// resv := buf[pstrlen+1 : pstrlen+1+8]
			info_hash := string(buf[pstrlen+1+8 : pstrlen+1+8+20])
			// peer_id := buf[pstrlen+1+8+20 : pstrlen+1+8+20+20]
			// fmt.Println("protocol: ", pstr)
			// fmt.Println("reserved: ", resv)
			// fmt.Println("info hash: ", info_hash)
			// fmt.Println("peer_id: ", peer_id)
			exists := false
			numTorrents := len(c.TorrentList)
			for i := 0; i < numTorrents; i++ {
				torrent := c.TorrentList[i]
				if torrent.InfoHash == info_hash {
					// fmt.Println("info hash matches.. start sending bitMap info")
					t = torrent
					exists = true
				}
			}

			if !exists {
				fmt.Println("torrent requested does not exist")
				return
			}

			if t != nil {
				// fmt.Println("sending handshake")
				conn.Write(createHandShakeMsg("BitTorrent protocol", t.InfoHash, c.Id))
				bitMapMsg := createBitMapMsg(t)
				fmt.Println("sending ", bitMapMsg)
				conn.Write(bitMapMsg)
			}
			bitMapMsg := createBitMapMsg(t)
			// fmt.Println("sending ", bitMapMsg)
			conn.Write(bitMapMsg)

		} else {
			readBuffer = append(readBuffer, buf...)
			size := binary.BigEndian.Uint32(buf[0:4])
			if size == 0 {
				// fmt.Println("----- alive message received in seeding thread", buf)
				continue
			}
			bufferLen := len(readBuffer)
			for int(size) <= bufferLen-4 {
				// fmt.Println("---- msg ----")
				protocol := readBuffer[4]
				data := readBuffer[5 : 5+size-1]

				// fmt.Println("size: ", size)
				// fmt.Println("protocol: ", protocol)
				// fmt.Println("payload: ", data)
				// fmt.Println("do something with this protocol")

				go c.FunctionMap[int(protocol)](p, t, data)

				if int(size) == len(readBuffer)-4 {
					// fmt.Println("end of buffer")
					readBuffer = make([]byte, 0)
					bufferLen = 0
					size = 0
					break
				} else {
					// fmt.Println("more left in buffer", bufferLen, size)
					readBuffer = readBuffer[5+size-1:]
					size = binary.BigEndian.Uint32(readBuffer[0:4])
					bufferLen = len(readBuffer)
					// fmt.Println(readBuffer)
				}
			}
		}
	}
}

//returns a list of boolean. if bool at index i is true, than piece [i] is already downloaded
func (torrent *Torrent) checkAlreadyDownloaded() []bool {
	hash := torrent.MetaInfo.Info.Pieces
	numPiece := len(hash) / 20
	bitMap := make([]bool, numPiece)

	for i := 0; i < numPiece; i++ {
		piece := torrent.PieceMap[uint32(i)]

		pieceHash := hash[i*20 : (i+1)*20]
		pieceData := make([]byte, 0)

		for j := 0; j < len(piece.FileMap); j++ {
			filename := piece.FileMap[j].FileName
			startIndex := piece.FileMap[j].startIndx
			endIndex := piece.FileMap[j].endIndx

			file, err := os.Open(filename)
			if err != nil {
				break
			}

			b := make([]byte, endIndex-startIndex)
			_, err = file.ReadAt(b, startIndex)

			if err != nil {
				break
			}

			pieceData = append(pieceData, b...)
		}

		if checkHash(pieceData, pieceHash) {
			bitMap[i] = true
		} else {
			bitMap[i] = false
		}
	}
	return bitMap
}

func createClient() *Client {
	client := new(Client)
	client.Id = url.QueryEscape(generatePeerId())
	client.createStateFunctionMap()
	return client
}

func (c *Client) createStateFunctionMap() {
	functionMap := make(map[int]func(*Peer, *Torrent, []byte))

	functionMap[CHOKE] = c.handleChoke
	functionMap[UNCHOKE] = c.handleUnchoke
	functionMap[INTERESTED] = c.handleInterested
	functionMap[NOT_INTERESTED] = c.handleNotInterested
	functionMap[HAVE] = c.handleHave
	functionMap[BITFIELD] = c.handleBitfield
	functionMap[REQUEST] = c.handleRequest
	functionMap[PIECE] = c.handlePiece
	functionMap[CANCEL] = c.handleCancel
	functionMap[PORT] = c.handlePort

	c.FunctionMap = functionMap
}

func generateFilePath(path []string) string {
	filePath := "down"
	for i := 0; i < len(path); i++ {
		filePath = filePath + "/" + path[i]
	}
	return filePath
}

func createFiles(metaInfo *MetaInfo) {
	numFiles := len(metaInfo.Info.Files)
	if numFiles > 0 {
		fmt.Println("====== ", numFiles, " files ========")
		for i := 0; i < numFiles; i++ {
			fmt.Println(metaInfo.Info.Files[i].Path)
			fmt.Println(metaInfo.Info.Files[i].Length)

			//IF FILE ALREADY EXIST THEN CONTINUE..
			if _, err := os.Stat(generateFilePath(metaInfo.Info.Files[i].Path)); err == nil {
				// fmt.Println("file already exist!")
				continue
			}

			//Create File
			// Open a new file for writing only
			file, err := os.OpenFile(
				generateFilePath(metaInfo.Info.Files[i].Path),
				os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
				0666,
			)
			if err != nil {
				fmt.Println("=== ERROR CREATING FILE =====")
				fmt.Println(err)
				return
			}
			file.Close()
		}
	} else {

		path := make([]string, 1)
		path[0] = metaInfo.Info.Name

		//IF FILE ALREADY EXIST THEN CONTINUE..
		if _, err := os.Stat(path[0]); err == nil {
			// fmt.Println("========   file already exist!   ======")
			return
		}

		file, err := os.OpenFile(
			generateFilePath(path),
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0666,
		)

		if err != nil {
			fmt.Println("=== ERROR CREATING FILE =====")
			fmt.Println(err)
			return
		}
		file.Close()
	}
}

func (torrent *Torrent) createDataBlocks() {
	for i := 0; i < torrent.NumPieces; i++ {
		piece := new(Piece)
		piece.Index = i
		piece.BlockMap = make(map[uint32]*Block)
		numBlocks := int(math.Ceil(float64(torrent.PieceSize / BLOCKSIZE)))
		piece.NumBlocks = numBlocks
		piece.BitMap = createZerosBitMap(numBlocks)

		fileDictList := torrent.MetaInfo.Info.Files
		byteIndx := int64(i) * torrent.PieceSize
		pieceOffset := int64(0)
		cursor := int64(0)
		totalSize := int64(0)

		//CREATE FILE MAPPING FOR EACH PIECE
		for k := 0; k < len(fileDictList); k++ {
			fileInfo := fileDictList[k]
			totalSize += fileInfo.Length

			if byteIndx < cursor+fileInfo.Length {
				// start writing to file
				file := new(File)
				file.FileName = generateFilePath(fileInfo.Path)
				file.startIndx = byteIndx - cursor
				torrent.FileList = append(torrent.FileList, file)
				bytesLeftInFile := cursor + fileInfo.Length - byteIndx
				piece.FileMap = append(piece.FileMap, file)

				if torrent.PieceSize-pieceOffset <= bytesLeftInFile {
					file.endIndx = file.startIndx + torrent.PieceSize - pieceOffset
					break
				} else {
					file.endIndx = file.startIndx + bytesLeftInFile
					pieceOffset += bytesLeftInFile
					byteIndx = cursor + fileInfo.Length
				}
			}
			cursor += fileInfo.Length
		}

		//EDGE CASE FOR LAST PIECE WITH DIFFERENT SIZE AND THUS DIFFERENT NUM BLOCKS
		if i == torrent.NumPieces-1 {
			//TODO: for the last piece the size and NUMBLOCK is different!! recalculate
			bytesDownloaded := int64(i) * int64(torrent.PieceSize)
			numBlocks = int(math.Ceil(float64(totalSize-bytesDownloaded) / float64(BLOCKSIZE)))
			piece.NumBlocks = numBlocks
			// fmt.Println("\n\n\n========number of blocks in the last piece  : ", numBlocks, "==========\n\n\n")
			piece.BitMap = createZerosBitMap(numBlocks)
		}

		pieceSize := int64(0)

		//CREATE BLOCKS FOR PIECE : MAKE SURE TO TAKE CARE OF LAST PEICE EDGE CASE
		for j := 0; j < numBlocks; j++ {
			b := new(Block)
			b.Offset = j * BLOCKSIZE
			b.PieceIndex = i
			b.Data = make([]byte, BLOCKSIZE)
			b.Size = BLOCKSIZE

			if j == numBlocks-1 && i == torrent.NumPieces-1 {
				b.Size = int((totalSize - int64(i)*int64(torrent.PieceSize)) - int64(b.Offset))
				b.Data = make([]byte, b.Size)
			}

			pieceSize += int64(b.Size)
			piece.BlockMap[uint32(b.Offset)] = b
		}

		piece.PieceSize = pieceSize
		torrent.PieceMap[uint32(i)] = piece
	}

}

func (torrent *Torrent) createWorkList(downloadMap []bool) {

	for i := 0; i < torrent.NumPieces; i++ {
		if downloadMap[i] {

			byteIndx := int(i / 8)
			bitIndx := int(i % 8)

			byteVal := torrent.BitMap[byteIndx]
			flipByteVal := setBit(int(byteVal), 7-uint(bitIndx))
			torrent.BitMap[byteIndx] = byte(flipByteVal)

			fmt.Println("ALREADY DOWNLOADED.. downloaded piece index : ", i)
			continue
		}

		pieceIndex := i
		piece := torrent.PieceMap[uint32(pieceIndex)]

		for _, block := range piece.BlockMap {
			torrent.WorkList.PushBack(block)
		}
	}

	fmt.Println("\n\n==========")
	fmt.Println(torrent.WorkList.Len())
	fmt.Println("==========\n\n")
}

func (torrent *Torrent) divideWork(downloadMap []bool) {
	count := 0

	for i := 0; i < torrent.NumPieces; i++ {
		if downloadMap[i] {

			byteIndx := int(i / 8)
			bitIndx := int(i % 8)

			byteVal := torrent.BitMap[byteIndx]
			flipByteVal := setBit(int(byteVal), 7-uint(bitIndx))
			torrent.BitMap[byteIndx] = byte(flipByteVal)

			fmt.Println("ALREADY DOWNLOADED.. downloaded piece index : ", i)
			continue
		}

		pieceIndex := i
		piece := torrent.PieceMap[uint32(pieceIndex)]

		for {
			count += 1

			peer := torrent.PeerList[count%(len(torrent.PeerList))]
			fmt.Println("peer list length :  ", len(torrent.PeerList), "  / count :  ", count, " / peeer : ", peer.RemotePeerId)

			//if peer bit map in index of currnet piece is 1 then give all piece blocks to peer
			bitShiftIndex := 7 - (pieceIndex % 8)
			bitmapIndex := pieceIndex / 8
			fmt.Println(peer.RemoteBitMap)

			if getBit(peer.RemoteBitMap[bitmapIndex], int(bitShiftIndex)) == 1 {
				for _, block := range piece.BlockMap {
					torrent.PeerWorkMap[peer] = append(torrent.PeerWorkMap[peer], block)
				}

				break

			}
		}

	}
}

func (c *Client) addTorrent(arg []string) {
	filename := arg[2]
	fmt.Println("filename : ", filename)
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile(filename)
	fmt.Println("===== checking how many files")
	fmt.Println(metaInfo.Info.Name)
	fmt.Println(metaInfo.Info.Length)
	fmt.Println(len(metaInfo.Info.Files))
	fmt.Println("===========")

	createFiles(metaInfo)

	torrent := new(Torrent)
	torrent.Name = filename
	torrent.MetaInfo = metaInfo

	torrent.NumPieces = len(metaInfo.Info.Pieces) / 20
	torrent.PieceSize = metaInfo.Info.PieceLength
	torrent.BlockOffsetMap = make(map[uint32]int64)
	torrent.PeerWorkMap = make(map[*Peer]([]*Block))
	torrent.WorkList = list.New()
	torrent.FileList = make([]*File, 0)
	torrent.MetaInfo = metaInfo
	torrent.ClientId = c.Id

	fmt.Println("======= ", torrent.NumPieces, " PIECES TOTAL ==========")
	if torrent.NumPieces == 0 {
		return
	}

	for i := 0; i < torrent.NumPieces; i++ {
		torrent.BlockOffsetMap[uint32(i)] = 0
	}

	torrent.initBitMap()
	trackerUrl := metaInfo.Announce
	fmt.Println(metaInfo.AnnounceList)

	trackerUrl = metaInfo.AnnounceList[0][0]
	data := parseMetaInfo(metaInfo)
	data["peer_id"] = c.Id
	data["event"] = "started"

	peerList := torrent.get_peer_list(trackerUrl, data)
	torrent.PeerList = peerList
	torrent.InfoHash = metaInfo.InfoHash
	torrent.PieceMap = make(map[uint32]*Piece)

	c.TorrentList = append(c.TorrentList, torrent)

	//CREATE ALL PIECE AND BLOCKS IN TORRENT AND STORE IN STRUCT
	torrent.createDataBlocks()
	c.peerListHandShake(torrent)

	if len(torrent.PeerList) == 0 {
		fmt.Println("all peers failed handshake...")
		return
	}

	//DIVIDE WORK AMONG PEERS HERE
	downloadMap := torrent.checkAlreadyDownloaded()
	torrent.createWorkList(downloadMap)
	// torrent.divideWork(downloadMap)

	for _, peer := range torrent.PeerList {

		go c.handlePeerConnection(peer, torrent)
	}

	go torrent.handleTracker()

}

func (torrent *Torrent) getTotalSize() int64 {
	size := int64(0)
	for i := 0; i < int(torrent.NumPieces); i++ {
		size += int64(torrent.PieceMap[uint32(i)].PieceSize)
	}
	return size

}

func (torrent *Torrent) sendComplete() {
	fmt.Println("===== SENDING COMPLETE MSG TO TRACKER... ======")
	trackerUrl := torrent.TrackerUrl
	data := parseMetaInfo(torrent.MetaInfo)
	data["peer_id"] = torrent.ClientId
	data["left"] = strconv.FormatInt(0, 10)
	data["downloaded"] = strconv.FormatInt(torrent.getTotalSize(), 10)
	data["event"] = "completed"

	url := createTrackerQuery(trackerUrl, data)
	resp, err := http.Get(url)

	if err != nil {
		fmt.Println("error in contacing tracker every interval...")
		return
	}

	fmt.Println("======== TRACKER RESPONSE AFTER SENDING COMPLTE =======")
	fmt.Println(resp.Body)
	fmt.Println("======================")

	resp.Body.Close()
}

func (torrent *Torrent) handleTracker() {
	for {
		time.Sleep(time.Duration(torrent.TrackerInterval) * time.Second)

		boolMap := torrent.checkAlreadyDownloaded()
		downloaded := int64(0)
		left := int64(0)

		for _, isDownloaded := range boolMap {
			if isDownloaded {
				downloaded += torrent.PieceSize
			} else {
				left += torrent.PieceSize
			}
		}

		trackerUrl := torrent.TrackerUrl
		data := parseMetaInfo(torrent.MetaInfo)
		data["peer_id"] = torrent.ClientId
		data["left"] = strconv.FormatInt(left, 10)
		data["downloaded"] = strconv.FormatInt(downloaded, 10)

		url := createTrackerQuery(trackerUrl, data)
		resp, err := http.Get(url)

		if err != nil {
			fmt.Println("error in contacing tracker every interval...")
			resp.Body.Close()
			return
		}

		resp.Body.Close()

	}
}

func (c *Client) peerListHandShake(torrent *Torrent) {
	validPeers := make([]*Peer, 0)

	for _, peer := range torrent.PeerList {
		//IF handshake filed.
		if peer != nil && c.connectToPeer(peer, torrent) {
			peer.SelfInterested = true
			validPeers = append(validPeers, peer)
		}
	}
	torrent.PeerList = validPeers
	go keepPeerListAlive(torrent)
	return

}

func (p *Peer) sendKeepAlive() {
	conn := *p.Connection
	conn.Write(make([]byte, 0))
}

//sends KEEP ALIVE to each peer periodically
func keepPeerListAlive(torrent *Torrent) {
	for {
		time.Sleep(10 * time.Second)
		for _, p := range torrent.PeerList {
			p.sendKeepAlive()
		}
	}
}

func (peer *Peer) closePeerConnection() {
	peer.PeerChannel <- true
	(*peer.Connection).Close()
}

func (c *Client) handlePeerConnection(peer *Peer, torrent *Torrent) {
	conn := *peer.Connection

	// 2) SEND INTERESTED MSG
	interestMsg := createInterestMsg()
	_, err := conn.Write(interestMsg)

	if err != nil {
		fmt.Println("==== ERROR IN WRITING INTERESTED MSG ======")
		fmt.Println(err)
		fmt.Println("============")
		return
	}

	fmt.Println("waiting for  response after sending our interested msg..")

	for {
		sizeBuf := make([]byte, 4)
		_, err := conn.Read(sizeBuf)
		if err != nil && err != io.EOF {
			fmt.Println("read error from peer.. 111  :", err)
			return
		}

		size := binary.BigEndian.Uint32(sizeBuf[0:4])
		if size == 0 {
			conn.Write(interestMsg)
			continue
		}

		payload := make([]byte, 0)
		numReceived := 0

		for numReceived < int(size) {
			buf := make([]byte, int(size)-numReceived)
			bytesToAdd := 0
			bytesToAdd, err = conn.Read(buf)
			if err != nil {
				fmt.Println("read error from peer..  222:", err)
				return
			}
			numReceived += bytesToAdd
			payload = append(payload, buf[0:bytesToAdd]...)
		}

		protocol := payload[0]
		data := payload[1:]
		go c.FunctionMap[int(protocol)](peer, torrent, data)

		select {
		case flag := <-peer.PeerChannel:
			if flag {
				fmt.Println("closing peer connection ... ")
				return
			}

		default:
			continue
		}
	}

}

//TODO: COMPLETE THIS PART
func parseMetaInfo(info *MetaInfo) map[string]string {

	data := make(map[string]string)
	data["info_hash"] = url.QueryEscape(info.InfoHash)
	data["port"] = "6881"
	data["uploaded"] = "0"
	data["downloaded"] = "0"
	data["left"] = "0"

	return data
}

func (torrent *Torrent) get_peer_list(trackerUrl string, data map[string]string) []*Peer {
	url := createTrackerQuery(trackerUrl, data)
	torrent.TrackerUrl = trackerUrl

	fmt.Println("\n\n==========  CONTACTING TRACKER ========")
	fmt.Println(url)
	fmt.Println("======================================\n\n")

	resp, err := http.Get(url)

	if err != nil {
		fmt.Println("\n\n====  error in getting resp from tracker server  ====")
		fmt.Println(err)
		fmt.Println("========\n\n")
		return make([]*Peer, 0)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	r := bytes.NewReader(body)
	peerDictData, decodeErr := bencode.Decode(r)
	if decodeErr != nil {
		fmt.Println("=== ERROR IN DECODING BENCODE FROM TRACKER ======")
		fmt.Println(decodeErr)
		return make([]*Peer, 0)
	}

	peerDict, _ := peerDictData.(map[string]interface{})
	peers := []byte(peerDict["peers"].(string))
	torrent.TrackerInterval = int(peerDict["interval"].(int64))

	fmt.Println("==== PEER DATA FROM TRACKER =====")
	fmt.Println(peers)
	fmt.Println("==========")

	peerData := make([]*Peer, 0)

	myIP := get_external_IP()
	for i := 0; i < len(peers)/6; i++ {
		index := i * 6
		ip := peers[index : index+4]
		port := peers[index+4 : index+6]

		peer := new(Peer)
		peer.PeerChannel = make(chan bool, 1)
		peer.RemotePeerIP = net.IPv4(ip[0], ip[1], ip[2], ip[3]).String()

		fmt.Println("---------- IPS ---------- ")
		fmt.Println(myIP)
		fmt.Println(peer.RemotePeerIP)
		if peer.RemotePeerIP == myIP {
			fmt.Println("my IP Address in peer list")
			continue
		}

		peer.RemotePeerPort = binary.BigEndian.Uint16(port)
		peer.RemotePeerId = ""
		peer.BlockQueue = lane.NewQueue()
		peer.PeerQueueMap = make(map[string](*Block))
		peer.CurrentBlock = 0
		peerData = append(peerData, peer)
	}

	return peerData
}

func createTrackerQuery(baseUrl string, data map[string]string) string {
	url := baseUrl + "?"
	count := 0

	for k, v := range data {
		url = url + k + "=" + v

		if count < len(data)-1 {
			url = url + "&"
		}
		count += 1
	}
	return url
}

// Conducts HANDSHAKE WITH PEER AND STARTS CONNECTION
// RETURN FALSE IF HANDSHAKE FAILED ...
func (c *Client) connectToPeer(peer *Peer, torrent *Torrent) bool {
	peerIP := peer.RemotePeerIP
	peerPortNum := peer.RemotePeerPort
	infohash := torrent.InfoHash
	count := 0

	for count < 1 {
		count += 1

		fmt.Println("Conducting handshake to  : ", peerIP, " : ", peerPortNum, "   ......")
		conn, err := net.DialTimeout("tcp", peerIP+":"+strconv.Itoa(int(peerPortNum)), time.Duration(1)*time.Second)
		if err != nil {
			fmt.Println("ERROR IN PEER TCP HANDSHAKE  : ", err)
			peerPortNum += 1
			continue
		}
		fmt.Println("---- tcp handshake complete.. starting peer handshake-----")

		// Client transmit first message to server
		firstMsg := createHandShakeMsg("BitTorrent protocol", infohash, c.Id)
		conn.Write(firstMsg)

		//Recieve handshake
		lengthBuf := make([]byte, 1)
		_, err = conn.Read(lengthBuf)
		if err != nil {
			peerPortNum += 1
			continue
		}

		strLen := lengthBuf[0]
		handshakeLen := int(strLen) + 48
		buf := make([]byte, handshakeLen)
		_, err = conn.Read(buf)

		if err != nil {
			peerPortNum += 1
			continue
		}

		recvInfoHash := string(buf[8+strLen : 28+strLen])
		recvPeerId := string(buf[28+strLen:])

		//CHECK for inconsistent info hash  or peer id
		if peer.RemotePeerId == "" {
			peer.RemotePeerId = recvPeerId
		}

		if peer.RemotePeerId != recvPeerId || infohash != recvInfoHash {
			conn.Close()
			fmt.Println("incorrect handshake value in peer handshake ")
			return false
		}

		peer.Connection = &conn

		bitMapBuf := make([]byte, 4096)
		numRecved := 0
		numRecved, err = conn.Read(bitMapBuf)

		if err != nil && err != io.EOF {
			fmt.Println("error in reading bitmap of peer ,  error:", err)
			return false
		}

		bitMapBuf = bitMapBuf[0:numRecved]
		bitMapRecvLen := binary.BigEndian.Uint32(bitMapBuf[:4]) - 1
		if bitMapRecvLen == 0 {
			return false
		}

		peer.RemoteBitMap = bitMapBuf[5 : 5+bitMapRecvLen]
		fmt.Println(bitMapBuf[5 : 5+bitMapRecvLen])
		fmt.Println("handshake complete...\n\n\n")
		return true

	}

	return false
}

func (p *Peer) sendRequestMessage(b *Block) {
	fmt.Println("requesting data to peer... piece index : ", b.PieceIndex, "  / byte offset : ", b.Offset)
	data := createRequestMsg(b.PieceIndex, b.Offset, b.Size)
	_, err := (*p.Connection).Write(data)
	if err != nil {
		fmt.Println("==== ERROR IN SENDING REQUEST MESG TO PEER  ======")
		fmt.Println(err)
		fmt.Println("==============")
	}

	key := b.createBlockKey()
	p.QueueLock.Lock()
	p.PeerQueueMap[key] = b
	p.QueueLock.Unlock()
}

func (p *Peer) sendPieceMessage(indx uint32, begin uint32, block []byte) {
	fmt.Println("sending piece message with payload")
	data := createPieceMsg(indx, begin, block)
	_, err := (*p.Connection).Write(data)
	if err != nil {
		fmt.Println("==== ERROR IN SENDING REQUEST MESG TO PEER  ======")
		fmt.Println(err)
		fmt.Println("==============")
	}
}

func createPieceMsg(indx uint32, begin uint32, block []byte) []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(len(block)+9))
	data = append(data, tmp...)
	data = append(data, uint8(7))
	tmp = make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(indx))
	data = append(data, tmp...)
	tmp = make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(begin))
	data = append(data, tmp...)
	data = append(data, block...)
	return data
}
func createHaveMsg(pieceIndex int) []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(5))
	data = append(data, tmp...)
	data = append(data, uint8(4))

	pIndex := make([]byte, 4)
	binary.BigEndian.PutUint32(pIndex, uint32(pieceIndex))
	data = append(data, pIndex...)

	return data
}

func createRequestMsg(pieceIndex int, byteOffset int, byteSize int) []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(13))
	data = append(data, tmp...)
	data = append(data, uint8(6))
	binary.BigEndian.PutUint32(tmp, uint32(pieceIndex))
	data = append(data, tmp...)
	binary.BigEndian.PutUint32(tmp, uint32(byteOffset))
	data = append(data, tmp...)
	binary.BigEndian.PutUint32(tmp, uint32(byteSize))
	data = append(data, tmp...)
	return data
}

func createInterestMsg() []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(1))
	data = append(data, tmp...)
	data = append(data, uint8(2))
	return data
}

func createUnChokeMsg() []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(1))
	data = append(data, tmp...)
	data = append(data, uint8(1))
	return data
}

func createBitMapMsg(t *Torrent) []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(len(t.BitMap)+1))
	data = append(data, tmp...)
	data = append(data, uint8(5))
	data = append(data, t.BitMap...)
	return data
}

func createHandShakeMsg(msg string, infohash string, peerId string) []byte {

	msgLen := uint8(len(msg))
	zeros := make([]byte, 8)

	data := make([]byte, 0)
	data = append(data, msgLen)
	data = append(data, []byte(msg)...)
	data = append(data, zeros...)
	data = append(data, []byte(infohash)...)
	data = append(data, []byte(peerId)...)

	return data
}
