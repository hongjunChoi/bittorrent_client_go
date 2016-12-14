package main

import (
	"crypto/sha1"
	"math/rand"
	"strconv"
	"time"
	"os"
	"net/http"
	"io/ioutil"
	"fmt"
	"strings"
)

func generatePeerId() string {
	t := time.Now().Unix()
	return strconv.FormatInt(t, 10) + RandStringBytes(10)

}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func get_external_IP() string{
	resp, err := http.Get("http://checkip.amazonaws.com/")
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer resp.Body.Close()
	if b, err := ioutil.ReadAll(resp.Body); err == nil {
	    return strings.TrimSpace(string(b))
	} 
	fmt.Println("unable to retrieve external IP")
	return ""
}
func getPeerIndex(torrent *Torrent, peer *Peer) int {
	for i, p := range torrent.PeerList {
		if p == peer {
			return i
		}
	}
	return 0
}

func checkHash(data []byte, hash string) bool {
	sha1 := (sha1.Sum(data))
	sha1String := make([]byte, len(sha1))
	for i := 0; i < len(sha1); i++ {
		sha1String[i] = sha1[i]
	}

	return string(sha1String) == hash

}
