import os

from .src.{{NodeNameLower}} import {{NodeName}}

WEB_DIRECTORY = os.path.join(os.path.dirname(__file__), "js")
CSS_DIRECTORY = os.path.join(os.path.dirname(__file__), "css")

NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}},
}
NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{NodeName}}",
}

__all__ = ["NODE_CLASS_MAPPINGS", "NODE_DISPLAY_NAME_MAPPINGS", "WEB_DIRECTORY", "CSS_DIRECTORY"]
