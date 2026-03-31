"""
Node utilities for {{NodeName}}

This module provides utility classes for node settings persistence
and UI component building for ComfyUI nodes.

Author: {{Author}}
"""

import os
import json
from typing import Any, Dict, List, Optional, Union, Callable
from pathlib import Path


class NodeSettings:
    """
    Persistent settings manager for ComfyUI nodes.

    Handles saving/loading configuration to disk with support for:
    - Default values
    - Type validation
    - Automatic file management
    - Settings migration

    Example:
        settings = NodeSettings(node_name="my_node")
        settings.set("strength", 0.75)
        settings.set("mode", "enhanced")
        value = settings.get("strength", default=1.0)
    """

    # Default settings directory within ComfyUI
    DEFAULT_SETTINGS_DIR = "user/node_settings"

    def __init__(self,
                 node_name: str,
                 settings_dir: Optional[str] = None,
                 defaults: Optional[Dict[str, Any]] = None):
        """
        Initialize the settings manager.

        Args:
            node_name: Identifier for this node's settings file
            settings_dir: Custom directory for settings storage
            defaults: Default values for settings
        """
        self.node_name = node_name
        self.defaults = defaults or {}
        self._settings: Dict[str, Any] = {}
        self._dirty = False

        # Determine settings path
        if settings_dir:
            self._settings_dir = Path(settings_dir)
        else:
            # Try to use ComfyUI's base path if available
            comfy_path = self._get_comfy_base_path()
            self._settings_dir = comfy_path / self.DEFAULT_SETTINGS_DIR

        self._settings_file = self._settings_dir / f"{node_name}_settings.json"

        # Load existing settings
        self._load()

    def _get_comfy_base_path(self) -> Path:
        """Get the ComfyUI base directory."""
        # Try environment variable first
        comfy_path = os.environ.get("COMFYUI_PATH")
        if comfy_path:
            return Path(comfy_path)

        # Try to find it relative to current working directory
        cwd = Path.cwd()

        # Check if we're in a custom_nodes subdirectory
        if "custom_nodes" in cwd.parts:
            idx = cwd.parts.index("custom_nodes")
            return Path(*cwd.parts[:idx])

        # Default to cwd
        return cwd

    def _ensure_dir(self) -> None:
        """Ensure the settings directory exists."""
        self._settings_dir.mkdir(parents=True, exist_ok=True)

    def _load(self) -> None:
        """Load settings from disk."""
        if self._settings_file.exists():
            try:
                with open(self._settings_file, "r", encoding="utf-8") as f:
                    self._settings = json.load(f)
            except (json.JSONDecodeError, IOError) as e:
                print(f"Warning: Could not load settings for {self.node_name}: {e}")
                self._settings = {}
        else:
            self._settings = {}

    def _save(self) -> None:
        """Save settings to disk."""
        try:
            self._ensure_dir()
            with open(self._settings_file, "w", encoding="utf-8") as f:
                json.dump(self._settings, f, indent=2, ensure_ascii=False)
            self._dirty = False
        except IOError as e:
            print(f"Warning: Could not save settings for {self.node_name}: {e}")

    def get(self, key: str, default: Any = None) -> Any:
        """
        Get a setting value.

        Args:
            key: Setting key to retrieve
            default: Default value if key not found (overrides class defaults)

        Returns:
            The setting value, or default if not found
        """
        if key in self._settings:
            return self._settings[key]
        if key in self.defaults:
            return self.defaults[key]
        return default

    def set(self, key: str, value: Any, save: bool = True) -> None:
        """
        Set a setting value.

        Args:
            key: Setting key to set
            value: Value to store
            save: Whether to immediately save to disk
        """
        self._settings[key] = value
        self._dirty = True
        if save:
            self._save()

    def update(self, settings: Dict[str, Any], save: bool = True) -> None:
        """
        Update multiple settings at once.

        Args:
            settings: Dictionary of settings to update
            save: Whether to immediately save to disk
        """
        self._settings.update(settings)
        self._dirty = True
        if save:
            self._save()

    def delete(self, key: str, save: bool = True) -> bool:
        """
        Delete a setting.

        Args:
            key: Setting key to delete
            save: Whether to immediately save to disk

        Returns:
            True if the key existed and was deleted
        """
        if key in self._settings:
            del self._settings[key]
            self._dirty = True
            if save:
                self._save()
            return True
        return False

    def clear(self, save: bool = True) -> None:
        """
        Clear all settings.

        Args:
            save: Whether to immediately save to disk
        """
        self._settings = {}
        self._dirty = True
        if save:
            self._save()

    def reset_to_defaults(self, save: bool = True) -> None:
        """
        Reset settings to default values.

        Args:
            save: Whether to immediately save to disk
        """
        self._settings = dict(self.defaults)
        self._dirty = True
        if save:
            self._save()

    def all(self) -> Dict[str, Any]:
        """
        Get all current settings merged with defaults.

        Returns:
            Dictionary of all settings
        """
        result = dict(self.defaults)
        result.update(self._settings)
        return result

    def export_json(self) -> str:
        """
        Export settings as JSON string.

        Returns:
            JSON-formatted settings string
        """
        return json.dumps(self._settings, indent=2)

    def import_json(self, json_str: str, merge: bool = False, save: bool = True) -> bool:
        """
        Import settings from JSON string.

        Args:
            json_str: JSON-formatted settings string
            merge: If True, merge with existing; if False, replace
            save: Whether to immediately save to disk

        Returns:
            True if import succeeded
        """
        try:
            imported = json.loads(json_str)
            if merge:
                self._settings.update(imported)
            else:
                self._settings = imported
            self._dirty = True
            if save:
                self._save()
            return True
        except json.JSONDecodeError:
            return False

    @property
    def is_dirty(self) -> bool:
        """Check if there are unsaved changes."""
        return self._dirty

    def save_if_dirty(self) -> bool:
        """
        Save settings only if there are unsaved changes.

        Returns:
            True if settings were saved
        """
        if self._dirty:
            self._save()
            return True
        return False

    def __contains__(self, key: str) -> bool:
        """Check if a setting exists."""
        return key in self._settings or key in self.defaults

    def __getitem__(self, key: str) -> Any:
        """Get a setting value using bracket notation."""
        return self.get(key)

    def __setitem__(self, key: str, value: Any) -> None:
        """Set a setting value using bracket notation."""
        self.set(key, value)


