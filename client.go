package main

import (
	"fmt"
	"./bencode-go"
	"os"
)

func main() {
	fmt.Println("----")
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
