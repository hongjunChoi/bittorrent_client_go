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
	BLOCKSIZE      = 1024
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
	CurrentBlock     int
}

type Client struct {
	Id          string  // self peer id
	Peers       []*Peer //MAP of remote peer id : peer data
	TorrentList []*Torrent
	FunctionMap map[int]func(*Peer, *Torrent, []byte)
}

func main() {

	// // Open a new file for writing only
	// file, err := os.OpenFile(
	//     "test",
	//     os.O_WRONLY|os.O_TRUNC,
	//     0666,
	// )
	// if err != nil {
	//     fmt.Println(err)
	// }
	// defer file.Close()

	// // Write bytes to file
	// file.Seek(0, 0)
	// byteSlice := []byte("Bytes!\n")
	// bytesWritten, err := file.Write(byteSlice)
	// if err != nil {
	//     fmt.Println(err)
	// }
	// fmt.Println("Wrote %d bytes.\n", bytesWritten)

	// file.Seek(20, 0)
	// bytesWritten, err = file.Write(byteSlice)
	// if err != nil {
	//     fmt.Println(err)
	// }
	// fmt.Println("Wrote %d bytes.\n", bytesWritten)
	client := createClient()
	args := os.Args
	torrentName := args[1]
	client.addTorrent(torrentName)

	//TODO: cli here
	for {
		time.Sleep(100 * time.Second)
	}

}

