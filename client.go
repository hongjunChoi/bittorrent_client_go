package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"bytes"
	"./bencode-go"
	// "strconv"
	"encoding/binary"
	// "string"
)

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("data/hamlet.torrent")
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
	// responseData = bencode
// <<<<<<< HEAD
// 	// Decode bencoded metainfo file.
// 	// peerDict, er := bencode.Decode(string(body))
// 	// if er != nil {
// 	// 	return false
// 	// }

// 	// // fileMetaData is map of maps of... maps. Get top level map.
// 	// metaInfoMap, ok := peerDict.([]interface{})
// 	// if !ok {
// 	// 	return false
// 	// }
// 	// fmt.Println(metaInfoMap)
// =======
// >>>>>>> f3f50255e80002216717f880cdc07ff5cdfbd8c5

	fmt.Println("\n\n======= body =======\n")
	r := bytes.NewReader(body)
	peerDictData, er := bencode.Decode(r)
	if er != nil {
		fmt.Println(er)
	}
	peerDict, _ := peerDictData.(map[string]interface{})
	fmt.Println(peerDict)
	fmt.Println("\n==============",[]byte(peerDict["peers"].(string)))
	peers := []byte(peerDict["peers"].(string))
	ip := peers[0:4]
	fmt.Println(ip)
	// buf := bytes.NewBuffer(ip)
	// datas, _ := binary.ReadVarint(buf)
	// x := bytes.NewReader([]byte(peerDict["peers"].(string)))
	// d, _ := bencode.Decode(x)
	// peerd, _ := d.(map[string]interface{})

	fmt.Println(net.IPv4(ip[0],ip[1],ip[2],ip[3]))
	// r = bytes.NewReader(peerDict["peers"])
	// peer, _ := bencode.Decode(r)
	// peerDict, _ = peer.(map[string]interface{})
	// fmt.Println("\n==============",peerDict)
	port := peers[4:6]
	// var v uint32
	// err = binary.Read(bytes.NewReader(port),binary.LittleEndian, &v)
	// if err != nil {
	// // return 0, err
	// 	fmt.Println(err)
	// }
	// // return v, nil
		fmt.Println(binary.BigEndian.Uint16(port))

	

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
