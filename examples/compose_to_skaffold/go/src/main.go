package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 8080, "Port number to use")
	ip := flag.String("ip", "127.0.0.1", "IP of interface to listen on")
	flag.Parse()
	// simple handler on the default mux
	http.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
		log.Default().Println("received request")
		_, err := writer.Write([]byte("Hello there!"))
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(err.Error()))
		}
	})
	listenAddr := fmt.Sprintf("%s:%d", *ip, *port)
	log.Default().Printf("listening on: %s \n", listenAddr)
	err := http.ListenAndServe(listenAddr, nil)
	if err != http.ErrServerClosed {
		log.Default().Fatalf("encountered error: %v", err)
	}
	os.Exit(0)
}
