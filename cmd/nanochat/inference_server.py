#!/usr/bin/env python3
"""
SmolLM Inference Server using llama-cpp-python

Provides OpenAI-compatible chat completion API using SmolLM GGUF model.
This server bridges the Go simulator to SmolLM model inference via llama.cpp.

Usage:
    python cmd/nanochat/inference_server.py --port 8081 --model-path ~/.cache/openai-api-simulator/smollm

Features:
    - Streaming Server-Sent Events for token generation
    - Efficient KV cache management via llama.cpp
    - OpenAI-compatible /chat/completions endpoint
    - Fast CPU inference with quantized GGUF models
    - Automatic model download and caching
"""

import argparse
import asyncio
import json
import logging
import os
import sys
from pathlib import Path
from typing import AsyncGenerator, Optional, List
from urllib.request import urlopen
from urllib.error import URLError

try:
    from fastapi import FastAPI, HTTPException
    from fastapi.middleware.cors import CORSMiddleware
    from fastapi.responses import StreamingResponse
    from pydantic import BaseModel
    import uvicorn
except ImportError:
    print("Error: Required packages not installed.")
    print("Install with: pip install fastapi uvicorn llama-cpp-python pydantic")
    sys.exit(1)

try:
    from llama_cpp import Llama
except ImportError:
    print("Error: llama-cpp-python not installed.")
    print("Install with: pip install llama-cpp-python")
    sys.exit(1)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# ============================================================================
# Constants
# ============================================================================

MODEL_REPO = "HuggingFaceTB/SmolLM2-360M-Instruct-GGUF"
MODEL_FILE = "smollm2-360m-instruct-q8_0.gguf"
MODEL_URL_BASE = f"https://huggingface.co/{MODEL_REPO}/resolve/main/"

# ============================================================================
# Data Models
# ============================================================================

class ChatMessage(BaseModel):
    """OpenAI-compatible chat message format."""
    role: str  # "user" or "assistant"
    content: str


class ChatCompletionRequest(BaseModel):
    """OpenAI-compatible chat completion request."""
    messages: List[ChatMessage]
    temperature: Optional[float] = 0.7
    max_tokens: Optional[int] = 512
    top_k: Optional[int] = 40
    top_p: Optional[float] = 0.9
    model: Optional[str] = "smollm"


# ============================================================================
# Model Management
# ============================================================================

def download_model_if_missing(model_path: Path) -> Path:
    """
    Download SmolLM GGUF model if it doesn't exist.
    
    Args:
        model_path: Path to directory where model should be stored
    
    Returns:
        Path to model file
    """
    model_path = Path(model_path)
    model_path.mkdir(parents=True, exist_ok=True)
    
    model_file = model_path / MODEL_FILE
    
    if model_file.exists():
        logger.info(f"Model file already exists: {model_file}")
        file_size_gb = model_file.stat().st_size / (1024 ** 3)
        logger.info(f"Model size: {file_size_gb:.2f} GB")
        return model_file
    
    logger.info(f"Downloading SmolLM model from Hugging Face...")
    logger.info(f"Model: {MODEL_REPO}")
    logger.info(f"File: {MODEL_FILE}")
    logger.info(f"Target: {model_file}")
    
    model_url = MODEL_URL_BASE + MODEL_FILE
    
    try:
        with urlopen(model_url) as response:
            total_size = int(response.headers.get('content-length', 0))
            
            logger.info(f"Downloading {total_size / (1024**2):.0f} MB...")
            
            with open(model_file, 'wb') as f:
                chunk_size = 8192
                downloaded = 0
                
                while True:
                    chunk = response.read(chunk_size)
                    if not chunk:
                        break
                    
                    f.write(chunk)
                    downloaded += len(chunk)
                    
                    if total_size > 0:
                        percent = (downloaded / total_size) * 100
                        mb_downloaded = downloaded / (1024 ** 2)
                        mb_total = total_size / (1024 ** 2)
                        logger.info(f"Progress: {percent:.1f}% ({mb_downloaded:.0f}/{mb_total:.0f} MB)")
        
        logger.info(f"✓ Model downloaded successfully")
        
    except URLError as e:
        logger.error(f"Failed to download model: {e}")
        logger.error(f"URL: {model_url}")
        raise
    
    return model_file


