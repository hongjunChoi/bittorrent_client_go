package main

import (
	"math/rand"
	"strconv"
	"time"
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

func getPeerIndex(torrent *Torrent, peer *Peer) int {
	for i, p := range torrent.PeerList {
		if p == peer {
			return i
		}
	}
	return 0
}
