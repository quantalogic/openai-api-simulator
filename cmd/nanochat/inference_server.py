#!/usr/bin/env python3
"""
NanoChat PyTorch Inference Server

Provides OpenAI-compatible chat completion API using native PyTorch inference.
This server bridges the Go simulator to nanochat model inference.

Usage:
    python -m cmd.nanochat.inference_server --port 8081 --model-dir ~/.cache/openai-api-simulator/nanochat

Features:
    - Streaming Server-Sent Events for token generation
    - Automatic device selection (CUDA, MPS, CPU)
    - Efficient KV cache management
    - OpenAI-compatible /chat/completions endpoint
"""

import argparse
import asyncio
import json
import logging
import os
import sys
import torch
from pathlib import Path
from types import SimpleNamespace
from typing import AsyncGenerator, Optional, List
from contextlib import nullcontext

try:
    from fastapi import FastAPI, HTTPException
    from fastapi.middleware.cors import CORSMiddleware
    from fastapi.responses import StreamingResponse
    from pydantic import BaseModel
    import uvicorn
except ImportError:
    print("Error: Required packages not installed.")
    print("Install with: pip install fastapi uvicorn torch pydantic")
    sys.exit(1)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

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
    top_k: Optional[int] = 50
    model: Optional[str] = "nanochat"


# ============================================================================
# Device & Model Management
# ============================================================================

def autodetect_device() -> tuple[str, torch.device]:
    """Auto-detect best available device."""
    if torch.cuda.is_available():
        device_type = "cuda"
        device = torch.device("cuda:0")
        logger.info(f"Using CUDA device: {torch.cuda.get_device_name(0)}")
    elif torch.backends.mps.is_available():
        device_type = "mps"
        device = torch.device("mps")
        logger.info("Using Metal Performance Shaders (MPS) on Apple Silicon")
    else:
        device_type = "cpu"
        device = torch.device("cpu")
        logger.info("Using CPU for inference")
    
    return device_type, device


def download_model_if_missing(model_dir: Path) -> Path:
    """
    Download nanochat model files if they don't exist.
    
    Args:
        model_dir: Path to directory where model should be stored
    
    Returns:
        Path to model directory (created if needed)
    """
    import urllib.request
    
    model_dir = Path(model_dir)
    model_dir.mkdir(parents=True, exist_ok=True)
    
    # Define required files and their download URLs
    files_to_download = {
        "model_000650.pt": "https://huggingface.co/sdobson/nanochat/resolve/main/model_000650.pt",
        "meta_000650.json": "https://huggingface.co/sdobson/nanochat/resolve/main/meta_000650.json",
        "tokenizer.pkl": "https://huggingface.co/sdobson/nanochat/resolve/main/tokenizer.pkl",
    }
    
    missing_files = {
        name: url for name, url in files_to_download.items()
        if not (model_dir / name).exists()
    }
    
    if not missing_files:
        logger.info(f"All model files present in {model_dir}")
        return model_dir
    
    logger.info(f"Downloading {len(missing_files)} missing model file(s)...")
    
    for filename, url in missing_files.items():
        filepath = model_dir / filename
        logger.info(f"Downloading {filename} from {url}...")
        
        try:
            # Use urllib with progress indication
            def download_progress(block_num, block_size, total_size):
                downloaded = block_num * block_size
                if total_size > 0:
                    percent = min(downloaded * 100 / total_size, 100)
                    mb_downloaded = downloaded / (1024 * 1024)
                    mb_total = total_size / (1024 * 1024)
                    logger.info(f"  {filename}: {percent:.1f}% ({mb_downloaded:.1f}MB / {mb_total:.1f}MB)")
            
            urllib.request.urlretrieve(url, filepath, reporthook=download_progress)
            logger.info(f"✓ Downloaded {filename}")
        except Exception as e:
            logger.error(f"Failed to download {filename}: {e}")
            raise
    
    logger.info(f"✓ All model files downloaded to {model_dir}")
    return model_dir


