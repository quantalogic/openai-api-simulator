package main

import (
	"flag"
	"log"

	"github.com/quantalogic/openai-api-simulator/internal/nanochat"
)

func main() {
	publicPort := flag.Int("port", 8090, "Public API port")
	llamaPort := flag.Int("llama-port", 8081, "Internal llama.cpp port")
	flag.Parse()

	if err := nanochat.Run(*publicPort, *llamaPort); err != nil {
		log.Fatalf("nanochat failed: %v", err)
	}
}
