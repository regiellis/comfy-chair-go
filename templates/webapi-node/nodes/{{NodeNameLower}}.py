class {{NodeName}}:
    """Core logic for {{NodeName}} node.

    Extend methods or add helpers as needed.
    """

    @classmethod
    def INPUT_TYPES(cls):
        return {
            "required": {
                "topic": ("STRING", {"default": "demo", "multiline": False}),
            },
        }

    RETURN_TYPES = ("STRING",)
    FUNCTION = "run"
    CATEGORY = "custom"
    DISPLAY_NAME = "{{NodeName}}"

    def run(self, topic: str):
        return (f"Generated for {topic}",)

NODE_CLASS_MAPPINGS = {"{{NodeName}}": {{NodeName}}}
NODE_DISPLAY_NAME_MAPPINGS = {"{{NodeName}}": "{{NodeName}}"}