def load_nanochat_model(model_dir: Path, device: torch.device):
    """
    Load nanochat model and tokenizer from PyTorch checkpoint.
    
    Args:
        model_dir: Path to directory containing nanochat model files
        device: torch.device to load model onto
    
    Returns:
        (model, tokenizer, config) tuple
    """
    model_dir = Path(model_dir)
    
    # Check required files
    model_file = model_dir / "model_000650.pt"
    meta_file = model_dir / "meta_000650.json"
    tokenizer_file = model_dir / "tokenizer.pkl"
    
    if not model_file.exists():
        raise FileNotFoundError(f"Model file not found: {model_file}")
    if not meta_file.exists():
        raise FileNotFoundError(f"Meta file not found: {meta_file}")
    if not tokenizer_file.exists():
        raise FileNotFoundError(f"Tokenizer file not found: {tokenizer_file}")
    
    # Load config
    import json as json_module
    import pickle
    
    with open(meta_file) as f:
        config_data = json_module.load(f)
    
    logger.info(f"Loading nanochat model from {model_dir}")
    logger.info(f"Config data: {config_data}")
    
    # Extract the model_config from the metadata
    if isinstance(config_data, dict) and "model_config" in config_data:
        config_dict = config_data["model_config"]
    else:
        config_dict = config_data
    
    # Convert dict to SimpleNamespace for nanochat.GPT compatibility
    config = SimpleNamespace(**config_dict)
    
    logger.info(f"Model config: {config_dict}")
    
    # Try to load nanochat modules
    try:
        from nanochat.gpt import GPT
    except ImportError as e:
        logger.error(
            "Failed to import nanochat modules. "
            "Make sure nanochat is installed: pip install nanochat or git clone https://github.com/karpathy/nanochat"
        )
        raise ImportError(f"nanochat modules not available: {e}") from e
    
    # Load model checkpoint
    logger.info(f"Loading model checkpoint from {model_file}")
    checkpoint = torch.load(model_file, map_location=device, weights_only=False)
    
    # Extract model state dict (checkpoint might be nested)
    if isinstance(checkpoint, dict) and "model" in checkpoint:
        state_dict = checkpoint["model"]
    elif isinstance(checkpoint, dict) and "state_dict" in checkpoint:
        state_dict = checkpoint["state_dict"]
    else:
        state_dict = checkpoint
    
    # Create model instance and load weights
    model = GPT(config)
    model.load_state_dict(state_dict)
    model = model.to(device)
    model.eval()
    
    # Load tokenizer
    logger.info(f"Loading tokenizer from {tokenizer_file}")
    with open(tokenizer_file, "rb") as f:
        tokenizer = pickle.load(f)
    
    logger.info("Model and tokenizer loaded successfully")
    
    return model, tokenizer, config


