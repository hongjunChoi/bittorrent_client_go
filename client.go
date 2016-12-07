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

type Torrent struct {
	BitMap   []byte
	FileName string
}

type Client struct {
	Id    string  // self peer id
	Peers []*Peer //MAP of remote peer id : peer data
}

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("data/hamlet.torrent")
	torrent := new(Torrent)
	torrent.initBitMap(metaInfo.Info.PieceLength)
	trackerUrl := metaInfo.Announce
	fmt.Println(metaInfo.Info.PieceLength)
	data := parseMetaInfo(metaInfo)

	peerId := url.QueryEscape(generatePeerId())
	data["peer_id"] = peerId

	fmt.Println(len(peerId))
	peerList := get_peer_list(trackerUrl, data)

	client := new(Client)
	client.Peers = peerList
	client.Id = peerId

	for _, peer := range peerList {
		client.connectToPeer(peer, metaInfo.InfoHash)
	}

	return
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
	// fmt.Print(a.)

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

func (c *Client) connectToPeer(peer *Peer, infohash string) {
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

	// bitMapMsg := createBitMapMsg(to)

}

// func (c *Client) createBitMapMsg() []byte {
// 	//torrent - bitmap
// 	//torrent - peers
// 	// peer --> torrent --> bitmap

// 	// peer (torrent1, torrent2)
// }

func createBitMapMsg(numPieces int64, bitMap []byte) []byte {
	data := make([]byte, 0)
	data = append(data, uint8(numPieces))
	data = append(data, uint8(5))
	data = append(data, bitMap...)

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
