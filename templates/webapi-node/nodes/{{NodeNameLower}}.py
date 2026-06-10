"""{{NodeName}} - core logic and V3 node definition.

The business logic lives in plain functions (the "engine") so the aiohttp API
layer in api.py and the graph node share one implementation.

Author: {{Author}}
"""

from comfy_api.latest import IO


def generate(topic: str) -> str:
    """Engine: produce a result for the given topic.

    Replace this with your real logic; both the node and the HTTP endpoints
    call it.
    """
    if not topic or not topic.strip():
        raise ValueError("topic required")
    return f"Generated for {topic}"


class {{NodeName}}(IO.ComfyNode):
    """Graph-facing wrapper around the engine."""

    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="{{NodeName}}",
            display_name="{{NodeName}}",
            category="custom",
            description="{{NodeDesc}}",
            inputs=[
                IO.String.Input("topic", default="demo", tooltip="Topic to generate for."),
            ],
            outputs=[
                IO.String.Output("result"),
            ],
        )

    @classmethod
    def execute(cls, topic) -> IO.NodeOutput:
        return IO.NodeOutput(generate(topic))

    @classmethod
    def validate_inputs(cls, topic) -> bool | str:
        if not topic or not topic.strip():
            return "topic is required and cannot be empty"
        if len(topic) > 500:
            return f"topic length ({len(topic)}) exceeds maximum of 500 characters"
        return True
