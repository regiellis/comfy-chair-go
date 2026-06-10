"""{{NodeName}} - ComfyUI custom node (V3 schema).

{{NodeDesc}}

This starter node demonstrates the V3 node API: a class-based schema
(`define_schema`), classmethod execution, and typed inputs/outputs.
V3 reference: https://docs.comfy.org/custom-nodes/v3/overview

Author: {{Author}}
"""

from comfy_api.latest import IO


class {{NodeName}}(IO.ComfyNode):
    """Starter node showing the common V3 widget input and output types."""

    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="{{NodeName}}",
            display_name="{{NodeName}}",
            category="custom",
            description="{{NodeDesc}}",
            inputs=[
                IO.String.Input(
                    "input_text",
                    default="Hello, Comfy!",
                    tooltip="Text to process.",
                ),
                IO.Int.Input(
                    "input_number",
                    default=42,
                    min=0,
                    max=100,
                    tooltip="A bounded integer.",
                ),
                IO.Boolean.Input("input_bool", default=True),
                IO.Combo.Input(
                    "input_choice",
                    options=["option1", "option2", "option3"],
                    default="option1",
                ),
                IO.String.Input(
                    "input_optional",
                    optional=True,
                    default="",
                    tooltip="Optional extra text; appended to the output when set.",
                ),
            ],
            outputs=[
                IO.String.Output("output_text"),
                IO.Int.Output("output_number"),
                IO.Boolean.Output("output_bool"),
                IO.String.Output("output_choice"),
            ],
        )

    @classmethod
    def execute(cls, input_text, input_number, input_bool, input_choice, input_optional="") -> IO.NodeOutput:
        """Node logic. Return values in the same order as the schema outputs."""
        output_text = f"You entered: {input_text}"
        if input_optional:
            output_text += f" (Optional: {input_optional})"

        output_number = input_number * 2
        output_bool = not input_bool
        output_choice = input_choice.upper()

        return IO.NodeOutput(output_text, output_number, output_bool, output_choice)

    @classmethod
    def validate_inputs(cls, input_text, input_number, input_bool, input_choice, input_optional="") -> bool | str:
        """Semantic validation before execution (V1's VALIDATE_INPUTS).

        Return True when valid, or an error string describing the problem.
        Type and range checks from the schema happen automatically.
        """
        if input_text is not None and not input_text.strip():
            return "input_text cannot be empty or whitespace only"
        return True

    # Caching: by default ComfyUI re-runs the node only when inputs change.
    # To customize (V1's IS_CHANGED), define fingerprint_inputs:
    #
    # @classmethod
    # def fingerprint_inputs(cls, input_text, input_number, **kwargs):
    #     return float("nan")  # always re-run (random/external-state nodes)
