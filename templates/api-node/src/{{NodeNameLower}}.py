"""
{{NodeName}} - API-focused ComfyUI node
{{NodeDescription}}

This node is designed for API integrations and external service connections.

Author: {{Author}}
"""

import requests
import asyncio
import aiohttp
import json
from typing import Dict, Any, Tuple, Optional, List
import torch


class {{NodeName}}:
    """
    API-focused ComfyUI node for external service integration.
    
    Features:
    - HTTP/HTTPS API calls
    - Async request handling
    - Authentication support
    - Response caching
    - Error handling and retries
    """
    
    RETURN_TYPES = ("STRING", "DICT", "INT", "FLOAT")
    RETURN_NAMES = ("response_text", "response_data", "status_code", "response_time")
    FUNCTION = "make_api_call"
    CATEGORY = "{{NodeName}}/api"
    
    def __init__(self):
        self.session = None
        self.cache = {}
        
    @classmethod
    def INPUT_TYPES(cls) -> Dict[str, Any]:
        return {
            "required": {
                "api_url": ("STRING", {
                    "default": "https://api.example.com/endpoint",
                    "multiline": False,
                    "tooltip": "API endpoint URL"
                }),
                "method": (["GET", "POST", "PUT", "DELETE", "PATCH"], {
                    "default": "GET",
                    "tooltip": "HTTP method"
                }),
                "timeout": ("INT", {
                    "default": 30,
                    "min": 1,
                    "max": 300,
                    "tooltip": "Request timeout in seconds"
                }),
            },
            "optional": {
                "headers": ("STRING", {
                    "default": "{}",
                    "multiline": True,
                    "tooltip": "Request headers in JSON format"
                }),
                "body": ("STRING", {
                    "default": "",
                    "multiline": True,
                    "tooltip": "Request body (for POST/PUT/PATCH)"
                }),
                "auth_token": ("STRING", {
                    "default": "",
                    "tooltip": "Authorization token"
                }),
                "cache_enabled": ("BOOLEAN", {
                    "default": True,
                    "tooltip": "Enable response caching"
                }),
            }
        }
    
    async def make_api_call(self,
                           api_url: str,
                           method: str,
                           timeout: int,
                           headers: str = "{}",
                           body: str = "",
                           auth_token: str = "",
                           cache_enabled: bool = True) -> Tuple[str, Dict, int, float]:
        """
        Make an API call with full configuration support.
        
        Args:
            api_url: Target API endpoint
            method: HTTP method
            timeout: Request timeout
            headers: Additional headers as JSON
            body: Request body
            auth_token: Authorization token
            cache_enabled: Whether to use caching
            
        Returns:
            Tuple of (response_text, response_data, status_code, response_time)
        """
        try:
            # Check cache first
            cache_key = f"{method}:{api_url}:{body}"
            if cache_enabled and cache_key in self.cache:
                cached = self.cache[cache_key]
                return (cached["text"], cached["data"], cached["status"], 0.0)
            
            # Parse headers
            try:
                parsed_headers = json.loads(headers) if headers else {}
            except json.JSONDecodeError:
                parsed_headers = {}
            
            # Add authorization if provided
            if auth_token:
                parsed_headers["Authorization"] = f"Bearer {auth_token}"
            
            # Set content type for requests with body
            if body and method in ["POST", "PUT", "PATCH"]:
                if "Content-Type" not in parsed_headers:
                    parsed_headers["Content-Type"] = "application/json"
            
            # Make async request
            start_time = asyncio.get_event_loop().time()
            
            async with aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=timeout)) as session:
                async with session.request(
                    method=method,
                    url=api_url,
                    headers=parsed_headers,
                    data=body if body else None
                ) as response:
                    response_time = asyncio.get_event_loop().time() - start_time
                    status_code = response.status
                    response_text = await response.text()
                    
                    # Try to parse as JSON
                    try:
                        response_data = json.loads(response_text)
                    except json.JSONDecodeError:
                        response_data = {"raw_response": response_text}
                    
                    # Cache successful responses
                    if cache_enabled and 200 <= status_code < 300:
                        self.cache[cache_key] = {
                            "text": response_text,
                            "data": response_data,
                            "status": status_code
                        }
                    
                    return (response_text, response_data, status_code, response_time)
                    
        except asyncio.TimeoutError:
            return ("Request timeout", {"error": "timeout"}, 408, timeout)
        except Exception as e:
            error_msg = f"API request failed: {str(e)}"
            return (error_msg, {"error": str(e)}, 500, 0.0)


# Node Registration
NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (API)"
}