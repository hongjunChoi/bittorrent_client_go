package main

import (
	"./bencode-go"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

func main() {
	peerId := url.QueryEscape("asdf")
	fmt.Println(peerId)

	file, er := os.Open("trial.torrent")
	if er != nil {
		// return false
		fmt.Println(er)
	}
	defer file.Close()

	// Decode bencoded metainfo file.
	fileMetaData, er := bencode.Decode(file)
	if er != nil {
		fmt.Println(er)
	}
	metaInfoMap, _ := fileMetaData.(map[string]interface{})

	for k, _ := range metaInfoMap {
		fmt.Printf("key[%s] ", k)
	}

	fmt.Println(metaInfoMap["url-list"])
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

func createTrackerQuery(baseUrl string, data map[string]string) string {
	params := url.Values{}
	for k, v := range data {
		params.Add(k, v)
	}

	finalUrl := baseUrl + params.Encode()
	return finalUrl
}
