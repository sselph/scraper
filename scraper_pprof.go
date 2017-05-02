// +build pprof

package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func init() {
	log.Printf("Starting PPROF")
	go http.ListenAndServe(":8080", nil)
}