//returns a list of boolean. if bool at index i is true, than piece [i] is already downloaded
func (c *Client) checkAlreadyDownloaded(torrent *Torrent) []bool {
	hash := torrent.MetaInfo.Info.Pieces
	numPiece := len(hash) / 20
	bitMap := make([]bool, numPiece)

	for i := 0; i < numPiece; i++ {
		piece := torrent.PieceMap[uint32(i)]

		for j := 0; j < len(piece.FileMap); j++ {
			filename := piece.FileMap[j].FileName
			startIndex := piece.FileMap[j].startIndx
			endIndex := piece.FileMap[j].endIndx

			file, err := os.Open(filename)
			if err != nil {
				fmt.Println("==== ERROR READING FILE ====")
				bitMap[i] = false
				break
			}

			b := make([]byte, endIndex-startIndex+1)
			n := 0
			n, err = file.ReadAt(b, startIndex)

			if err != nil {
				fmt.Println("======    ERROR IN READING FILE TO CHECK ALREADY EXIST     ========")
				fmt.Println(err)
				bitMap[i] = false
				break
			}

			if checkHash(b[0:n], hash[i*20:(i+1)*20]) {
				bitMap[i] = true
			} else {
				bitMap[j] = false
				break
			}
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
func (c *Client) addTorrent(filename string) {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile(filename)
	fmt.Println("===== checking how many files")
	fmt.Println(metaInfo.Info.Name)
	fmt.Println(metaInfo.Info.Length)

	torrent := new(Torrent)
	torrent.MetaInfo = metaInfo

	numFiles := len(metaInfo.Info.Files)
	if numFiles > 0 {
		fmt.Println("====== ", numFiles, " files ========")
		for i := 0; i < numFiles; i++ {
			fmt.Println(metaInfo.Info.Files[i].Path)
			fmt.Println(metaInfo.Info.Files[i].Length)
			//Create File
			// Open a new file for writing only
			file, err := os.OpenFile(
				generateFilePath(metaInfo.Info.Files[i].Path),
				os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
				0666,
			)
			if err != nil {
				fmt.Println(err)
			}

			file.Close()
			// if i == 0 {
			// 	torrent.FileList[i] = file
			// }
		}
	}

	// torrent.FileTrial.Seek(50, 0)
	// torrent.FileTrial.Write([]byte("aaaa"))
	// torrent.FileTrial.Seek(0, 0)
	// torrent.FileTrial.Write([]byte("aaaa"))
	torrent.NumPieces = len(metaInfo.Info.Pieces) / 20
	torrent.PieceSize = metaInfo.Info.PieceLength
	torrent.BlockOffsetMap = make(map[uint32]int64)
	torrent.PeerWorkMap = make(map[*Peer]([]*Block))
	torrent.MetaInfo = metaInfo

	for i := 0; i < torrent.NumPieces; i++ {
		torrent.BlockOffsetMap[uint32(i)] = 0
	}

	torrent.initBitMap()

	trackerUrl := metaInfo.Announce

	data := parseMetaInfo(metaInfo)
	data["peer_id"] = c.Id

	peerList := get_peer_list(trackerUrl, data)
	torrent.PeerList = peerList
	torrent.InfoHash = metaInfo.InfoHash
	torrent.PieceMap = make(map[uint32]*Piece)

	c.TorrentList = append(c.TorrentList, torrent)



	//CREATE ALL PIECE AND BLOCKS IN TORRENT AND STORE IN STRUCT
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
		for k := 0; k < len(fileDictList); k++ {
			fileInfo := fileDictList[k]
			// fmt.Println("check")
			if byteIndx < cursor+fileInfo.Length {
				// start writing to file
				file := new(File)
				file.FileName = generateFilePath(fileInfo.Path)
				file.startIndx = byteIndx - cursor

				bytesLeftInFile := cursor + fileInfo.Length - byteIndx
				piece.FileMap = append(piece.FileMap, file)

				if torrent.PieceSize - pieceOffset <= bytesLeftInFile {
					file.endIndx = file.startIndx + torrent.PieceSize - pieceOffset
					// torrent.FileList[i].Seek(, whence)
					break
				} else {
					// fmt.Println("to next file!")
					file.endIndx = file.startIndx + bytesLeftInFile
					pieceOffset += bytesLeftInFile
					byteIndx = cursor + fileInfo.Length
				}
			}
			cursor += fileInfo.Length
		}
		fmt.Println("fileMap: ")
		for j:= 0; j<len(piece.FileMap); j++ {
			fmt.Println(piece.FileMap[j])
		}
		for j := 0; j < numBlocks; j++ {
			b := new(Block)
			b.Offset = j * BLOCKSIZE
			b.PieceIndex = i
			b.Data = make([]byte, BLOCKSIZE)
			b.Size = BLOCKSIZE

			if j == numBlocks-1 {
				b.Size = int(torrent.PieceSize) - b.Offset
			}

			piece.BlockMap[uint32(b.Offset)] = b
		}
		torrent.PieceMap[uint32(i)] = piece
	}
	
	c.peerListHandShake(torrent)

	if len(torrent.PeerList) == 0 {
		fmt.Println("all peers failed handshake...")
		return
	}

	//DIVIDE WORK AMONG PEERS HERE
	downloadMap := c.checkAlreadyDownloaded(torrent)

	for i := 0; i < torrent.NumPieces; i++ {
		if downloadMap[i] {
			continue
		}

		pieceIndex := i
		piece := torrent.PieceMap[uint32(pieceIndex)]
		// pieceCount := torrent.NumPieces
		count := 0

		for {
			peer := torrent.PeerList[count%(len(torrent.PeerList))]
			count += 1

			//if peer bit map in index of currnet piece is 1 then give all piece blocks to peer
			bitShiftIndex := 7 - (pieceIndex % 8)
			bitmapIndex := pieceIndex / 8

			// fmt.Println("======  dividing work here =====")
			// fmt.Println(pieceCount)
			// fmt.Println(pieceIndex)
			// fmt.Println(bitmapIndex)
			// fmt.Println(bitShiftIndex)
			// fmt.Println("====================")

			if getBit(peer.RemoteBitMap[bitmapIndex], int(bitShiftIndex)) == 1 {
				for _, block := range piece.BlockMap {
					torrent.PeerWorkMap[peer] = append(torrent.PeerWorkMap[peer], block)
				}

				break
			}
		}

	}

	for _, peer := range torrent.PeerList {
		go c.handlePeerConnection(peer, torrent)
	}

}

func (c *Client) peerListHandShake(torrent *Torrent) {
	for _, peer := range torrent.PeerList {
		//IF handshake filed.
		if !c.connectToPeer(peer, torrent) {
			//delete that peer struct pointer from torrent
			deleteIndex := getPeerIndex(torrent, peer)
			torrent.PeerList = append(torrent.PeerList[:deleteIndex], torrent.PeerList[deleteIndex+1:]...)
			fmt.Println("hand shake failed...")
		}
	}
	go keepPeerListAlive(torrent)
}

func (p *Peer) sendKeepAlive() {
	conn := *p.Connection
	conn.Write(make([]byte, 0))
}

//sends KEEP ALIVE to each peer periodically
func keepPeerListAlive(torrent *Torrent) {
	for {
		for _, p := range torrent.PeerList {
			p.sendKeepAlive()
		}
		time.Sleep(30 * time.Second)
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

	fmt.Println(interestMsg)
	fmt.Println("waiting for  response after sending our interested msg..")

	for {
		fmt.Println("waiting ... ")
		buf := make([]byte, 8192)
		numRecved := 0
		numRecved, err = conn.Read(buf)

		if err != nil && err != io.EOF {
			fmt.Println("read error from peer..  :", err)
			return
		}

		buf = buf[0:numRecved]

		//IF RECVED MSG IS NOT KEEP ALIVE
		if numRecved > 0 && buf[4] != 0 {

			recvId := buf[4]
			msgLen := binary.BigEndian.Uint32(buf[0:4]) - 1
			payload := make([]byte, 0)

			if msgLen > 0 {

				payload = buf[5 : 5+msgLen]
			}

			// fmt.Println("======  RECEIVED PAYLOAD ====")
			// fmt.Println(recvId)
			// fmt.Println(payload)
			// fmt.Println("===========")

			// STATE MACHINE HERE
			c.FunctionMap[int(recvId)](peer, torrent, payload)
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
	// data["compact"] = "0"
	// data["no_peer_id"]
	// data["event"]
	// a := [1]map[string]string{data}

	return data
}

func get_peer_list(trackerUrl string, data map[string]string) []*Peer {
	url := createTrackerQuery(trackerUrl, data)
	resp, err := http.Get(url)

	if err != nil {
		// handle error
		fmt.Println("\n\n====  error in getting resp from tracker server  ====")
		fmt.Println(err)
		fmt.Println("========\n\n")
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

	fmt.Println("==== PEER DATA FROM TRACKER =====")
	fmt.Println(peers)
	fmt.Println("==========")

	peerData := make([]*Peer, len(peers)/6)

	for i := 0; i < len(peers)/6; i++ {
		index := i * 6
		ip := peers[index : index+4]
		port := peers[index+4 : index+6]

		peer := new(Peer)

		peer.RemotePeerIP = net.IPv4(ip[0], ip[1], ip[2], ip[3]).String()
		peer.RemotePeerPort = binary.BigEndian.Uint16(port)
		peer.RemotePeerId = ""
		peer.BlockQueue = lane.NewQueue()
		peer.CurrentBlock = 0

		peerData[i] = peer
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

	for peerPortNum < 6889 {
		fmt.Println("Conducting handshake to  : ", peerIP, " : ", peerPortNum, "   ......")
		conn, err := net.DialTimeout("tcp", peerIP+":"+strconv.Itoa(int(peerPortNum)), time.Duration(1)*time.Second)
		if err != nil {
			fmt.Println("====   ERROR IN PEER HANDSHAKE   =====")
			fmt.Println(err)
			peerPortNum += 1
			return false
		}

		// Client transmit first message to server
		firstMsg := createHandShakeMsg("BitTorrent protocol", infohash, c.Id)
		conn.Write(firstMsg)

		buf := make([]byte, 256)
		_, err = conn.Read(buf)

		if err != nil && err != io.EOF {
			fmt.Println("read error:", err)
			return false
		}

		recvMsgLen := uint8(buf[0])
		recvMsg := string(buf[1 : 1+recvMsgLen])
		recvInfoHash := string(buf[9+recvMsgLen : 29+recvMsgLen])
		recvPeerId := string(buf[29+recvMsgLen : 49+recvMsgLen])

		//CHECK for inconsistent info hash  or peer id
		if peer.RemotePeerId == "" {
			peer.RemotePeerId = recvPeerId
		}

		if peer.RemotePeerId != recvPeerId || infohash != recvInfoHash {
			fmt.Println("=== INCORRECT VALUE FROM HANDSHAKE ======")
			fmt.Println(peer.RemotePeerId)
			fmt.Println(recvPeerId)
			fmt.Println(infohash)
			fmt.Println(recvInfoHash)
			fmt.Println("================")
			conn.Close()
			return false
		}

		fmt.Println("handshake complete...", recvMsg)
		peer.Connection = &conn

		bitMapBuf := make([]byte, 256)
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
		return true
	}
	return false
}

func (p *Peer) sendRequestMessage(b *Block) {
	data := createRequestMsg(b.PieceIndex, b.Offset, b.Size)
	(*p.Connection).Write(data)
	p.BlockQueue.Enqueue(b)
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
	fmt.Println(data)
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
