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

	return
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
		c.connectToPeer(peer, torrent, metaInfo.InfoHash)
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
	peerDictData, er := bencode.Decode(r)
	if er != nil {
		fmt.Println(er)
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
	// params := url.Values{}
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

func (c *Client) connectToPeer(peer *Peer, torrent *Torrent, infohash string) {
	peerIP := peer.RemotePeerIP
	peerPortNum := peer.RemotePeerPort

	conn, err := net.Dial("tcp", peerIP+":"+strconv.Itoa(int(peerPortNum)))
	if err != nil {
		fmt.Println("ERROR IN PEER HANDSHAKE")
		fmt.Println(err)
		return
	}

	// Client transmit first message to server
	firstMsg := createHandShakeMsg("BitTorrent protocol", infohash, c.Id)
	conn.Write(firstMsg)

	buf := make([]byte, 256) // big buffer

	_, err = conn.Read(buf)

	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return
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
		return
	}

	fmt.Println("handshake complete...", recvMsg)
	peer.Connection = &conn

	bitMapMsg := createBitMapMsg(torrent)
	conn.Write(bitMapMsg)

	bitMapBuf := make([]byte, 256) // big buffer

	_, err = conn.Read(bitMapBuf)

	if err != nil && err != io.EOF {
		fmt.Println("read error:", err)
		return
	}
	bitMapRecvLen := binary.BigEndian.Uint32(bitMapBuf[:4])
	bitMapRecvProtocol := int(bitMapBuf[4])
	// recvInfoHash := string(buf[9+recvMsgLen : 29+recvMsgLen])
	// recvPeerId := string(buf[29+recvMsgLen : 49+recvMsgLen])
	fmt.Println(bitMapBuf)
	fmt.Println("bitfeild message complete...", bitMapRecvLen, bitMapRecvProtocol)
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