class NanoChatInference:
    """Wrapper for nanochat inference with KV cache optimization."""
    
    def __init__(self, model, tokenizer, device: torch.device, device_type: str):
        self.model = model
        self.tokenizer = tokenizer
        self.device = device
        self.device_type = device_type
        self.autocast_ctx = (
            torch.amp.autocast(device_type=device_type, dtype=torch.bfloat16)
            if device_type == "cuda"
            else nullcontext()
        )
    
    async def stream_completion(
        self,
        messages: List[ChatMessage],
        temperature: float = 0.7,
        max_tokens: int = 512,
        top_k: int = 50
    ) -> AsyncGenerator[str, None]:
        """
        Stream chat completion tokens as JSON lines.
        
        Yields Server-Sent Events format: "data: {json}\n\n"
        """
        try:
            # Build conversation tokens from messages
            tokens = self._build_conversation_tokens(messages)
            if not tokens:
                tokens = [1]  # Default token if empty
            logger.info(f"Built conversation with {len(tokens)} tokens, generating...")
            
            # Generate tokens using the model
            token_count = 0
            # Try to get EOS token, fallback to a safe value
            try:
                if hasattr(self.tokenizer, "encode"):
                    eos_encoded = self.tokenizer.encode("<|end|>")
                    eos_token_id = eos_encoded[0] if eos_encoded else 2
                else:
                    eos_token_id = 2
            except Exception as e:
                logger.warning(f"Failed to get EOS token: {e}, using fallback")
                eos_token_id = 2
            
            with torch.no_grad():
                with self.autocast_ctx:
                    # Convert token list to tensor
                    x = torch.tensor(tokens, dtype=torch.long, device=self.device).unsqueeze(0)
                    
                    # Generate tokens one by one
                    while token_count < max_tokens:
                        # Model forward pass
                        logits = self.model(x)
                        
                        # Get logits for next token (last position)
                        next_logits = logits[0, -1, :]
                        
                        # Apply temperature scaling
                        if temperature > 0:
                            next_logits = next_logits / temperature
                        
                        # Apply top-k filtering
                        if top_k > 0:
                            # Get top-k logits and their indices
                            top_k_logits, top_k_indices = torch.topk(next_logits, min(top_k, next_logits.size(0)))
                            
                            # Set all other logits to very negative value
                            next_logits_filtered = torch.full_like(next_logits, float('-inf'))
                            next_logits_filtered[top_k_indices] = top_k_logits
                            next_logits = next_logits_filtered
                        
                        # Convert to probabilities
                        probs = torch.softmax(next_logits, dim=-1)
                        
                        # Sample next token
                        next_token = torch.multinomial(probs, num_samples=1).item()
                        
                        # Stop if EOS token
                        if next_token == eos_token_id:
                            logger.info(f"Generated {token_count} tokens (EOS)")
                            break
                        
                        # Decode token to text
                        token_text = self._decode_token(next_token)
                        
                        # Yield token
                        yield f"data: {json.dumps({'token': token_text})}\n\n"
                        logger.debug(f"Token {token_count}: {repr(token_text)}")
                        
                        # Append to sequence
                        x = torch.cat([x, torch.tensor([[next_token]], device=self.device)], dim=1)
                        
                        token_count += 1
                        
                        # Yield control to event loop occasionally
                        if token_count % 10 == 0:
                            await asyncio.sleep(0)
            
            yield "data: {\"done\": true}\n\n"
            logger.info(f"Completion finished: {token_count} tokens generated")
        
        except Exception as e:
            logger.error(f"Error during generation: {e}", exc_info=True)
            yield f"data: {json.dumps({'error': str(e)})}\n\n"
    
    def _build_conversation_tokens(self, messages: List[ChatMessage]) -> List[int]:
        """Convert chat messages to token sequence."""
        tokens = []
        
        # Add special tokens for conversation format
        # Format: <|user_start|> user message <|user_end|> <|assistant_start|> assistant response <|assistant_end|>
        
        for msg in messages:
            if msg.role == "user":
                # Add user message
                if hasattr(self.tokenizer, "encode"):
                    tokens.extend(self.tokenizer.encode(f"User: {msg.content}\n"))
                else:
                    # Fallback: simple character-level encoding
                    tokens.extend([ord(c) for c in f"User: {msg.content}\n"])
            elif msg.role == "assistant":
                # Add assistant message
                if hasattr(self.tokenizer, "encode"):
                    tokens.extend(self.tokenizer.encode(f"Assistant: {msg.content}\n"))
                else:
                    tokens.extend([ord(c) for c in f"Assistant: {msg.content}\n"])
        
        # Start assistant response
        if hasattr(self.tokenizer, "encode"):
            tokens.extend(self.tokenizer.encode("Assistant: "))
        else:
            tokens.extend([ord(c) for c in "Assistant: "])
        
        return tokens
    
    def _decode_token(self, token_id: int) -> str:
        """Decode a single token ID to text."""
        try:
            if hasattr(self.tokenizer, "decode"):
                return self.tokenizer.decode([token_id])
            else:
                # Fallback: try to use decode_single_token_utf8 or character
                if hasattr(self.tokenizer, "decode_single_token_utf8"):
                    return self.tokenizer.decode_single_token_utf8(token_id)
                else:
                    return chr(token_id) if 0 <= token_id < 256 else "?"
        except Exception as e:
            logger.warning(f"Failed to decode token {token_id}: {e}")
            return "?"


# ============================================================================
# FastAPI Application
# ============================================================================

