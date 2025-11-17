package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/openai/openai-api-simulator/pkg/streaming"

	"github.com/openai/openai-api-simulator/pkg/server"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the simulator HTTP server on")
	// Default streaming options (jitter & token throttle) that apply when a
	// request does not provide explicit `stream_options`.
	delayMin := flag.Int("stream_delay_min_ms", 0, "Default min per-chunk delay (ms) to simulate jitter when stream_options missing")
	delayMax := flag.Int("stream_delay_max_ms", 0, "Default max per-chunk delay (ms) to simulate jitter when stream_options missing")
	tokensPerSec := flag.Float64("stream_tokens_per_second", 0, "Default token emission rate for streaming chunks; 0 disables throttling")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting OpenAI API Simulator on %s", addr)
	defaults := streaming.StreamOptions{}
	if *delayMin > 0 {
		defaults.DelayMin = time.Duration(*delayMin) * time.Millisecond
	}
	if *delayMax > 0 {
		defaults.DelayMax = time.Duration(*delayMax) * time.Millisecond
	}
	if *tokensPerSec > 0 {
		defaults.TokensPerSecond = *tokensPerSec
	}

	handler := server.NewRouterWithStreamDefaults(defaults)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
