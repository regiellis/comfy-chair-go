import typing

class {{NodeName}}:
    """
    Example ComfyUI Custom Node

    This template provides a comprehensive starting point for building your own ComfyUI custom node.
    Edit this class to implement your node logic, define inputs/outputs, and add options.

    Features:
    - Customizable input and output types
    - Node options (with default values)
    - Category and display name
    - Example docstrings and type hints
    - Support for batch processing
    - Example error handling
    """

    @classmethod
    def INPUT_TYPES(cls):
        return {
            "required": {
                "input_text": ("STRING", {"default": "Hello, Comfy!", "multiline": False}),
                "input_number": ("INT", {"default": 42, "min": 0, "max": 100}),
                "input_bool": ("BOOLEAN", {"default": True}),
                "input_choice": ("CHOICE", {"choices": ["option1", "option2", "option3"], "default": "option1"}),
            },
            "optional": {
                "input_optional": ("STRING", {"default": "Optional value"}),
            },
        }

    RETURN_TYPES = ("STRING", "INT", "BOOLEAN", "CHOICE")
    RETURN_NAMES = ("output_text", "output_number", "output_bool", "output_choice")
    FUNCTION = "run"
    CATEGORY = "custom"
    DISPLAY_NAME = "Example Custom Node"

    def run(self, input_text: str, input_number: int, input_bool: bool, input_choice: str, input_optional: typing.Optional[str] = None):
        """
        Main node logic. Processes inputs and returns outputs.

        Args:
            input_text (str): Text input from the user.
            input_number (int): Integer input.
            input_bool (bool): Boolean input.
            input_choice (str): Choice input.
            input_optional (str, optional): Optional string input.

        Returns:
            tuple: Outputs matching RETURN_TYPES.
        """
        # Example processing logic
        output_text = f"You entered: {input_text}"
        output_number = input_number * 2
        output_bool = not input_bool
        output_choice = input_choice.upper()

        # Example: use optional input if provided
        if input_optional:
            output_text += f" (Optional: {input_optional})"

        # Example error handling
        if input_number < 0:
            raise ValueError("input_number must be non-negative")

        return (output_text, output_number, output_bool, output_choice)

    @classmethod
    def IS_CHANGED(cls, input_text, input_number, input_bool, input_choice, input_optional=None):
        """
        Determines if the node needs re-execution based on input changes.

        ComfyUI uses this method for caching: if the return value matches
        a previous execution, cached results are used instead of re-running.

        Return options:
        - float('nan'): Always re-run the node (no caching)
        - Hash/value of inputs: Re-run only when inputs change
        - Constant value: Always use cached result (if available)

        Customize this method based on your node's behavior:
        - For deterministic nodes: hash the inputs
        - For nodes with randomness or external state: return float('nan')
        - For nodes reading files: include file modification time in hash

        Example customizations:
            # Always re-run (for random/non-deterministic nodes):
            return float('nan')

            # Hash specific inputs for selective caching:
            return hash((input_text, input_number))
        """
        # Default: hash all inputs for deterministic caching
        return hash((input_text, input_number, input_bool, input_choice, input_optional))

    @classmethod
    def VALIDATE_INPUTS(cls, input_text, input_number, input_bool, input_choice, input_optional=None):
        """
        Validates inputs before the node executes.

        This method is called before execution to catch invalid inputs early,
        providing better error messages and preventing wasted computation.

        Return values:
        - True: All inputs are valid, proceed with execution
        - String: Error message describing the validation failure

        Note: Type checking is handled by ComfyUI automatically based on INPUT_TYPES.
        Use this method for semantic validation (value ranges, format checks, etc.).

        Example customizations:
            # Validate text format:
            if input_text and not input_text.strip():
                return "input_text cannot be empty or whitespace only"

            # Validate number ranges beyond INPUT_TYPES constraints:
            if input_number % 2 != 0:
                return "input_number must be an even number"
        """
        # Validate input_text is not empty if provided
        if input_text is not None and isinstance(input_text, str) and len(input_text) == 0:
            return "input_text cannot be empty"

        # Validate input_number range (additional semantic check)
        if input_number is not None and input_number < 0:
            return "input_number must be non-negative"

        # All validations passed
        return True

    @classmethod
    def BATCHED(cls):
        """
        Optional: Return True if this node supports batched processing.
        """
        return False

# Starter Node
# This is a basic example node that can be used as a starting point for your custom nodes.
# You can extend this class to create more complex nodes with additional functionality.
# To use this node, save it in the appropriate directory and load it in ComfyUI.
# Make sure to test your node thoroughly and handle any edge cases.