def load_smollm_model(model_path: Path, n_gpu_layers: int = 0) -> Llama:
    """
    Load SmolLM model using llama.cpp with optimized parameters.
    
    Args:
        model_path: Path to GGUF model file
        n_gpu_layers: Number of layers to offload to GPU (0 = CPU only)
    
    Returns:
        Llama model instance
    """
    model_path = Path(model_path)
    
    if not model_path.exists():
        raise FileNotFoundError(f"Model file not found: {model_path}")
    
    logger.info(f"Loading SmolLM model from {model_path}")
    logger.info(f"Model size: {model_path.stat().st_size / (1024**3):.2f} GB")
    
    try:
        model = Llama(
            model_path=str(model_path),
            n_gpu_layers=n_gpu_layers,
            n_ctx=2048,              # Context window
            n_threads=4,              # CPU threads (matches small model)
            n_batch=512,              # Batch size for prompt processing
            n_ubatch=128,             # Micro-batch size for efficient memory use
            verbose=False,
            # Optimization flags for CPU inference
            f16_kv=True,              # Use half-precision for KV cache
            logits_all=False,         # Only compute logits for last token (faster)
            # Memory optimization
            use_mlock=False,          # Don't lock model in memory
            use_mmap=True,            # Use memory-mapped file I/O (faster loading)
        )
        
        logger.info("✓ SmolLM model loaded successfully")
        return model
        
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        raise


# ============================================================================
# FastAPI Application
# ============================================================================

def create_app(model_path: str, n_gpu_layers: int = 0) -> FastAPI:
    """Create and configure FastAPI application."""
    
    app = FastAPI(title="SmolLM Inference Server", version="0.1.0")
    
    # Add CORS middleware for external clients
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    
    # Model state
    model: Optional[Llama] = None
    model_error: Optional[str] = None
    
    # Startup event
    @app.on_event("startup")
    async def startup():
        nonlocal model, model_error
        try:
            logger.info(f"Starting SmolLM inference server...")
            
            # Download model if needed
            model_file = download_model_if_missing(model_path)
            
            # Load model
            model = load_smollm_model(model_file, n_gpu_layers=n_gpu_layers)
            logger.info("Inference server ready!")
            
        except Exception as e:
            model_error = str(e)
            logger.error(f"Failed to initialize model: {e}")
    
    # Health check endpoint
    @app.get("/health")
    async def health():
        if model_error:
            return {
                "status": "error",
                "message": f"Model load failed: {model_error}",
                "ready": False
            }
        return {
            "status": "ok",
            "ready": model is not None,
            "model": "smollm2-360m-instruct"
        }
    
    # Chat completion endpoint
    @app.post("/chat/completions")
    async def chat_completions(request: ChatCompletionRequest):
        """OpenAI-compatible chat completions endpoint (streaming)."""
        
        if model_error:
            raise HTTPException(status_code=503, detail=f"Model not ready: {model_error}")
        
        if not model:
            raise HTTPException(status_code=503, detail="Inference engine not initialized")
        
        # Validate inputs
        if not request.messages:
            raise HTTPException(status_code=400, detail="No messages provided")
        
        if len(request.messages) > 500:
            raise HTTPException(status_code=400, detail="Too many messages (max 500)")
        
        # Clamp parameters for better quality
        temperature = max(0.1, min(1.0, request.temperature or 0.7))  # Reduced range for quality
        max_tokens = max(1, min(1024, request.max_tokens or 512))     # Cap at 1024 to avoid repetition
        top_k = max(1, min(50, request.top_k or 40)) if request.top_k else 40
        top_p = max(0.5, min(0.95, request.top_p or 0.9))            # Prevent too low top_p
        
        # Build prompt from messages
        # SmolLM2 uses the standard chat template: <|user|>\nmessage\n<|assistant|>\nresponse<|end_of_turn|>
        # When only user messages are provided, we add <|assistant|> to let the model complete the response
        prompt_parts = []
        for msg in request.messages:
            if msg.role == "user":
                prompt_parts.append(f"<|user|>\n{msg.content}")
            elif msg.role == "assistant":
                prompt_parts.append(f"<|assistant|>\n{msg.content}<|end_of_turn|>")
            elif msg.role == "system":
                # System messages go at the beginning
                prompt_parts.insert(0, f"<|system|>\n{msg.content}")
        
        # Build final prompt - add assistant turn for model to complete
        prompt = "\n".join(prompt_parts)
        if not prompt.endswith("<|assistant|>"):
            prompt += "\n<|assistant|>\n"
        
        logger.info(f"Chat request with {len(request.messages)} messages, max_tokens={max_tokens}")
        
        # Run inference in executor to avoid blocking
        async def token_generator():
            """Generate tokens using llama.cpp with OpenAI-compatible streaming format"""
            try:
                logger.info("Starting token generation...")
                
                # Call the model with optimized sampling parameters
                completion = model(
                    prompt=prompt,
                    max_tokens=max_tokens,
                    temperature=temperature,
                    top_k=top_k,
                    top_p=top_p,
                    repeat_penalty=1.1,          # Penalize repeated tokens
                    frequency_penalty=0.1,        # Reduce frequency of common tokens
                    presence_penalty=0.1,         # Encourage diverse tokens
                    stream=True
                )
                
                token_count = 0
                response_started = False
                for chunk in completion:
                    # Extract token from llama-cpp-python streaming format
                    if "choices" in chunk and len(chunk["choices"]) > 0:
                        token_text = chunk["choices"][0].get("text", "")
                        
                        if token_text:
                            # Skip the assistant marker at the start
                            if not response_started:
                                if "<|assistant|>" in token_text:
                                    token_text = token_text.replace("<|assistant|>", "").lstrip()
                                    if not token_text:
                                        continue
                                response_started = True
                            
                            # Stop at end-of-turn markers
                            stop_markers = ["<|end_of_turn|>", "<|user|>", "<|system|>"]
                            should_stop = False
                            for marker in stop_markers:
                                if marker in token_text:
                                    token_text = token_text.split(marker)[0]
                                    should_stop = True
                                    break
                            
                            # Send token if not empty
                            if token_text:
                                # Format as OpenAI-compatible streaming response
                                response = {
                                    "id": "chatcmpl-smollm",
                                    "object": "text_completion.chunk",
                                    "created": 0,
                                    "model": "smollm",
                                    "choices": [
                                        {
                                            "index": 0,
                                            "delta": {
                                                "content": token_text
                                            },
                                            "finish_reason": None
                                        }
                                    ]
                                }
                                yield f"data: {json.dumps(response)}\n\n"
                                token_count += 1
                            
                            # Stop if we hit a stop marker
                            if should_stop:
                                break
                            
                            # Yield control occasionally to prevent blocking
                            if token_count % 5 == 0:
                                await asyncio.sleep(0)
                
                # Send final completion marker in OpenAI format
                final_response = {
                    "id": "chatcmpl-smollm",
                    "object": "text_completion.chunk",
                    "created": 0,
                    "model": "smollm",
                    "choices": [
                        {
                            "index": 0,
                            "delta": {},
                            "finish_reason": "stop"
                        }
                    ]
                }
                yield f"data: {json.dumps(final_response)}\n\n"
                yield "data: [DONE]\n\n"
                logger.info(f"Completion finished: {token_count} tokens generated")
                
            except Exception as e:
                logger.error(f"Error during generation: {e}", exc_info=True)
                error_response = {
                    "id": "chatcmpl-smollm",
                    "object": "text_completion.chunk",
                    "created": 0,
                    "model": "smollm",
                    "choices": [
                        {
                            "index": 0,
                            "delta": {
                                "content": f"Error: {str(e)}"
                            },
                            "finish_reason": "error"
                        }
                    ]
                }
                yield f"data: {json.dumps(error_response)}\n\n"
                yield "data: [DONE]\n\n"
        
        return StreamingResponse(
            token_generator(),
            media_type="text/event-stream"
        )
    
    # Info endpoint
    @app.get("/info")
    async def info():
        return {
            "name": "smollm-inference",
            "version": "0.1.0",
            "model": "smollm2-360m-instruct",
            "model_size": "0.4B parameters",
            "quantization": "Q8_0"
        }
    
    return app

