package main

import (
	// "./bencode-go"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("trial.torrent")
	fmt.Println(metaInfo.Info.Files)

	peerId := url.QueryEscape("asdf")
	fmt.Println(peerId)

	return
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
