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
# Using a smaller, more compatible model for development
# The original large model can be configured via environment variable  
MODEL_NAME = os.environ.get("MODEL_NAME", "microsoft/DialoGPT-medium") 
CACHE_DIR = "models"
TAGS_FILE = Path("tags.json")

logging.info(f"Development mode: Using simple keyword-based tagging instead of LLM")
logging.info("To enable LLM tagging, set environment variable ENABLE_LLM=true")

# For development, disable LLM model loading to avoid compatibility issues
model = None
tokenizer = None

if os.environ.get("ENABLE_LLM", "false").lower() == "true":
    logging.info(f"Loading model: {MODEL_NAME}...")
    try:
        os.makedirs(CACHE_DIR, exist_ok=True)
        
        tokenizer = AutoTokenizer.from_pretrained(MODEL_NAME, cache_dir=CACHE_DIR)
        # Set pad_token if not present (common issue with some models)
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token
        
        model = AutoModelForCausalLM.from_pretrained(
            MODEL_NAME,
            torch_dtype=torch.float32,  # Use float32 for CPU compatibility
            low_cpu_mem_usage=True,
            device_map="cpu",  # Force CPU for development to avoid GPU issues
            cache_dir=CACHE_DIR,
        )
        logging.info("Model loaded successfully.")
    except Exception as e:
        logging.error(f"Fatal error: Could not load model {MODEL_NAME}. {e}")
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
    # In development mode, we're always healthy even without LLM model
    is_healthy = True  # Always healthy since we have fallback keyword tagging
    return HealthResponse(
        status="healthy" if is_healthy else "unhealthy",
        model_loaded=model is not None
    )

@app.post("/generate-tags", response_model=TagResponse)
async def generate_tags_endpoint(request: TagRequest):
    logging.info(f"Received request for content: \"{request.content[:50]}...\"")
    
    # Load existing tags
    existing_tags = load_existing_tags()
    
    # For development, use a simple keyword-based approach if model fails
    if not model or not tokenizer:
        logging.warning("Model not available, using simple keyword extraction")
        generated_tags = generate_simple_tags(request.content, existing_tags)
    else:
        try:
            generated_tags = generate_llm_tags(request.content, existing_tags)
        except Exception as e:
            logging.error(f"LLM generation failed: {e}, falling back to simple tags")
            generated_tags = generate_simple_tags(request.content, existing_tags)

    # Update the master list of tags and save it
    updated_tags = existing_tags + generated_tags
    save_tags(updated_tags)
    
    return TagResponse(tags=generated_tags)

def generate_simple_tags(content: str, existing_tags: list[str]) -> list[str]:
    """Simple keyword-based tag generation for development"""
    import re
    
    # Convert to lowercase and extract words
    words = re.findall(r'\b\w+\b', content.lower())
    
    # Filter out common stop words
    stop_words = {'the', 'a', 'an', 'and', 'or', 'but', 'in', 'on', 'at', 'to', 'for', 'of', 'with', 'by', 'from', 'up', 'about', 'into', 'through', 'during', 'before', 'after', 'above', 'below', 'between', 'under', 'is', 'am', 'are', 'was', 'were', 'be', 'been', 'being', 'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'should', 'could', 'i', 'me', 'my', 'we', 'our', 'you', 'your', 'he', 'him', 'his', 'she', 'her', 'it', 'its', 'they', 'them', 'their'}
    
    # Extract meaningful words
    meaningful_words = [word for word in words if len(word) > 2 and word not in stop_words]
    
    # Prioritize existing tags that match
    matched_tags = [tag for tag in existing_tags if any(word in content.lower() for word in tag.split())]
    
    # Add new tags from meaningful words
    new_tags = meaningful_words[:3]  # Take first 3 meaningful words
    
    # Combine and deduplicate
    all_tags = list(dict.fromkeys(matched_tags + new_tags))  # Preserves order, removes duplicates
    
    return all_tags[:5]  # Return up to 5 tags

def generate_llm_tags(content: str, existing_tags: list[str]) -> list[str]:
    """LLM-based tag generation"""
    tags_json_string = json.dumps(existing_tags)
    
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
                f"TEXT: \"{content}\""
            )
        }
    ]
    
    # Try to use chat template if available
    try:
        inputs = tokenizer.apply_chat_template(
            messages,
            add_generation_prompt=True,
            tokenize=True,
            return_tensors="pt",
        ).to(model.device)
    except Exception:
        logging.info("Chat template not supported, using simple concatenation")
        prompt_text = f"{messages[0]['content']}\n\n{messages[1]['content']}"
        inputs = tokenizer.encode(prompt_text, return_tensors="pt").to(model.device)
    
    # Generate the model's output
    outputs = model.generate(inputs, max_new_tokens=60, pad_token_id=tokenizer.eos_token_id)
    
    # Decode the output and extract the JSON response
    response_text = tokenizer.decode(outputs[0][inputs.shape[-1]:], skip_special_tokens=True)
    try:
        json_response_str = response_text[response_text.find('['):response_text.rfind(']')+1]
        generated_tags = json.loads(json_response_str)
        if not isinstance(generated_tags, list):
            raise ValueError("LLM did not return a list.")
        logging.info(f"LLM generated tags: {generated_tags}")
        return generated_tags
    except Exception as e:
        logging.error(f"Failed to parse LLM output: {e}. Response was: \"{response_text}\"")
        raise Exception("Failed to parse LLM output")

# To run this app, use the command: uvicorn app.main:app --host 0.0.0.0 --port 8000
if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)