"""{{NodeName}} - ComfyUI custom node package (V3 schema).

Author: {{Author}}
"""

from comfy_api.latest import ComfyExtension, IO

from .src.{{NodeNameLower}} import {{NodeName}}

WEB_DIRECTORY = "./js"


class {{NodeName}}Extension(ComfyExtension):
    async def get_node_list(self) -> list[type[IO.ComfyNode]]:
        return [{{NodeName}}]


async def comfy_entrypoint() -> {{NodeName}}Extension:
    return {{NodeName}}Extension()
