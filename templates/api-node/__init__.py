"""{{NodeName}} - API-focused ComfyUI node package (V3 schema).

Author: {{Author}}
"""

from comfy_api.latest import ComfyExtension, IO

from .src.{{NodeNameLower}} import {{NodeName}}


class {{NodeName}}Extension(ComfyExtension):
    async def get_node_list(self) -> list[type[IO.ComfyNode]]:
        return [{{NodeName}}]


async def comfy_entrypoint() -> {{NodeName}}Extension:
    return {{NodeName}}Extension()