class UIComponents:
    """
    Builder class for creating ComfyUI-compatible UI component definitions.

    Provides fluent interface for building input specifications, widgets,
    and UI metadata for ComfyUI nodes.

    Example:
        ui = UIComponents()
        inputs = ui.image_input("input_image", tooltip="Main input")
        inputs.update(ui.slider("strength", 0.0, 2.0, default=1.0))
    """

    def __init__(self):
        """Initialize the UI components builder."""
        self._components: Dict[str, Dict] = {}
        self._callbacks: Dict[str, Callable] = {}

    # -------------------------------------------------------------------------
    # Input Type Builders
    # -------------------------------------------------------------------------

    def image_input(self,
                    name: str,
                    tooltip: Optional[str] = None,
                    optional: bool = False) -> Dict[str, tuple]:
        """
        Create an image input specification.

        Args:
            name: Input parameter name
            tooltip: Hover tooltip text
            optional: Whether this input is optional

        Returns:
            Input specification dictionary
        """
        config = {}
        if tooltip:
            config["tooltip"] = tooltip

        spec = (name, ("IMAGE", config))
        self._components[name] = spec

        return {name: ("IMAGE", config)} if not optional else {}

    def mask_input(self,
                   name: str = "mask",
                   tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a mask input specification.

        Args:
            name: Input parameter name
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {}
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("MASK", config)}

    def model_input(self,
                    name: str = "model",
                    tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a model input specification.

        Args:
            name: Input parameter name
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {}
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("MODEL", config)}

    def latent_input(self,
                     name: str = "latent",
                     tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a latent input specification.

        Args:
            name: Input parameter name
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {}
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("LATENT", config)}

    def conditioning_input(self,
                           name: str = "conditioning",
                           tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a conditioning input specification.

        Args:
            name: Input parameter name
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {}
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("CONDITIONING", config)}

    # -------------------------------------------------------------------------
    # Widget Builders
    # -------------------------------------------------------------------------

    def slider(self,
               name: str,
               min_val: float,
               max_val: float,
               default: Optional[float] = None,
               step: float = 0.01,
               tooltip: Optional[str] = None,
               display: str = "slider") -> Dict[str, tuple]:
        """
        Create a float slider widget specification.

        Args:
            name: Parameter name
            min_val: Minimum value
            max_val: Maximum value
            default: Default value (defaults to min_val)
            step: Step increment
            tooltip: Hover tooltip text
            display: Display mode ("slider" or "number")

        Returns:
            Input specification dictionary
        """
        config = {
            "min": min_val,
            "max": max_val,
            "step": step,
            "display": display,
            "default": default if default is not None else min_val,
        }
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("FLOAT", config)}

    def int_slider(self,
                   name: str,
                   min_val: int,
                   max_val: int,
                   default: Optional[int] = None,
                   step: int = 1,
                   tooltip: Optional[str] = None,
                   display: str = "slider") -> Dict[str, tuple]:
        """
        Create an integer slider widget specification.

        Args:
            name: Parameter name
            min_val: Minimum value
            max_val: Maximum value
            default: Default value (defaults to min_val)
            step: Step increment
            tooltip: Hover tooltip text
            display: Display mode ("slider" or "number")

        Returns:
            Input specification dictionary
        """
        config = {
            "min": min_val,
            "max": max_val,
            "step": step,
            "display": display,
            "default": default if default is not None else min_val,
        }
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("INT", config)}

    def dropdown(self,
                 name: str,
                 options: List[str],
                 default: Optional[str] = None,
                 tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a dropdown selection widget specification.

        Args:
            name: Parameter name
            options: List of selectable options
            default: Default selected option
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {}
        if default:
            config["default"] = default
        if tooltip:
            config["tooltip"] = tooltip

        return {name: (options, config)}

    def checkbox(self,
                 name: str,
                 default: bool = False,
                 tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a boolean checkbox widget specification.

        Args:
            name: Parameter name
            default: Default checked state
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {"default": default}
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("BOOLEAN", config)}

    def text_input(self,
                   name: str,
                   default: str = "",
                   multiline: bool = False,
                   placeholder: Optional[str] = None,
                   tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a text input widget specification.

        Args:
            name: Parameter name
            default: Default text value
            multiline: Whether to use multiline input
            placeholder: Placeholder text
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {
            "default": default,
            "multiline": multiline,
        }
        if placeholder:
            config["placeholder"] = placeholder
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("STRING", config)}

    def seed_input(self,
                   name: str = "seed",
                   tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a seed input widget with randomize control.

        Args:
            name: Parameter name
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {
            "default": 0,
            "min": 0,
            "max": 0xffffffffffffffff,
        }
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("INT", config)}

    def color_picker(self,
                     name: str,
                     default: str = "#ffffff",
                     tooltip: Optional[str] = None) -> Dict[str, tuple]:
        """
        Create a color picker widget specification.

        Args:
            name: Parameter name
            default: Default color in hex format
            tooltip: Hover tooltip text

        Returns:
            Input specification dictionary
        """
        config = {
            "default": default,
            "display": "color",
        }
        if tooltip:
            config["tooltip"] = tooltip

        return {name: ("STRING", config)}

    # -------------------------------------------------------------------------
    # Compound Builders
    # -------------------------------------------------------------------------

    def build_inputs(self,
                     required: Optional[Dict] = None,
                     optional: Optional[Dict] = None,
                     hidden: Optional[Dict] = None) -> Dict[str, Dict]:
        """
        Build a complete INPUT_TYPES specification.

        Args:
            required: Required input specifications
            optional: Optional input specifications
            hidden: Hidden input specifications

        Returns:
            Complete INPUT_TYPES dictionary
        """
        result = {}

        if required:
            result["required"] = required
        if optional:
            result["optional"] = optional
        if hidden:
            result["hidden"] = hidden

        return result

    def standard_hidden_inputs(self) -> Dict[str, str]:
        """
        Get standard hidden inputs for nodes.

        Returns:
            Dictionary of standard hidden inputs
        """
        return {
            "node_id": "UNIQUE_ID",
            "extra_pnginfo": "EXTRA_PNGINFO",
        }

    # -------------------------------------------------------------------------
    # UI State Management
    # -------------------------------------------------------------------------

    def register_callback(self, event: str, callback: Callable) -> None:
        """
        Register a callback for UI events.

        Args:
            event: Event name (e.g., "on_change", "on_submit")
            callback: Callback function
        """
        self._callbacks[event] = callback

    def trigger_callback(self, event: str, *args, **kwargs) -> Any:
        """
        Trigger a registered callback.

        Args:
            event: Event name to trigger
            *args: Positional arguments for callback
            **kwargs: Keyword arguments for callback

        Returns:
            Callback result, or None if no callback registered
        """
        if event in self._callbacks:
            return self._callbacks[event](*args, **kwargs)
        return None

    def create_group(self,
                     name: str,
                     components: List[Dict],
                     collapsed: bool = False) -> Dict[str, Any]:
        """
        Create a grouped set of UI components.

        Args:
            name: Group display name
            components: List of component specifications
            collapsed: Whether group starts collapsed

        Returns:
            Group specification dictionary
        """
        merged = {}
        for comp in components:
            merged.update(comp)

        return {
            "_group": name,
            "_collapsed": collapsed,
            "components": merged,
        }

    @staticmethod
    def validate_spec(spec: Dict) -> bool:
        """
        Validate an input specification dictionary.

        Args:
            spec: Specification to validate

        Returns:
            True if valid, False otherwise
        """
        if not isinstance(spec, dict):
            return False

        for key, value in spec.items():
            if not isinstance(key, str):
                return False
            if not isinstance(value, tuple) or len(value) != 2:
                return False

        return True
