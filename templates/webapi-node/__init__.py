from .nodes import NODE_CLASS_MAPPINGS, NODE_DISPLAY_NAME_MAPPINGS
WEB_DIRECTORY = "./web"

try:
    from . import api  # noqa: F401
    import server
    api.setup_api_routes(server.PromptServer.instance.app)
except ImportError:
    pass  # Server not available (offline tooling)
except Exception as e:  # pragma: no cover
    print(f"Warning: Could not setup API routes: {e}")
