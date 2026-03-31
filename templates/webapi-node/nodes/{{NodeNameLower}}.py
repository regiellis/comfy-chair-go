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

    @classmethod
    def IS_CHANGED(cls, topic):
        """
        Determines if the node needs re-execution based on input changes.

        For web/API endpoint nodes, consider:
        - Whether the endpoint returns dynamic data
        - Caching requirements for your use case
        - Rate limiting considerations

        Return options:
        - float('nan'): Always re-run (for dynamic/real-time data)
        - Hash of inputs: Re-run only when inputs change (for stable endpoints)

        Example customizations:
            # Always fetch fresh data:
            return float('nan')

            # Include timestamp for time-based refresh:
            import time
            return hash((topic, int(time.time() / 60)))  # Refresh every minute
        """
        # Default: hash inputs for caching
        # Change to float('nan') if your endpoint returns dynamic data
        return hash(topic)

    @classmethod
    def VALIDATE_INPUTS(cls, topic):
        """
        Validates inputs before the node executes.

        For web/API endpoint nodes, validate:
        - Required parameters are present
        - Parameter formats match expected patterns
        - Values are within acceptable ranges

        Return values:
        - True: All inputs are valid
        - String: Error message describing the validation failure

        Example customizations:
            # Validate topic format:
            if not topic.replace('_', '').replace('-', '').isalnum():
                return "topic must contain only alphanumeric characters, hyphens, and underscores"

            # Check topic length:
            if len(topic) > 100:
                return "topic cannot exceed 100 characters"
        """
        # Validate topic is not empty
        if not topic or not topic.strip():
            return "topic is required and cannot be empty"

        # Validate topic length (reasonable limit)
        if len(topic) > 500:
            return f"topic length ({len(topic)}) exceeds maximum of 500 characters"

        # All validations passed
        return True


NODE_CLASS_MAPPINGS = {"{{NodeName}}": {{NodeName}}}
NODE_DISPLAY_NAME_MAPPINGS = {"{{NodeName}}": "{{NodeName}}"}
