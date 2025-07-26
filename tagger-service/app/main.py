import json
import logging
import os
from pathlib import Path
from threading import Lock

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import AutoModelForCausalLM, AutoTokenizer

# --- Configuration ---
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# --- Pydantic Models for API validation ---
class TagRequest(BaseModel):
    content: str

class TagResponse(BaseModel):
    tags: list[str]

class HealthResponse(BaseModel):
    status: str
    model_loaded: bool

# --- Global Variables & Initialization ---
app = FastAPI(title="B2 Tagger Service", version="1.0.0")
lock = Lock() 

# --- Model & Tokenizer Loading ---
# Using the official repo name the user provided in the snippet.
# Using optimizations for better performance in a service.
MODEL_NAME = "hugging-quants/Meta-Llama-3.1-8B-Instruct-AWQ-INT4" 
CACHE_DIR = "models"
TAGS_FILE = Path("tags.json")

logging.info(f"Loading model: {MODEL_NAME}...")
try:
    os.makedirs(CACHE_DIR, exist_ok=True)
    
    tokenizer = AutoTokenizer.from_pretrained(MODEL_NAME, cache_dir=CACHE_DIR)
    model = AutoModelForCausalLM.from_pretrained(
        MODEL_NAME,
        torch_dtype=torch.float16,
        low_cpu_mem_usage=True,
        device_map="auto", # Automatically use GPU if available
        cache_dir=CACHE_DIR,
    )
    logging.info("Model loaded successfully.")
except Exception as e:
    logging.error(f"Fatal error: Could not load model. {e}")
    model = None 
    tokenizer = None

# --- Helper Functions for tag management ---
def load_existing_tags() -> list[str]:
    """Loads the list of tags from the JSON file."""
    with lock:
        if not TAGS_FILE.exists():
            return []
        with open(TAGS_FILE, "r") as f:
            return json.load(f)

def save_tags(tags: list[str]):
    """Saves the updated list of tags, sorted and without duplicates."""
    with lock:
        unique_sorted_tags = sorted(list(set(tags)))
        with open(TAGS_FILE, "w") as f:
            json.dump(unique_sorted_tags, f, indent=2)

# --- API Endpoints ---
@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint for the Go backend integration."""
    return HealthResponse(
        status="healthy" if model is not None else "unhealthy",
        model_loaded=model is not None
    )

@app.post("/generate-tags", response_model=TagResponse)
async def generate_tags_endpoint(request: TagRequest):
    if not model or not tokenizer:
        raise HTTPException(status_code=503, detail="Model is not available.")

    logging.info(f"Received request for content: \"{request.content[:50]}...\"")
    
    # 1. Load the current list of tags
    existing_tags = load_existing_tags()
    tags_json_string = json.dumps(existing_tags)
    
    # 2. Define the chat messages for the prompt (like in your snippet)
    messages = [
        {
            "role": "system",
            "content": (
                "You are an expert at categorizing notes. Your task is to generate relevant, single-word, lowercase tags for a given text. "
                "You are given a list of existing tags to choose from. Prioritize using these existing tags if they are relevant. "
                "If no existing tags fit well, you are allowed to generate new, relevant, single-word, lowercase tags. "
                "Your final output MUST be a single, raw JSON array of strings, and nothing else. For example: [\"tag1\", \"tag2\"]"
            )
        },
        {
            "role": "user",
            "content": (
                f"Here is the list of existing tags to reference:\n{tags_json_string}\n\n"
                f"Now, please generate up to 5 tags for the following text:\n\n"
                f"TEXT: \"{request.content}\""
            )
        }
    ]
    
    # 3. Apply the chat template and tokenize
    inputs = tokenizer.apply_chat_template(
        messages,
        add_generation_prompt=True,
        tokenize=True,
        return_tensors="pt",
    ).to(model.device)
    
    # 4. Generate the model's output
    outputs = model.generate(inputs, max_new_tokens=60, pad_token_id=tokenizer.eos_token_id)
    
    # 5. Decode the output and extract the JSON response
    response_text = tokenizer.decode(outputs[0][inputs.shape[-1]:], skip_special_tokens=True)
    try:
        # A simple way to extract the JSON part from the response string
        json_response_str = response_text[response_text.find('['):response_text.rfind(']')+1]
        generated_tags = json.loads(json_response_str)
        if not isinstance(generated_tags, list): # Basic validation
            raise ValueError("LLM did not return a list.")
        logging.info(f"LLM generated tags: {generated_tags}")
    except Exception as e:
        logging.error(f"Failed to parse LLM output: {e}. Response was: \"{response_text}\"")
        raise HTTPException(status_code=500, detail="Failed to parse LLM output.")

    # 6. Update the master list of tags and save it
    updated_tags = existing_tags + generated_tags
    save_tags(updated_tags)
    
    return TagResponse(tags=generated_tags)

# To run this app, use the command: uvicorn app.main:app --host 0.0.0.0 --port 8000
if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)