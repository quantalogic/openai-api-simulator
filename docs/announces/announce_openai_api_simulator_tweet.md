# Announcement: OpenAI API Simulator — tweet + thread

A short, viral-ready announcement to share this repository on Twitter/X and other social platforms.

## Primary Tweet (single tweet)

🤯 Tired of flaky, expensive LLM dev cycles? Run an OpenAI-compatible chat model *locally* — same API, deterministic outputs, streaming SSE & tool-call emulation. OpenAI API Simulator is a single-binary dev server for predictable tests & local UIs. Try it: [quantalogic/openai-api-simulator](https://github.com/quantalogic/openai-api-simulator) 🚀 #LocalAI #DevTools #OpenSource

---

## Short tweet (shorter variant)

Local LLM dev without the surprises. OpenAI-compatible, SSE streaming, deterministic seeding. One binary. No cloud. [quantalogic/openai-api-simulator](https://github.com/quantalogic/openai-api-simulator) 🔁 #AI #DevTools

---

## 3-Tweet Thread (for more context)

1/ Want to iterate on chat UIs, tools, or CI flows without spending a cent on API calls? Meet OpenAI API Simulator — a compact, deterministic, OpenAI-compatible Chat Completion simulator you can run locally. 🎛️

2/ Built-in features: SSE streaming with OpenAI `data:` events, structured JSON generation, tool-call simulation, deterministic seeding, and quick Docker support for integration tests. Great for end-to-end UI tests. ⚙️

3/ Try it in seconds:
```bash
git clone https://github.com/quantalogic/openai-api-simulator && cd openai-api-simulator
make build
./server -port 3080
curl -X POST http://localhost:3080/v1/chat/completions -H "Content-Type: application/json" -d '{"model":"gpt-sim-1","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

---

## Tweet assets & alt text suggestions

- Demo GIF: `./demo.gif` — highlight streaming token chunks, `[DONE]` sentinel.
- Alt text: "Demo of the OpenAI API Simulator showing streaming chat completions in a terminal with `data: <json>` SSE events and `[DONE]`."
- Suggested header image: a screenshot of Open Web UI connected to the simulator.

---

## Hashtags & mentions

- Hashtags: #OpenSource #LocalAI #DevTools #AI #OpenAI #Testing
- Mentions: If appropriate, tag `@openwebui` for the demo integration or any maintainers.

---

## Suggested replies to spark engagement

- "Try it in your CI — you’ll never flake on flaky tests again." ✅
- "Works great with Open Web UI and can replace expensive API calls during dev." 💸🔁
- "Want stricter JSON validation? See ADR 0001 for future enhancements." 📚

---

## Notes for maintainers

- The tweet link points to the GitHub repo root — ensure README contains the demo & quick start (it already does).
- Add the demo GIF to the tweet or attach to a seeded thread post for visual engagement.
- This file is added under `docs/announces` so it can be copied to community channels or used for scheduled posts.
