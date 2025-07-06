"""
{{NodeName}} - API-focused ComfyUI Node Package

This package provides API integration functionality for ComfyUI nodes.

Author: {{Author}}
"""

from .src.{{NodeNameLower}} import {{NodeName}}

NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (API)"
}

__all__ = ["{{NodeName}}"]