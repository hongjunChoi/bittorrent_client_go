package main

import (
	bencode "./bencode-go"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type Peer struct {
	SelfChoking      bool
	SelfInterested   bool
	RemoteChoking    bool
	RemoteInterested bool
	RemotePeerId     string
	RemotePeerIP     string
	RremotePeerPort  uint16
}

type Client struct {
	Id    string          // self peer id
	Peers map[string]Peer //MAP of remote peer id : peer data
}

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("data/hamlet.torrent")
	trackerUrl := metaInfo.Announce
	fmt.Println(metaInfo.Info.PieceLength)
	data := parseMetaInfo(metaInfo)

	peerId := url.QueryEscape(generatePeerId())
	data["peer_id"] = peerId

	fmt.Println(len(peerId))
	peerList := get_peer_list(trackerUrl, data)
	fmt.Println(peerList)

	return
}

//TODO: COMPLETE THIS PART
func parseMetaInfo(info *MetaInfo) map[string]string {
	fmt.Println("===== info hash =====")
	fmt.Println(info.InfoHash)
	fmt.Println(url.QueryEscape(info.InfoHash))
	fmt.Println("=======")

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
		peer.RremotePeerPort = binary.BigEndian.Uint16(port)

		peerData[i] = peer
	}

	return peerData
}

func startTCPConnection(ip string, port string) {

	conn, _ := net.Dial("tcp", ip+":"+port)
	for {
		// read in input from stdin
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Text to send: ")
		text, _ := reader.ReadString('\n')
		// send to socket
		fmt.Fprintf(conn, text+"\n")
		// listen for reply
		message, _ := bufio.NewReader(conn).ReadString('\n')
		fmt.Print("Message from server: " + message)
	}
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

func (c *Client) connectToPeer(peer *Peer) {
	peerIP := peer.RemotePeerIP
	peerPortNum := peer.RremotePeerPort

	conn, err := net.Dial("tcp", peerIP+strconv.Itoa(int(peerPortNum)))
	if err != nil {
		fmt.Println("ERROR IN PEER HANDSHAKE")
		return
	}

	// Client transmit first message to server
	firstMsg := ""
	fmt.Fprintf(conn, firstMsg)

	// for {
	// 	// read in input from stdin
	// 	reader := bufio.NewReader(os.Stdin)
	// 	fmt.Print("Text to send: ")
	// 	text, _ := reader.ReadString('\n')
	// 	// send to socket

	// 	// listen for reply
	// 	message, _ := bufio.NewReader(conn).ReadString('\n')
	// 	fmt.Print("Message from server: " + message)
	// }
}

func createHandShakeMsg(msg string, infohash string, peerId string) []byte {
	msgLen := uint32(len(msg))
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, msgLen)
	return make([]byte, 0)
}
