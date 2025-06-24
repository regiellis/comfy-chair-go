"""
{{NodeName}} - Model Loading ComfyUI Node Package

This package provides model loading and management functionality for ComfyUI nodes.

Author: {{Author}}
"""

from .src.{{NodeNameLower}} import {{NodeName}}

NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (Model Loader)"
}

__all__ = ["{{NodeName}}"]