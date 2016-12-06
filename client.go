package main

import (
	"fmt"
)

func main() {

	metaInfo := new(MetaInfo)
	metaInfo.ReadTorrentMetaInfoFile("trial.torrent")
	fmt.Println(metaInfo.Info.Files)
	return
}
