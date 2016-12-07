package main

import (
	bencode "./bencode-go"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

type Peer struct {
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
}

func main() {

	client := createClient()
	client.addTorrent("data/hamlet.torrent")

	//TODO: cli here
	for {

	}

}

func createClient() *Client {
	client := new(Client)
	client.Id = url.QueryEscape(generatePeerId())
	return client
}

func (c *Client) addTorrent(filename string) {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile(filename)

	torrent := new(Torrent)
	torrent.NumPieces = metaInfo.Info.PieceLength
	torrent.initBitMap()

	trackerUrl := metaInfo.Announce

	data := parseMetaInfo(metaInfo)
	data["peer_id"] = c.Id
	peerList := get_peer_list(trackerUrl, data)
	torrent.PeerList = peerList
	torrent.InfoHash = metaInfo.InfoHash

	c.TorrentList = append(c.TorrentList, torrent)

	//TODO: perhaps make this async or move to other func
	for _, peer := range peerList {
		go c.handlePeerConnection(peer, torrent)
	}

}

func (c *Client) handlePeerConnection(peer *Peer, torrent *Torrent) {
	//IF handshake filed.
	if !c.connectToPeer(peer, torrent) {
		//TODO: delete that peer struct pointer from torrent
		fmt.Println("hand shake failed...")
		return
	}
	conn := *peer.Connection

	bitMapMsg := createBitMapMsg(torrent)
	conn.Write(bitMapMsg)

	bitMapBuf := make([]byte, 256) // big buffer

	_, err := conn.Read(bitMapBuf)

	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return 
	}

	bitMapRecvLen := binary.BigEndian.Uint32(bitMapBuf[:4])
	bitMapRecvProtocol := int(bitMapBuf[4])
	fmt.Println(bitMapBuf)
	fmt.Println("bitfeild message complete...", bitMapRecvLen, bitMapRecvProtocol)
	//TODO: READ INCOMING MSG FROM PEER AND ACT ACCORDINGLY


	for {
		buf := make([]byte, 256)
		_, err := conn.Read(buf)

		if err != nil && err != io.EOF {
			fmt.Println("read error from peer..  :", err)
			return
		}

		//ACT ACCORDINGLY
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
	// peer.Connection = &conn
}

// func (c *Client) createBitMapMsg() []byte {
// 	//torrent - bitmap
// 	//torrent - peers
// 	// peer --> torrent --> bitmap

// 	// peer (torrent1, torrent2)
// }

func createBitMapMsg(t *Torrent) []byte {
	data := make([]byte, 0)
	data = append(data, uint8(t.NumPieces))
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
