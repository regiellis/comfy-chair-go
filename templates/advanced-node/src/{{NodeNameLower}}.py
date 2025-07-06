"""
{{NodeName}} - Advanced ComfyUI node with web UI components
{{NodeDescription}}

Author: {{Author}}
"""

import torch
import numpy as np
from typing import Dict, Any, Tuple, Optional, List
import json
import os
from ..utils import NodeSettings, UIComponents


class {{NodeName}}:
    """
    Advanced ComfyUI node with rich UI components and real-time preview capabilities.
    
    This node demonstrates advanced patterns including:
    - Custom UI components
    - Settings persistence
    - Real-time preview
    - Multiple processing modes
    """
    
    # ComfyUI Node Configuration
    RETURN_TYPES = ("IMAGE", "STRING", "DICT")
    RETURN_NAMES = ("output_image", "status_text", "metadata")
    FUNCTION = "process"
    CATEGORY = "{{NodeName}}/advanced"
    OUTPUT_NODE = True
    
    # UI Configuration
    UI_TYPE = "advanced"
    HAS_PREVIEW = True
    HAS_SETTINGS = True
    
    def __init__(self):
        """Initialize the advanced node with settings and UI state."""
        self.settings = NodeSettings(node_name="{{NodeNameLower}}")
        self.ui_components = UIComponents()
        self.preview_enabled = True
        self.processing_mode = "standard"
        
    @classmethod
    def INPUT_TYPES(cls) -> Dict[str, Any]:
        """
        Define input types with advanced UI components.
        
        Returns:
            Dict containing input specifications with UI enhancements
        """
        return {
            "required": {
                "input_image": ("IMAGE", {
                    "tooltip": "Primary input image for processing"
                }),
                "processing_mode": (["standard", "enhanced", "experimental"], {
                    "default": "standard",
                    "tooltip": "Processing algorithm to use"
                }),
                "strength": ("FLOAT", {
                    "default": 1.0,
                    "min": 0.0,
                    "max": 2.0,
                    "step": 0.1,
                    "display": "slider",
                    "tooltip": "Processing strength (0.0 = no effect, 2.0 = maximum)"
                }),
                "enable_preview": ("BOOLEAN", {
                    "default": True,
                    "tooltip": "Show real-time preview of processing"
                }),
            },
            "optional": {
                "mask": ("MASK", {
                    "tooltip": "Optional mask to limit processing area"
                }),
                "settings_json": ("STRING", {
                    "multiline": True,
                    "default": "{}",
                    "tooltip": "Advanced settings in JSON format"
                }),
                "custom_params": ("DICT", {
                    "tooltip": "Custom parameters from other nodes"
                }),
            },
            "hidden": {
                "node_id": "UNIQUE_ID",
                "extra_pnginfo": "EXTRA_PNGINFO",
            }
        }
    
    def process(self, 
                input_image: torch.Tensor,
                processing_mode: str,
                strength: float,
                enable_preview: bool,
                mask: Optional[torch.Tensor] = None,
                settings_json: str = "{}",
                custom_params: Optional[Dict] = None,
                node_id: str = "",
                extra_pnginfo: Optional[Dict] = None) -> Tuple[torch.Tensor, str, Dict]:
        """
        Main processing function with advanced features.
        
        Args:
            input_image: Input image tensor
            processing_mode: Algorithm mode to use
            strength: Processing strength
            enable_preview: Whether to enable preview
            mask: Optional processing mask
            settings_json: Advanced settings in JSON format
            custom_params: Custom parameters from other nodes
            node_id: Unique node identifier
            extra_pnginfo: Extra PNG metadata
            
        Returns:
            Tuple of (processed_image, status_text, metadata)
        """
        try:
            # Parse advanced settings
            settings = self._parse_settings(settings_json)
            
            # Update node settings
            self.settings.update({
                "processing_mode": processing_mode,
                "strength": strength,
                "enable_preview": enable_preview,
                **settings
            })
            
            # Initialize processing context
            context = self._create_processing_context(
                input_image, mask, custom_params, node_id
            )
            
            # Process image based on mode
            if processing_mode == "standard":
                output_image = self._process_standard(context, strength)
            elif processing_mode == "enhanced":
                output_image = self._process_enhanced(context, strength, settings)
            elif processing_mode == "experimental":
                output_image = self._process_experimental(context, strength, settings)
            else:
                raise ValueError(f"Unknown processing mode: {processing_mode}")
            
            # Apply mask if provided
            if mask is not None:
                output_image = self._apply_mask(output_image, input_image, mask)
            
            # Generate preview if enabled
            if enable_preview:
                self._generate_preview(output_image, node_id)
            
            # Create metadata
            metadata = self._create_metadata(context, settings)
            
            # Status message
            status_text = f"{{NodeName}} processed successfully using {processing_mode} mode"
            
            return (output_image, status_text, metadata)
            
        except Exception as e:
            error_msg = f"{{NodeName}} error: {str(e)}"
            # Return original image on error
            empty_metadata = {"error": str(e), "success": False}
            return (input_image, error_msg, empty_metadata)
    
    def _parse_settings(self, settings_json: str) -> Dict[str, Any]:
        """Parse and validate settings JSON."""
        try:
            settings = json.loads(settings_json) if settings_json else {}
            
            # Validate settings structure
            valid_keys = {
                "blur_radius", "sharpen_factor", "color_enhancement",
                "noise_reduction", "edge_preservation", "custom_filter"
            }
            
            return {k: v for k, v in settings.items() if k in valid_keys}
            
        except json.JSONDecodeError:
            return {}
    
    def _create_processing_context(self, 
                                   input_image: torch.Tensor, 
                                   mask: Optional[torch.Tensor],
                                   custom_params: Optional[Dict],
                                   node_id: str) -> Dict[str, Any]:
        """Create processing context with all necessary information."""
        batch_size, height, width, channels = input_image.shape
        
        context = {
            "image_shape": (batch_size, height, width, channels),
            "device": input_image.device,
            "dtype": input_image.dtype,
            "has_mask": mask is not None,
            "node_id": node_id,
            "custom_params": custom_params or {},
        }
        
        return context
    
    def _process_standard(self, context: Dict, strength: float) -> torch.Tensor:
        """Standard processing algorithm."""
        # Placeholder for standard processing
        # In a real implementation, this would contain your core algorithm
        input_image = context.get("input_image")
        
        # Example: simple brightness adjustment
        processed = input_image * (1.0 + strength * 0.2)
        return torch.clamp(processed, 0.0, 1.0)
    
    def _process_enhanced(self, context: Dict, strength: float, settings: Dict) -> torch.Tensor:
        """Enhanced processing with additional features."""
        # Placeholder for enhanced processing
        input_image = context.get("input_image")
        
        # Example: enhanced processing with settings
        blur_radius = settings.get("blur_radius", 1.0)
        sharpen_factor = settings.get("sharpen_factor", 0.5)
        
        # Apply processing (simplified example)
        processed = input_image * (1.0 + strength * 0.3)
        
        return torch.clamp(processed, 0.0, 1.0)
    
    def _process_experimental(self, context: Dict, strength: float, settings: Dict) -> torch.Tensor:
        """Experimental processing algorithms."""
        # Placeholder for experimental features
        input_image = context.get("input_image")
        
        # Example: experimental processing
        processed = input_image * (1.0 + strength * 0.5)
        
        return torch.clamp(processed, 0.0, 1.0)
    
    def _apply_mask(self, 
                    processed_image: torch.Tensor, 
                    original_image: torch.Tensor, 
                    mask: torch.Tensor) -> torch.Tensor:
        """Apply mask to blend processed and original images."""
        # Ensure mask has the right dimensions
        if mask.dim() == 3:
            mask = mask.unsqueeze(-1)  # Add channel dimension
        
        # Blend images using mask
        return processed_image * mask + original_image * (1.0 - mask)
    
    def _generate_preview(self, output_image: torch.Tensor, node_id: str) -> None:
        """Generate preview for the UI."""
        try:
            # Convert to PIL for preview
            from PIL import Image
            
            # Take first image from batch
            img_array = output_image[0].cpu().numpy()
            img_array = (img_array * 255).astype(np.uint8)
            
            preview_img = Image.fromarray(img_array)
            
            # Save preview (simplified - in real implementation, use ComfyUI's preview system)
            preview_path = f"/tmp/{{NodeNameLower}}_preview_{node_id}.png"
            preview_img.save(preview_path)
            
        except Exception as e:
            print(f"Preview generation failed: {e}")
    
    def _create_metadata(self, context: Dict, settings: Dict) -> Dict[str, Any]:
        """Create comprehensive metadata for the processing result."""
        metadata = {
            "node_name": "{{NodeName}}",
            "version": "{{Version}}",
            "processing_timestamp": torch.tensor([0.0]),  # Placeholder
            "image_shape": context["image_shape"],
            "settings": settings,
            "success": True,
            "custom_params": context.get("custom_params", {}),
        }
        
        return metadata


# Node Registration
NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (Advanced)"
}