"""{{NodeName}} - Advanced ComfyUI node (V3 schema) with image processing and preview.

{{NodeDesc}}

Demonstrates V3 patterns beyond the basics:
- IMAGE/MASK tensor inputs and a slider widget
- An inline preview via UI.PreviewImage
- Hidden inputs (unique_id) for client-server communication
- Schema-level validation plus semantic validate_inputs

Author: {{Author}}
"""

from comfy_api.latest import IO, UI

from .utils import adjust_brightness, adjust_contrast, blend_with_mask, invert, parse_settings


class {{NodeName}}(IO.ComfyNode):
    """Image processing node with selectable modes and an optional preview."""

    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="{{NodeName}}",
            display_name="{{NodeName}} (Advanced)",
            category="custom/advanced",
            description="{{NodeDesc}}",
            inputs=[
                IO.Image.Input("image", tooltip="Primary input image (B, H, W, C tensor)."),
                IO.Combo.Input(
                    "processing_mode",
                    options=["standard", "enhanced", "experimental"],
                    default="standard",
                    tooltip="Processing algorithm to use.",
                ),
                IO.Float.Input(
                    "strength",
                    default=1.0,
                    min=0.0,
                    max=2.0,
                    step=0.05,
                    display_mode=IO.NumberDisplay.slider,
                    tooltip="Processing strength (0.0 = no effect, 2.0 = maximum).",
                ),
                IO.Boolean.Input(
                    "enable_preview",
                    default=True,
                    tooltip="Show an inline preview of the processed image.",
                ),
                IO.Mask.Input(
                    "mask",
                    optional=True,
                    tooltip="Optional mask limiting where processing is applied.",
                ),
                IO.String.Input(
                    "settings_json",
                    optional=True,
                    multiline=True,
                    default="{}",
                    tooltip="Advanced settings as JSON, e.g. {\"gamma\": 1.2}.",
                    advanced=True,
                ),
            ],
            outputs=[
                IO.Image.Output("output_image"),
                IO.String.Output("status"),
            ],
            hidden=[IO.Hidden.unique_id],
        )

    @classmethod
    def execute(cls, image, processing_mode, strength, enable_preview, mask=None, settings_json="{}") -> IO.NodeOutput:
        settings = parse_settings(settings_json)

        if processing_mode == "standard":
            processed = adjust_brightness(image, strength)
        elif processing_mode == "enhanced":
            processed = adjust_contrast(adjust_brightness(image, strength), strength, settings)
        else:  # experimental
            processed = invert(adjust_brightness(image, strength))

        if mask is not None:
            processed = blend_with_mask(image, processed, mask)

        # cls.hidden holds the requested hidden inputs (e.g. cls.hidden.unique_id).
        status = f"{{NodeName}} processed {tuple(image.shape)} using {processing_mode} mode"

        ui = UI.PreviewImage(processed, cls=cls) if enable_preview else None
        return IO.NodeOutput(processed, status, ui=ui)

    @classmethod
    def validate_inputs(cls, image=None, processing_mode="standard", strength=1.0,
                        enable_preview=True, mask=None, settings_json="{}") -> bool | str:
        if settings_json and parse_settings(settings_json) is None:
            return "settings_json is not valid JSON"
        return True
