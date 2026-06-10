"""{{NodeName}} - Web/API ComfyUI node package (V3 schema).

Registers the node via the V3 extension entrypoint and bootstraps the aiohttp
routes on the running ComfyUI server. The API layer failing never breaks node
registration.

Author: {{Author}}
"""

from comfy_api.latest import ComfyExtension, IO

from .nodes.{{NodeNameLower}} import {{NodeName}}

WEB_DIRECTORY = "./web"

try:
    from . import api  # noqa: F401
    import server
    api.setup_api_routes(server.PromptServer.instance.app)
except ImportError:
    pass  # Server not available (offline tooling)
except Exception as e:  # pragma: no cover
    print(f"Warning: Could not setup API routes: {e}")


class {{NodeName}}Extension(ComfyExtension):
    async def get_node_list(self) -> list[type[IO.ComfyNode]]:
        return [{{NodeName}}]


async def comfy_entrypoint() -> {{NodeName}}Extension:
    return {{NodeName}}Extension()
