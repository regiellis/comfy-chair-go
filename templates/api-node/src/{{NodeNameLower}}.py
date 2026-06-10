"""{{NodeName}} - API-focused ComfyUI node (V3 schema).

{{NodeDesc}}

Calls an external HTTP API from inside a workflow. V3 supports async
`execute`, so requests don't block the execution loop.

Author: {{Author}}
"""

import json
import re
import time

import aiohttp

from comfy_api.latest import IO

_URL_PATTERN = re.compile(r"^https?://[^\s/$.?#].[^\s]*$", re.IGNORECASE)


class {{NodeName}}(IO.ComfyNode):
    """Make an HTTP request and return the response."""

    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="{{NodeName}}",
            display_name="{{NodeName}} (API)",
            category="custom/api",
            description="{{NodeDesc}}",
            inputs=[
                IO.String.Input(
                    "api_url",
                    default="https://api.example.com/endpoint",
                    tooltip="API endpoint URL.",
                ),
                IO.Combo.Input(
                    "method",
                    options=["GET", "POST", "PUT", "DELETE", "PATCH"],
                    default="GET",
                    tooltip="HTTP method.",
                ),
                IO.Int.Input("timeout", default=30, min=1, max=300, tooltip="Request timeout in seconds."),
                IO.String.Input(
                    "headers",
                    optional=True,
                    multiline=True,
                    default="{}",
                    tooltip="Request headers as a JSON object.",
                    advanced=True,
                ),
                IO.String.Input(
                    "body",
                    optional=True,
                    multiline=True,
                    default="",
                    tooltip="Request body (for POST/PUT/PATCH).",
                ),
                IO.String.Input(
                    "auth_token",
                    optional=True,
                    default="",
                    tooltip="Bearer token added as an Authorization header.",
                    advanced=True,
                ),
            ],
            outputs=[
                IO.String.Output("response_text"),
                IO.Int.Output("status_code"),
                IO.Float.Output("response_time", tooltip="Round-trip time in seconds."),
            ],
        )

    @classmethod
    async def execute(cls, api_url, method, timeout, headers="{}", body="", auth_token="") -> IO.NodeOutput:
        try:
            parsed_headers = json.loads(headers) if headers else {}
        except json.JSONDecodeError:
            parsed_headers = {}
        if auth_token:
            parsed_headers["Authorization"] = f"Bearer {auth_token}"
        if body and method in ("POST", "PUT", "PATCH"):
            parsed_headers.setdefault("Content-Type", "application/json")

        start = time.monotonic()
        try:
            client_timeout = aiohttp.ClientTimeout(total=timeout)
            async with aiohttp.ClientSession(timeout=client_timeout) as session:
                async with session.request(
                    method=method,
                    url=api_url,
                    headers=parsed_headers,
                    data=body if body else None,
                ) as response:
                    response_text = await response.text()
                    return IO.NodeOutput(response_text, response.status, time.monotonic() - start)
        except TimeoutError:
            return IO.NodeOutput("Request timeout", 408, float(timeout))
        except aiohttp.ClientError as e:
            return IO.NodeOutput(f"API request failed: {e}", 0, time.monotonic() - start)

    @classmethod
    def validate_inputs(cls, api_url, method, timeout, headers="{}", body="", auth_token="") -> bool | str:
        if not api_url:
            return "api_url is required"
        if not _URL_PATTERN.match(api_url):
            return f"Invalid URL format: {api_url}. Must start with http:// or https://"
        if headers:
            try:
                if not isinstance(json.loads(headers), dict):
                    return "headers must be a JSON object"
            except json.JSONDecodeError as e:
                return f"Invalid headers JSON: {e}"
        if body and method in ("POST", "PUT", "PATCH"):
            stripped = body.strip()
            if stripped.startswith(("{", "[")):
                try:
                    json.loads(body)
                except json.JSONDecodeError as e:
                    return f"Invalid body JSON: {e}"
        return True

    @classmethod
    def fingerprint_inputs(cls, api_url, method, timeout, headers="{}", body="", auth_token=""):
        """Caching control (V1's IS_CHANGED).

        External APIs can return different data for identical inputs. Returning
        NaN forces a fresh request on every run; switch to hashing the inputs
        if your endpoint is stable and you want workflow-level caching.
        """
        return float("nan")
