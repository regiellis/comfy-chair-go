class {{NodeName}}Node:
    """Minimal working custom node.
    Edit this class to implement your node logic.
    """
    @classmethod
    def INPUT_TYPES(cls):
        return {"required": {}}

    RETURN_TYPES = ()
    FUNCTION = "run"
    CATEGORY = "custom"

    def run(self):
        return ()
