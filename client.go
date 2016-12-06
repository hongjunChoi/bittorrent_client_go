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
	// "string"
)

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("data/trial.torrent")
	trackerUrl := metaInfo.Announce

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
	// responseData = bencode
	// Decode bencoded metainfo file.
	// peerDict, er := bencode.Decode(string(body))
	// if er != nil {
	// 	return false
	// }

	// // fileMetaData is map of maps of... maps. Get top level map.
	// metaInfoMap, ok := peerDict.([]interface{})
	// if !ok {
	// 	return false
	// }
	// fmt.Println(metaInfoMap)

	fmt.Println("\n\n======= body =======\n")
	fmt.Println(string(body))
	fmt.Println("\n==============")
	var s []string
	return s
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
