package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

func main() {
	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("trial.torrent")

	data := parseMetaInfo(metaInfo)
	peerId := url.QueryEscape("asdf")
	data["peer_id"] = peerId

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

func createTrackerQuery(baseUrl string, data map[string]string) string {
	params := url.Values{}
	for k, v := range data {
		params.Add(k, v)
	}

	finalUrl := baseUrl + params.Encode()
	return finalUrl
}
