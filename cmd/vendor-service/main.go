package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var totalHits uint64
var hitsPerSec uint64
var lastHit time.Time

func print() {
	curr := time.Now()
	if diff := curr.Sub(lastHit); diff.Seconds() > 1.0 {
		fmt.Printf("processed - hits:%d total:%d\n", hitsPerSec, totalHits)
		lastHit = curr
		hitsPerSec = 0
	}
}
func sendMessage(w http.ResponseWriter, r *http.Request) {
	//time.Sleep(100 * time.Millisecond)
	message := r.URL.Path
	message = strings.TrimPrefix(message, "/")
	message = "received:" + message
	w.Write([]byte(message))
	atomic.AddUint64(&totalHits, 1)
	atomic.AddUint64(&hitsPerSec, 1)

	print()
}
func main() {
	args := os.Args

	go func() {
		for {
			time.Sleep(1 * time.Second)
			if hitsPerSec > 0 {
				print()
			}
		}
	}()

	http.HandleFunc("/", sendMessage)
	if err := http.ListenAndServe(args[1], nil); err != nil {
		panic(err)
	}
}
