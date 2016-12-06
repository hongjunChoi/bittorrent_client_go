package main

import (
	// "./bencode-go"
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
)

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("trial.torrent")

	data := parseMetaInfo(metaInfo)
	peerId := url.QueryEscape("asdf")
	data["peer_id"] = peerId

	peerId := url.QueryEscape(generatePeerId())
	fmt.Println(len(peerId))
	return
}

//TODO: COMPLETE THIS PART
func parseMetaInfo(info MetaInfo) map[string]string {
	data := make(map[string]string)
	data["info_hash"] = info.InfoHash
	data["port"] = "6881"
	data["uploaded"] = "0"
	data["downloaded"] = "0"
	data["left"] = "0"
	data["compact"] = "0"
	// data["no_peer_id"]
	// data["event"]

}

func get_peer_list(trackerUrl string, data map[string]string) []string {
	url := createTrackerQuery(trackerUrl, data)
	resp, err := http.Get(url)

	if err != nil {
		// handle error
		fmt.Println("====  error in getting resp from tracker server  ====")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	print(body)

	return make([]string, 1)
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
	params := url.Values{}
	for k, v := range data {
		params.Add(k, v)
	}

	finalUrl := baseUrl + params.Encode()
	return finalUrl
}
