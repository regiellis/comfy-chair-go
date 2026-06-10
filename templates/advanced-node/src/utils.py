"""Processing helpers for {{NodeName}}.

Small, dependency-free tensor utilities used by the node. Images in ComfyUI
are float tensors shaped (batch, height, width, channels) in the 0..1 range;
masks are (batch, height, width).

Author: {{Author}}
"""

import json

import torch


def parse_settings(settings_json: str) -> dict | None:
    """Parse the advanced-settings JSON. Returns None when invalid."""
    if not settings_json or not settings_json.strip():
        return {}
    try:
        settings = json.loads(settings_json)
    except json.JSONDecodeError:
        return None
    return settings if isinstance(settings, dict) else None


def adjust_brightness(image: torch.Tensor, strength: float) -> torch.Tensor:
    """Brighten the image proportionally to strength."""
    return torch.clamp(image * (1.0 + strength * 0.2), 0.0, 1.0)


def adjust_contrast(image: torch.Tensor, strength: float, settings: dict | None = None) -> torch.Tensor:
    """Increase contrast around the mean, with an optional gamma from settings."""
    mean = image.mean(dim=(1, 2), keepdim=True)
    contrasted = torch.clamp((image - mean) * (1.0 + strength * 0.5) + mean, 0.0, 1.0)
    gamma = (settings or {}).get("gamma")
    if isinstance(gamma, (int, float)) and gamma > 0:
        contrasted = torch.clamp(contrasted ** (1.0 / float(gamma)), 0.0, 1.0)
    return contrasted


def invert(image: torch.Tensor) -> torch.Tensor:
    """Invert the image colors."""
    return 1.0 - image


def blend_with_mask(original: torch.Tensor, processed: torch.Tensor, mask: torch.Tensor) -> torch.Tensor:
    """Apply `processed` only where the mask is set; keep `original` elsewhere."""
    weights = mask.unsqueeze(-1).clamp(0.0, 1.0)
    return original * (1.0 - weights) + processed * weights
