"""{{NodeName}} - Model loader ComfyUI node (V3 schema).

{{NodeDesc}}

Loads a model file from a ComfyUI models folder, following the same pattern
as the built-in loaders (see comfy_extras/nodes_upscale_model.py). The model
dropdown is populated from the folder via folder_paths.

Author: {{Author}}
"""

import os

import comfy.utils
import folder_paths

from comfy_api.latest import IO

# The models subfolder to load from (a key registered in folder_paths).
# Built-in keys include "checkpoints", "loras", "vae", "upscale_models", ...
# To use a custom folder, register it once at import time, e.g.:
#   folder_paths.add_model_folder_path("{{NodeNameLower}}", os.path.join(folder_paths.models_dir, "{{NodeNameLower}}"))
MODEL_FOLDER = "checkpoints"


class {{NodeName}}(IO.ComfyNode):
    """Load a model file and hand its state dict to downstream nodes."""

    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="{{NodeName}}",
            display_name="{{NodeName}} (Model Loader)",
            category="custom/loaders",
            description="{{NodeDesc}}",
            inputs=[
                IO.Combo.Input(
                    "model_name",
                    options=folder_paths.get_filename_list(MODEL_FOLDER),
                    tooltip=f"Model file from the models/{MODEL_FOLDER} folder.",
                ),
            ],
            outputs=[
                # A custom type keeps the wire type explicit; downstream nodes
                # accept it with IO.Custom("{{NodeName}}_MODEL").Input(...).
                IO.Custom("{{NodeName}}_MODEL").Output("model"),
                IO.String.Output("model_info"),
            ],
        )

    @classmethod
    def execute(cls, model_name) -> IO.NodeOutput:
        model_path = folder_paths.get_full_path_or_raise(MODEL_FOLDER, model_name)

        # safe_load handles .safetensors and torch checkpoints safely.
        state_dict = comfy.utils.load_torch_file(model_path, safe_load=True)

        # Replace this with your model construction, e.g. wrap the state dict
        # in your architecture class, .eval() it, and move it to the right
        # device with comfy.model_management.get_torch_device().
        model = state_dict

        size_mb = os.path.getsize(model_path) / (1024 * 1024)
        model_info = f"Loaded {model_name}: {len(state_dict)} tensors, {size_mb:.1f} MB"
        return IO.NodeOutput(model, model_info)

    @classmethod
    def fingerprint_inputs(cls, model_name):
        """Re-load when the file on disk changes, not just when the name does."""
        try:
            model_path = folder_paths.get_full_path_or_raise(MODEL_FOLDER, model_name)
            return f"{model_name}:{os.path.getmtime(model_path)}"
        except Exception:
            return float("nan")
