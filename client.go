package main

import (
	"./bencode-go"
	"./datastructure"
	"bytes"
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
	"bufio"
	"strings"
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
	args := os.Args
	torrentName := args[1]
	go startListeningToSeed()
	client.addTorrent(torrentName)

	//TODO: cli here
	for {
		time.Sleep(100 * time.Second)
	}

}

func startListeningToSeed() {
	  // connect to this socket
	fmt.Println("start listening on port 6881 for seeding...")

	// listen on all interfaces
	ln, _ := net.Listen("tcp", ":6882")

	// accept connection on port
	conn, _ := ln.Accept()

	// run loop forever (or until ctrl-c)
	for {
		// will listen for message to process ending in newline (\n)
		message, _ := bufio.NewReader(conn).ReadString('\n')
		// output message received
		fmt.Print("Message Received:", string(message))
		// sample process for string received
		newmessage := strings.ToUpper(message)
		// send new string back to client
		conn.Write([]byte(newmessage + "\n"))
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
				fmt.Println("file already exist!")
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
			fmt.Println("file already exist!")
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
			fmt.Println("\n\n\n========number of blocks in the last piece  : ", numBlocks, "==========\n\n\n")
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

func (torrent *Torrent) divideWork(downloadMap []bool) {
	count := 0

	for i := 0; i < torrent.NumPieces; i++ {
		if downloadMap[i] {
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

			if getBit(peer.RemoteBitMap[bitmapIndex], int(bitShiftIndex)) == 1 {
				for _, block := range piece.BlockMap {
					torrent.PeerWorkMap[peer] = append(torrent.PeerWorkMap[peer], block)
				}

				break

			}
		}

	}
}

func (c *Client) addTorrent(filename string) {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile(filename)
	fmt.Println("===== checking how many files")
	fmt.Println(metaInfo.Info.Name)
	fmt.Println(metaInfo.Info.Length)
	fmt.Println(len(metaInfo.Info.Files))
	fmt.Println("===========")

	createFiles(metaInfo)

	torrent := new(Torrent)
	torrent.MetaInfo = metaInfo

	torrent.NumPieces = len(metaInfo.Info.Pieces) / 20
	torrent.PieceSize = metaInfo.Info.PieceLength
	torrent.BlockOffsetMap = make(map[uint32]int64)
	torrent.PeerWorkMap = make(map[*Peer]([]*Block))
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
	torrent.divideWork(downloadMap)

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
		if c.connectToPeer(peer, torrent) {
			validPeers = append(validPeers, peer)
		}
	}
	torrent.PeerList = validPeers
	go keepPeerListAlive(torrent)
	return

}

func (p *Peer) sendKeepAlive() {
	fmt.Println("=== SENDING ALIVE MSG TO PEER ======")
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

func (c *Client) handlePeerConnection(peer *Peer, torrent *Torrent) {
	conn := *peer.Connection

	// 2) SEND INTERESTED MSG
	fmt.Println("sending interested msg to peer ...")
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
	// data["compact"] = "0"
	// data["no_peer_id"]
	// data["event"]

	return data
}

func (torrent *Torrent) get_peer_list(trackerUrl string, data map[string]string) []*Peer {
	url := createTrackerQuery(trackerUrl, data)
	torrent.TrackerUrl = trackerUrl

	fmt.Println("\n\n==========  CONTACINT TRACKER ========")
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
		peer.RemotePeerIP = net.IPv4(ip[0], ip[1], ip[2], ip[3]).String()
		fmt.Println("IPS ---------- ")

		fmt.Println(myIP)
		fmt.Println(peer.RemotePeerIP)
		if peer.RemotePeerIP == myIP{
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

	for count < 8 {
		count += 1

		fmt.Println("Conducting handshake to  : ", peerIP, " : ", peerPortNum, "   ......")
		conn, err := net.DialTimeout("tcp", peerIP+":"+strconv.Itoa(int(peerPortNum)), time.Duration(1)*time.Second)
		if err != nil {
			fmt.Println("ERROR IN PEER TCP HANDSHAKE  : ", err)
			peerPortNum += 1
			continue
		}

		// Client transmit first message to server
		firstMsg := createHandShakeMsg("BitTorrent protocol", infohash, c.Id)
		conn.Write(firstMsg)

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

		interestMsg := createInterestMsg()
		_, err = conn.Write(interestMsg)

		if err != nil {
			fmt.Println("error in writing  interest to peer : intesest msg. error  : ", err)
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
		bitMapRecvProtocol := int(bitMapBuf[4])

		peer.RemoteBitMap = bitMapBuf[5 : 5+bitMapRecvLen]
		fmt.Println("RECVED bitfield message complete...", bitMapRecvLen, bitMapRecvProtocol)
		fmt.Println(bitMapBuf[5 : 5+bitMapRecvLen])
		fmt.Println("handshake complete...\n\n\n\n\n")

		return true

	}

	return false
}

func (p *Peer) sendRequestMessage(b *Block) {
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
	binary.BigEndian.PutUint32(tmp, uint32(indx))
	data = append(data, tmp...)
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
