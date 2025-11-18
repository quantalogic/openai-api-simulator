package nanochat

import (
	"context"
	"testing"
	"time"
)

func TestNewPythonEngine(t *testing.T) {
	engine := NewPythonEngine("/tmp/test-model")

	if engine.URL() != "http://127.0.0.1:8081" {
		t.Errorf("Expected URL 'http://127.0.0.1:8081', got '%s'", engine.URL())
	}

	if engine.IsRunning() {
		t.Error("New engine should not be running")
	}
}

func TestPythonEngineIsRunning(t *testing.T) {
	engine := NewPythonEngine("/tmp/test-model")

	if engine.IsRunning() {
		t.Error("Engine should not be running initially")
	}

	// Note: Can't test Start() without Python and inference_server.py available
	// This would be tested in integration tests
}

func TestChatMessageStructure(t *testing.T) {
	msg := ChatMessage{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", msg.Content)
	}
}

func TestChatCompletionRequest(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Content: "Test"},
	}

	temp := float32(0.7)
	maxTokens := 512
	topK := 50

	req := &ChatCompletionRequest{
		Messages:    messages,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopK:        &topK,
	}

	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}

	if req.Temperature == nil {
		t.Error("Temperature should not be nil")
	}

	if *req.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %v", *req.Temperature)
	}
}

func TestCompletionToken(t *testing.T) {
	token := CompletionToken{
		Token: "Hello",
		Done:  false,
		Error: "",
	}

	if token.Token != "Hello" {
		t.Errorf("Expected token 'Hello', got '%s'", token.Token)
	}

	if token.Done {
		t.Error("Token should not be marked as done")
	}

	errorToken := CompletionToken{
		Token: "",
		Done:  false,
		Error: "Test error",
	}

	if errorToken.Error != "Test error" {
		t.Errorf("Expected error 'Test error', got '%s'", errorToken.Error)
	}
}

func TestStreamResponseCollectTokens(t *testing.T) {
	// Create a channel with test tokens
	ch := make(chan CompletionToken, 3)
	ch <- CompletionToken{Token: "Hello", Done: false}
	ch <- CompletionToken{Token: " world", Done: false}
	ch <- CompletionToken{Token: "!", Done: true}
	close(ch)

	sr := &StreamResponse{channel: ch}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := sr.CollectTokens(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "Hello world!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestStreamResponseError(t *testing.T) {
	// Create a channel with an error token
	ch := make(chan CompletionToken, 2)
	ch <- CompletionToken{Token: "Partial", Done: false}
	ch <- CompletionToken{Token: "", Done: false, Error: "Test error"}
	close(ch)

	sr := &StreamResponse{channel: ch}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sr.CollectTokens(ctx)
	if err == nil {
		t.Error("Expected error from stream")
	}

	if err.Error() != "token error: Test error" {
		t.Errorf("Expected 'token error: Test error', got '%s'", err.Error())
	}
}

func TestPIDWhenNotRunning(t *testing.T) {
	engine := NewPythonEngine("/tmp/test-model")
	pid := engine.PID()

	if pid != 0 {
		t.Errorf("Expected PID 0 when not running, got %d", pid)
	}
}
