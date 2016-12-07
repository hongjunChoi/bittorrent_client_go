package main

import (
	bencode "./bencode-go"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
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
}

type Client struct {
	Id          string  // self peer id
	Peers       []*Peer //MAP of remote peer id : peer data
	TorrentList []*Torrent
	FunctionMap map[int]func(*Peer, *Torrent, []byte)
}

func main() {

	client := createClient()
	client.addTorrent("data/trial3.torrent")

	//TODO: cli here
	for {

	}

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

func (c *Client) addTorrent(filename string) {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile(filename)

	fmt.Println("====== torrent file info =========")
	fmt.Println(metaInfo.Info)
	fmt.Println("===================================")

	torrent := new(Torrent)
	torrent.NumPieces = len(metaInfo.Info.Pieces) / 20
	torrent.PieceSize = metaInfo.Info.PieceLength
	torrent.BlockOffsetMap = make(map[uint32]int64)
	torrent.PeerWorkMap = make(map[*Peer]([]*Block))

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
	c.TorrentList = append(c.TorrentList, torrent)

	c.peerListHandShake(torrent, peerList)
	count := 0

	//CREATE ALL PIECE AND BLOCKS IN TORRENT AND STORE IN STRUCT
	for i := 0; i < torrent.NumPieces; i++ {
		piece := new(Piece)
		piece.Index = i
		numBlocks := int(math.Ceil(float64(torrent.PieceSize / BLOCKSIZE)))
		piece.BitMap = make([]byte, int(math.Ceil(float64(numBlocks/8))))

		for j := 0; j < numBlocks; j++ {
			b := new(Block)
			b.Offset = j * BLOCKSIZE
			b.Data = make([]byte, BLOCKSIZE)

			piece.BlockMap[uint32(b.Offset)] = b
		}
		torrent.PieceMap[uint32(i)] = piece
	}

	//DIVIDE WORK AMONG PEERS HERE
	for _, piece := range torrent.PieceMap {
		for _, block := range piece.BlockMap {
			p := torrent.PeerList[(count % len(torrent.PeerList))]
			torrent.PeerWorkMap[p] = append(torrent.PeerWorkMap[p], block)
			count += 1
		}
	}

	for _, peer := range peerList {
		go c.handlePeerConnection(peer, torrent)
	}

}

func (c *Client) peerListHandShake(torrent *Torrent, peerList []*Peer) {
	for _, peer := range peerList {
		//IF handshake filed.
		if !c.connectToPeer(peer, torrent) {
			//delete that peer struct pointer from torrent
			deleteIndex := getPeerIndex(torrent, peer)
			torrent.PeerList = append(torrent.PeerList[:deleteIndex], torrent.PeerList[deleteIndex+1:]...)
			fmt.Println("hand shake failed...")
			return
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
		time.Sleep(120 * time.Second)
	}
}

func (c *Client) handlePeerConnection(peer *Peer, torrent *Torrent) {
	conn := *peer.Connection

	bitMapBuf := make([]byte, 256)
	numRecved, err := conn.Read(bitMapBuf)

	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return
	}

	// 	// bitMapBuf = bitMapBuf[0:numRecved]
	// 	// bitMapRecvLen := binary.BigEndian.Uint32(bitMapBuf[:4])
	// 	// bitMapRecvProtocol := int(bitMapBuf[4])

	// 	fmt.Println(bitMapBuf)

	// 	fmt.Println("---sending bitmap buf")
	// 	bitMapMsg := createBitMapMsg(torrent)
	// 	fmt.Println(bitMapMsg)
	// 	conn.Write(bitMapMsg)

	bitMapBuf = bitMapBuf[0:numRecved]
	bitMapRecvLen := binary.BigEndian.Uint32(bitMapBuf[:4]) - 1
	bitMapRecvProtocol := int(bitMapBuf[4])
	fmt.Println("RECVED bitfield message complete...", bitMapRecvLen, bitMapRecvProtocol)
	fmt.Println(bitMapBuf[5 : 5+bitMapRecvLen])

	// 2) SEND INTERESTED MSG

	interestMsg := createInterestMsg()
	conn.Write(interestMsg)
	fmt.Println("sending interested msg to peer ...")
	fmt.Println(interestMsg)

	fmt.Println("waiting for  response after sending our interested msg..")

	for {
		fmt.Println("waiting ... ")
		buf := make([]byte, 2048)
		numRecved, err = conn.Read(buf)

		if err != nil && err != io.EOF {
			fmt.Println("read error from peer..  :", err)
			return
		}

		//IF RECVED MSG IS NOT KEEP ALIVE
		if numRecved > 0 {
			msgLen := binary.BigEndian.Uint32(buf[0:4]) - 1
			recvId := buf[4]
			payload := make([]byte, 0)
			if msgLen > 0 {
				payload = buf[5 : 5+msgLen]
			}

			fmt.Println("....  RCVD ....")
			fmt.Println(recvId)
			fmt.Println(payload)
			fmt.Println(".......")

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

	fmt.Println("Conducting handshake to  : ", peerIP, " : ", peerPortNum, "   ......")

	conn, err := net.Dial("tcp", peerIP+":"+strconv.Itoa(int(peerPortNum)))
	if err != nil {
		fmt.Println("====   ERROR IN PEER HANDSHAKE   =====")
		fmt.Println(err)
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
		conn.Close()
		return false
	}

	fmt.Println("handshake complete...", recvMsg)
	peer.Connection = &conn
	return true
}

func createRequestMsg() []byte {
	data := make([]byte, 0)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(13))
	data = append(data, tmp...)
	data = append(data, uint8(6))
	binary.BigEndian.PutUint32(tmp, uint32(0))
	data = append(data, tmp...)
	binary.BigEndian.PutUint32(tmp, uint32(0))
	data = append(data, tmp...)
	binary.BigEndian.PutUint32(tmp, uint32(1024))
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
