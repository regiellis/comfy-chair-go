import os

from .nodes import NODE_CLASS_MAPPINGS, NODE_DISPLAY_NAME_MAPPINGS

WEB_DIRECTORY = os.path.join(os.path.dirname(__file__), "web")
CSS_DIRECTORY = os.path.join(os.path.dirname(__file__), "css")

__all__ = ["NODE_CLASS_MAPPINGS", "NODE_DISPLAY_NAME_MAPPINGS", "WEB_DIRECTORY", "CSS_DIRECTORY"]

try:
    from . import api  # noqa: F401
    import server
    api.setup_api_routes(server.PromptServer.instance.app)
except ImportError:
    pass  # Server not available (offline tooling)
except Exception as e:  # pragma: no cover
    print(f"Warning: Could not setup API routes: {e}")
