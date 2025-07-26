import json
import logging
import re
from pathlib import Path
from threading import Lock

from fastapi import FastAPI
from pydantic import BaseModel

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

TAGS_FILE = Path("tags.json")

logging.info("Simple tagger service starting - using keyword-based tagging")

# --- Helper Functions for tag management ---
def load_existing_tags() -> list[str]:
    """Loads the list of tags from the JSON file."""
    with lock:
        if not TAGS_FILE.exists():
            return []
        try:
            with open(TAGS_FILE, "r") as f:
                return json.load(f)
        except:
            return []

def save_tags(tags: list[str]):
    """Saves the updated list of tags, sorted and without duplicates."""
    with lock:
        unique_sorted_tags = sorted(list(set(tags)))
        with open(TAGS_FILE, "w") as f:
            json.dump(unique_sorted_tags, f, indent=2)

def generate_simple_tags(content: str, existing_tags: list[str]) -> list[str]:
    """Simple keyword-based tag generation"""
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

# --- API Endpoints ---
@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint for the Go backend integration."""
    return HealthResponse(
        status="healthy",
        model_loaded=False  # We're using simple keyword extraction, not an ML model
    )

@app.post("/generate-tags", response_model=TagResponse)
async def generate_tags_endpoint(request: TagRequest):
    logging.info(f"Received request for content: \"{request.content[:50]}...\"")
    
    # Load existing tags
    existing_tags = load_existing_tags()
    
    # Generate tags using simple keyword extraction
    generated_tags = generate_simple_tags(request.content, existing_tags)
    
    logging.info(f"Generated tags: {generated_tags}")
    
    # Update the master list of tags and save it
    updated_tags = existing_tags + generated_tags
    save_tags(updated_tags)
    
    return TagResponse(tags=generated_tags)

if __name__ == "__main__":
    import uvicorn
    import os
    port = int(os.environ.get("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)