# ============================================================================
# Main Entry Point
# ============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="SmolLM Inference Server using llama.cpp"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8081,
        help="Port to run server on (default: 8081)"
    )
    parser.add_argument(
        "--host",
        type=str,
        default="0.0.0.0",
        help="Host to bind to (default: 0.0.0.0)"
    )
    parser.add_argument(
        "--model-path",
        type=str,
        default=None,
        help="Path to directory containing GGUF model"
    )
    parser.add_argument(
        "--gpu-layers",
        type=int,
        default=0,
        help="Number of layers to offload to GPU (0 = CPU only, default: 0)"
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=1,
        help="Number of worker processes (default: 1)"
    )
    
    args = parser.parse_args()
    
    # Determine model directory
    if args.model_path:
        model_dir = args.model_path
    else:
        model_dir = os.path.expanduser("~/.cache/openai-api-simulator/smollm")
    
    # Create app
    app = create_app(model_dir, n_gpu_layers=args.gpu_layers)
    
    # Start server
    logger.info(f"Starting SmolLM inference server on {args.host}:{args.port}")
    logger.info(f"Model directory: {model_dir}")
    logger.info(f"GPU layers: {args.gpu_layers}")
    
    uvicorn.run(
        app,
        host=args.host,
        port=args.port,
        workers=args.workers,
        log_level="info"
    )


if __name__ == "__main__":
    main()
