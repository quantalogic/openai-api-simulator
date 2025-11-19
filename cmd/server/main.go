package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quantalogic/openai-api-simulator/pkg/streaming"

	"github.com/quantalogic/openai-api-simulator/pkg/server"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the simulator HTTP server on")
	// Default streaming options (jitter & token throttle) that apply when a
	// request does not provide explicit `stream_options`.
	delayMin := flag.Int("stream_delay_min_ms", 0, "Default min per-chunk delay (ms) to simulate jitter when stream_options missing")
	delayMax := flag.Int("stream_delay_max_ms", 0, "Default max per-chunk delay (ms) to simulate jitter when stream_options missing")
	tokensPerSec := flag.Float64("stream_tokens_per_second", 0, "Default token emission rate for streaming chunks; 0 disables throttling")
	defaultResponseLength := flag.String("stream_default_response_length", "", "Optional default response length when unspecified: short|medium|long; empty = infer from messages")

	// SmolLM proxy mode flags
	smollmEnabled := flag.Bool("smollm-enabled", false, "Enable smollm proxy mode")
	smollmUpstreamURL := flag.String("smollm-upstream-url", "http://127.0.0.1:8081", "Upstream llama.cpp server URL")

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
	// Fallback to environment variables for Docker / compose if flags are not
	// explicitly provided. This makes it convenient to configure realistic
	// latency and throughput when running the `docker-compose` stack.
	if defaults.DelayMin == 0 {
		if v := os.Getenv("STREAM_DELAY_MIN_MS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				defaults.DelayMin = time.Duration(n) * time.Millisecond
			}
		}
	}
	if defaults.DelayMax == 0 {
		if v := os.Getenv("STREAM_DELAY_MAX_MS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				defaults.DelayMax = time.Duration(n) * time.Millisecond
			}
		}
	}
	if defaults.TokensPerSecond == 0 {
		if v := os.Getenv("STREAM_TOKENS_PER_SECOND"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				defaults.TokensPerSecond = f
			}
		}
	}
	if *defaultResponseLength == "" {
		if v := os.Getenv("STREAM_DEFAULT_RESPONSE_LENGTH"); v != "" {
			// Normalize to lowercase
			*defaultResponseLength = strings.ToLower(v)
		}
	}

	handler := server.NewRouterWithStreamDefaults(defaults, *defaultResponseLength, *smollmEnabled, *smollmUpstreamURL)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
