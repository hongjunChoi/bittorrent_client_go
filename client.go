package main

import (
	"fmt"
)

func main() {

	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("trial.torrent")
	fmt.Println(metaInfo.Announce)
	fmt.Println(metaInfo.AnnounceList)
	return
}