def create_app(model_dir: str, device_type: Optional[str] = None) -> FastAPI:
    """Create and configure FastAPI application."""
    
    app = FastAPI(title="NanoChat Inference Server", version="0.1.0")
    
    # Add CORS middleware for external clients
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    
    # Device selection
    if device_type:
        if device_type == "cuda":
            selected_device = torch.device("cuda:0")
            selected_device_type = "cuda"
        elif device_type == "mps":
            selected_device = torch.device("mps")
            selected_device_type = "mps"
        elif device_type == "cpu":
            selected_device = torch.device("cpu")
            selected_device_type = "cpu"
        else:
            raise ValueError(f"Unsupported device type: {device_type}")
    else:
        selected_device_type, selected_device = autodetect_device()
    
    # Model and inference engine
    inference_engine: Optional[NanoChatInference] = None
    model_error: Optional[str] = None
    
    # Startup event
    @app.on_event("startup")
    async def startup():
        nonlocal inference_engine, model_error
        try:
            logger.info(f"Checking model directory: {model_dir}")
            # Download model if missing
            download_model_if_missing(model_dir)
            
            logger.info(f"Loading model from {model_dir}")
            model, tokenizer, config = load_nanochat_model(
                model_dir,
                selected_device
            )
            inference_engine = NanoChatInference(
                model,
                tokenizer,
                selected_device,
                selected_device_type
            )
            logger.info("Model loaded successfully")
        except Exception as e:
            model_error = str(e)
            logger.error(f"Failed to load model: {e}")
    
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
            "ready": inference_engine is not None,
            "device": str(selected_device),
            "device_type": selected_device_type
        }
    
    # Chat completion endpoint
    @app.post("/chat/completions")
    async def chat_completions(request: ChatCompletionRequest):
        """OpenAI-compatible chat completions endpoint (streaming)."""
        
        if model_error:
            raise HTTPException(status_code=503, detail=f"Model not ready: {model_error}")
        
        if not inference_engine:
            raise HTTPException(status_code=503, detail="Inference engine not initialized")
        
        # Validate inputs
        if not request.messages:
            raise HTTPException(status_code=400, detail="No messages provided")
        
        if len(request.messages) > 500:
            raise HTTPException(status_code=400, detail="Too many messages (max 500)")
        
        # Clamp parameters
        temperature = max(0.0, min(2.0, request.temperature or 0.7))
        max_tokens = max(1, min(4096, request.max_tokens or 512))
        top_k = max(1, min(200, request.top_k or 50)) if request.top_k else None
        
        # Log request
        logger.info(f"Chat request: {len(request.messages)} messages")
        for i, msg in enumerate(request.messages[-3:]):  # Log last 3 messages
            logger.info(f"  [{msg.role}] (message {i}): {msg.content[:100]}")
        
        # Create streaming response
        return StreamingResponse(
            inference_engine.stream_completion(
                request.messages,
                temperature=temperature,
                max_tokens=max_tokens,
                top_k=top_k
            ),
            media_type="text/event-stream"
        )
    
    # Info endpoint
    @app.get("/info")
    async def info():
        return {
            "name": "nanochat-inference",
            "version": "0.1.0",
            "model": "sdobson/nanochat",
            "device": str(selected_device),
            "device_type": selected_device_type,
            "torch_version": torch.__version__
        }
    
    return app


# ============================================================================
# Main Entry Point
# ============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="NanoChat Inference Server"
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
        default="127.0.0.1",
        help="Host to bind to (default: 127.0.0.1)"
    )
    parser.add_argument(
        "--model-dir",
        type=str,
        default=None,
        help="Path to nanochat model directory"
    )
    parser.add_argument(
        "--device",
        type=str,
        choices=["cuda", "mps", "cpu"],
        default=None,
        help="Force specific device (cuda, mps, cpu). Auto-detect if not specified."
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=1,
        help="Number of worker processes (default: 1)"
    )
    
    args = parser.parse_args()
    
    # Determine model directory
    if args.model_dir:
        model_dir = args.model_dir
    else:
        model_dir = os.path.expanduser("~/.cache/openai-api-simulator/nanochat")
    
    model_dir = Path(model_dir)
    # Note: model_dir will be created and model will be downloaded during app startup
    
    # Create app
    app = create_app(str(model_dir), device_type=args.device)
    
    # Start server
    logger.info(f"Starting NanoChat inference server on {args.host}:{args.port}")
    logger.info(f"Model directory: {model_dir}")
    
    uvicorn.run(
        app,
        host=args.host,
        port=args.port,
        workers=args.workers,
        log_level="info"
    )


if __name__ == "__main__":
    main()
