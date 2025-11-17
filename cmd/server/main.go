package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/openai/openai-api-simulator/pkg/server"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the simulator HTTP server on")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting OpenAI API Simulator on %s", addr)
	handler := server.NewRouter()
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
