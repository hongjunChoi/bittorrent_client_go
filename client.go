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
	"strconv"
)

type Peer struct {
	SelfChoking      bool
	SelfInterested   bool
	RemoteChoking    bool
	RemoteInterested bool
	RemotePeerId     string
	RemotePeerIP     string
	RremotePeerPort  int
}

type Client struct {
	Id    string          // self peer id
	Peers map[string]Peer //MAP of remote peer id : peer data
}

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("data/hamlet.torrent")
	trackerUrl := metaInfo.Announce

	data := parseMetaInfo(metaInfo)
	peerId := url.QueryEscape("ABCDEFGHIJKLMNOPQRST")
	data["peer_id"] = peerId

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

	return data
}

func get_peer_list(trackerUrl string, data map[string]string) []string {
	url := createTrackerQuery(trackerUrl, data)

	fmt.Println("========     URL TO CALL TRACKER    ========")
	fmt.Println(url)
	fmt.Println("===============")

	resp, err := http.Get(url)

	if err != nil {
		// handle error
		fmt.Println("\n\n====  error in getting resp from tracker server  ====")
		fmt.Println(err)
		fmt.Println("========\n\n")
		return make([]string, 0)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	// TODO: parse body with bencode

	reader := bytes.NewReader([]byte(body))

	dict, err := bencode.Decode(reader)
	if err != nil {
		fmt.Println("ERROR : error in decoding bencoded txt to data")
	}

	fmt.Println("\n\n======= body =======\n")
	fmt.Println(dict)
	fmt.Println("\n==============")

	// return resposneData
	return make([]string, 0)
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

	conn, err := net.Dial("tcp", peerIP+strconv.Itoa(peerPortNum))
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

}
