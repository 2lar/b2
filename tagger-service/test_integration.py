#!/usr/bin/env python3
"""
Integration test for the tagger service.
Tests the API endpoints and validates responses.
"""

import json
import requests
import time
import sys
from typing import Dict, Any

# Test configuration
BASE_URL = "http://localhost:8000"
TIMEOUT = 30

def print_status(message: str):
    print(f"🔍 {message}")

def print_success(message: str):
    print(f"✅ {message}")

def print_error(message: str):
    print(f"❌ {message}")

def print_warning(message: str):
    print(f"⚠️  {message}")

def wait_for_service(max_attempts: int = 30) -> bool:
    """Wait for the service to be available."""
    print_status("Waiting for tagger service to be ready...")
    
    for attempt in range(max_attempts):
        try:
            response = requests.get(f"{BASE_URL}/health", timeout=5)
            if response.status_code == 200:
                health_data = response.json()
                if health_data.get("model_loaded"):
                    print_success("Tagger service is ready!")
                    return True
                else:
                    print_warning(f"Service responding but model not loaded (attempt {attempt + 1}/{max_attempts})")
        except requests.RequestException:
            print_warning(f"Service not ready (attempt {attempt + 1}/{max_attempts})")
        
        time.sleep(2)
    
    return False

def test_health_endpoint() -> bool:
    """Test the health check endpoint."""
    print_status("Testing health endpoint...")
    
    try:
        response = requests.get(f"{BASE_URL}/health", timeout=TIMEOUT)
        
        if response.status_code != 200:
            print_error(f"Health check failed with status {response.status_code}")
            return False
        
        data = response.json()
        required_fields = ["status", "model_loaded"]
        
        for field in required_fields:
            if field not in data:
                print_error(f"Health response missing field: {field}")
                return False
        
        if data["status"] != "healthy":
            print_error(f"Service status is not healthy: {data['status']}")
            return False
        
        if not data["model_loaded"]:
            print_error("Model is not loaded")
            return False
        
        print_success("Health endpoint test passed")
        return True
        
    except requests.RequestException as e:
        print_error(f"Health check request failed: {e}")
        return False

def test_generate_tags(content: str, expected_count: int = None) -> bool:
    """Test the tag generation endpoint."""
    print_status(f"Testing tag generation for: '{content[:50]}...'")
    
    try:
        payload = {"content": content}
        response = requests.post(
            f"{BASE_URL}/generate-tags",
            json=payload,
            timeout=TIMEOUT,
            headers={"Content-Type": "application/json"}
        )
        
        if response.status_code != 200:
            print_error(f"Tag generation failed with status {response.status_code}: {response.text}")
            return False
        
        data = response.json()
        
        if "tags" not in data:
            print_error("Response missing 'tags' field")
            return False
        
        tags = data["tags"]
        
        if not isinstance(tags, list):
            print_error(f"Tags should be a list, got {type(tags)}")
            return False
        
        if len(tags) == 0:
            print_warning("No tags generated")
        
        # Validate tag format
        for tag in tags:
            if not isinstance(tag, str):
                print_error(f"Tag should be string, got {type(tag)}: {tag}")
                return False
            
            if len(tag) < 2:
                print_error(f"Tag too short: '{tag}'")
                return False
            
            if not tag.islower():
                print_error(f"Tag should be lowercase: '{tag}'")
                return False
        
        if expected_count and len(tags) != expected_count:
            print_warning(f"Expected {expected_count} tags, got {len(tags)}")
        
        print_success(f"Generated {len(tags)} tags: {tags}")
        return True
        
    except requests.RequestException as e:
        print_error(f"Tag generation request failed: {e}")
        return False

def test_error_handling() -> bool:
    """Test error handling with invalid requests."""
    print_status("Testing error handling...")
    
    # Test missing content
    try:
        response = requests.post(
            f"{BASE_URL}/generate-tags",
            json={},
            timeout=TIMEOUT
        )
        
        if response.status_code == 200:
            print_error("Expected error for missing content, but request succeeded")
            return False
        
        print_success("Correctly rejected request with missing content")
        
    except requests.RequestException as e:
        print_error(f"Error handling test failed: {e}")
        return False
    
    # Test invalid JSON
    try:
        response = requests.post(
            f"{BASE_URL}/generate-tags",
            data="invalid json",
            timeout=TIMEOUT,
            headers={"Content-Type": "application/json"}
        )
        
        if response.status_code == 200:
            print_error("Expected error for invalid JSON, but request succeeded")
            return False
        
        print_success("Correctly rejected request with invalid JSON")
        return True
        
    except requests.RequestException as e:
        print_error(f"Error handling test failed: {e}")
        return False

def run_all_tests() -> bool:
    """Run all integration tests."""
    print("🚀 Starting tagger service integration tests")
    print("=" * 50)
    
    # Wait for service to be ready
    if not wait_for_service():
        print_error("Service failed to start within timeout period")
        return False
    
    tests = [
        ("Health Check", test_health_endpoint),
        ("Basic Tag Generation", lambda: test_generate_tags("I'm learning about machine learning algorithms")),
        ("Technology Content", lambda: test_generate_tags("Building a React application with TypeScript and GraphQL")),
        ("Short Content", lambda: test_generate_tags("Hello world")),
        ("Long Content", lambda: test_generate_tags("This is a comprehensive guide to understanding machine learning algorithms, including supervised learning, unsupervised learning, and reinforcement learning. We'll cover neural networks, decision trees, and many other important concepts.")),
        ("Error Handling", test_error_handling),
    ]
    
    passed = 0
    total = len(tests)
    
    for test_name, test_func in tests:
        print(f"\n📝 Running: {test_name}")
        print("-" * 30)
        
        try:
            if test_func():
                passed += 1
            else:
                print_error(f"Test failed: {test_name}")
        except Exception as e:
            print_error(f"Test crashed: {test_name} - {e}")
    
    print("\n" + "=" * 50)
    print(f"🏁 Tests completed: {passed}/{total} passed")
    
    if passed == total:
        print_success("All tests passed! Integration is working correctly.")
        return True
    else:
        print_error(f"{total - passed} tests failed. Check the output above for details.")
        return False

if __name__ == "__main__":
    success = run_all_tests()
    sys.exit(0 if success else 1)