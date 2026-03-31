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

    @classmethod
    def IS_CHANGED(cls, api_url, method, timeout, headers="{}", body="",
                   auth_token="", cache_enabled=True):
        """
        Determines if the node needs re-execution based on input changes.

        For API nodes, consider:
        - External APIs may return different data even with same inputs
        - Caching behavior should match API semantics (GET vs POST)
        - Time-sensitive data may need forced refresh

        Return options:
        - float('nan'): Always re-run (recommended for most API calls)
        - Hash of inputs: Re-run only when inputs change (for stable endpoints)

        Example customizations:
            # Always fetch fresh data for POST requests:
            if method in ["POST", "PUT", "PATCH", "DELETE"]:
                return float('nan')

            # Cache GET requests based on URL and params:
            if method == "GET" and cache_enabled:
                return hash((api_url, headers))
        """
        # For API nodes, always re-run by default since external data may change
        # This ensures fresh data from APIs
        # If you want caching, modify based on your API's behavior
        if not cache_enabled:
            return float('nan')

        # When caching is enabled, hash the request parameters
        # Note: This only prevents redundant calls within the same workflow run
        return hash((api_url, method, headers, body, auth_token))

    @classmethod
    def VALIDATE_INPUTS(cls, api_url, method, timeout, headers="{}", body="",
                        auth_token="", cache_enabled=True):
        """
        Validates inputs before the node executes.

        For API nodes, validate:
        - URL format and protocol
        - JSON format for headers and body
        - API key/token format (without exposing secrets)
        - Timeout ranges

        Return values:
        - True: All inputs are valid
        - String: Error message describing the validation failure

        Example customizations:
            # Validate specific API endpoint patterns:
            if "api.example.com" in api_url and not auth_token:
                return "API key required for api.example.com"

            # Validate request body for specific methods:
            if method == "POST" and not body:
                return "POST requests require a body"
        """
        import re

        # Validate URL format
        if not api_url:
            return "api_url is required"

        # Basic URL format validation
        url_pattern = r'^https?://[^\s/$.?#].[^\s]*$'
        if not re.match(url_pattern, api_url, re.IGNORECASE):
            return f"Invalid URL format: {api_url}. Must start with http:// or https://"

        # Warn about non-HTTPS URLs (but allow them)
        # This is informational - actual security checks should be stricter
        if api_url.startswith("http://") and "localhost" not in api_url and "127.0.0.1" not in api_url:
            # Note: This is a warning, not an error - you might want to make this stricter
            pass

        # Validate headers JSON format
        if headers:
            try:
                import json
                parsed_headers = json.loads(headers)
                if not isinstance(parsed_headers, dict):
                    return "headers must be a JSON object (dict)"
            except json.JSONDecodeError as e:
                return f"Invalid headers JSON format: {str(e)}"

        # Validate body JSON format for methods that typically use JSON
        if body and method in ["POST", "PUT", "PATCH"]:
            # Only validate if it looks like JSON (starts with { or [)
            stripped_body = body.strip()
            if stripped_body.startswith('{') or stripped_body.startswith('['):
                try:
                    import json
                    json.loads(body)
                except json.JSONDecodeError as e:
                    return f"Invalid body JSON format: {str(e)}"

        # Validate timeout range
        if timeout < 1:
            return "timeout must be at least 1 second"
        if timeout > 300:
            return "timeout cannot exceed 300 seconds (5 minutes)"

        # Validate auth_token format (basic check - not empty spaces)
        if auth_token and auth_token.strip() != auth_token:
            return "auth_token contains leading/trailing whitespace"

        # All validations passed
        return True


# Node Registration
NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (API)"
}