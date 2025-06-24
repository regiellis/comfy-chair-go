"""
{{NodeName}} - Model Loading ComfyUI node
{{NodeDescription}}

This node is designed for loading and managing ML models with caching and optimization.

Author: {{Author}}
"""

import torch
import os
import hashlib
from typing import Dict, Any, Tuple, Optional, Union
from pathlib import Path
import json


class {{NodeName}}:
    """
    Model loading ComfyUI node with advanced caching and optimization.
    
    Features:
    - Model caching and management
    - Memory optimization
    - Multiple model format support
    - Model validation
    - Performance monitoring
    """
    
    RETURN_TYPES = ("MODEL", "STRING", "DICT")
    RETURN_NAMES = ("model", "model_info", "metadata")
    FUNCTION = "load_model"
    CATEGORY = "{{NodeName}}/models"
    
    def __init__(self):
        self.model_cache = {}
        self.model_metadata = {}
        
    @classmethod
    def INPUT_TYPES(cls) -> Dict[str, Any]:
        return {
            "required": {
                "model_path": ("STRING", {
                    "default": "",
                    "tooltip": "Path to the model file"
                }),
                "model_type": (["auto", "safetensors", "checkpoint", "diffusers", "onnx"], {
                    "default": "auto",
                    "tooltip": "Model format type"
                }),
                "device": (["auto", "cpu", "cuda", "mps"], {
                    "default": "auto",
                    "tooltip": "Target device for model"
                }),
                "precision": (["auto", "float32", "float16", "bfloat16"], {
                    "default": "auto",
                    "tooltip": "Model precision"
                }),
            },
            "optional": {
                "cache_enabled": ("BOOLEAN", {
                    "default": True,
                    "tooltip": "Enable model caching"
                }),
                "optimize_memory": ("BOOLEAN", {
                    "default": True,
                    "tooltip": "Enable memory optimizations"
                }),
                "model_config": ("STRING", {
                    "default": "{}",
                    "multiline": True,
                    "tooltip": "Model configuration in JSON format"
                }),
                "force_reload": ("BOOLEAN", {
                    "default": False,
                    "tooltip": "Force reload even if cached"
                }),
            }
        }
    
    def load_model(self,
                   model_path: str,
                   model_type: str,
                   device: str,
                   precision: str,
                   cache_enabled: bool = True,
                   optimize_memory: bool = True,
                   model_config: str = "{}",
                   force_reload: bool = False) -> Tuple[Any, str, Dict]:
        """
        Load a model with advanced caching and optimization.
        
        Args:
            model_path: Path to model file
            model_type: Model format type
            device: Target device
            precision: Model precision
            cache_enabled: Enable caching
            optimize_memory: Enable memory optimization
            model_config: Additional model configuration
            force_reload: Force reload from disk
            
        Returns:
            Tuple of (model, model_info, metadata)
        """
        try:
            # Validate model path
            if not model_path or not os.path.exists(model_path):
                return (None, f"Model file not found: {model_path}", {"error": "file_not_found"})
            
            # Generate cache key
            cache_key = self._generate_cache_key(model_path, device, precision, model_config)
            
            # Check cache
            if cache_enabled and not force_reload and cache_key in self.model_cache:
                cached_model = self.model_cache[cache_key]
                metadata = self.model_metadata.get(cache_key, {})
                return (cached_model, "Model loaded from cache", metadata)
            
            # Parse model configuration
            try:
                config = json.loads(model_config) if model_config else {}
            except json.JSONDecodeError:
                config = {}
            
            # Determine device
            target_device = self._determine_device(device)
            
            # Determine precision
            target_dtype = self._determine_precision(precision)
            
            # Auto-detect model type if needed
            if model_type == "auto":
                model_type = self._detect_model_type(model_path)
            
            # Load model based on type
            model, model_info = self._load_model_by_type(
                model_path, model_type, target_device, target_dtype, config
            )
            
            # Apply memory optimizations
            if optimize_memory and model is not None:
                model = self._apply_memory_optimizations(model, target_device)
            
            # Generate metadata
            metadata = self._generate_metadata(model_path, model_type, target_device, target_dtype)
            
            # Cache the model
            if cache_enabled and model is not None:
                self.model_cache[cache_key] = model
                self.model_metadata[cache_key] = metadata
            
            return (model, model_info, metadata)
            
        except Exception as e:
            error_msg = f"Failed to load model: {str(e)}"
            return (None, error_msg, {"error": str(e), "success": False})
    
    def _generate_cache_key(self, model_path: str, device: str, precision: str, config: str) -> str:
        """Generate a unique cache key for the model configuration."""
        content = f"{model_path}:{device}:{precision}:{config}"
        return hashlib.md5(content.encode()).hexdigest()
    
    def _determine_device(self, device: str) -> str:
        """Determine the best device for the model."""
        if device == "auto":
            if torch.cuda.is_available():
                return "cuda"
            elif hasattr(torch.backends, 'mps') and torch.backends.mps.is_available():
                return "mps"
            else:
                return "cpu"
        return device
    
    def _determine_precision(self, precision: str) -> torch.dtype:
        """Determine the appropriate dtype for the model."""
        if precision == "auto":
            return torch.float16 if torch.cuda.is_available() else torch.float32
        elif precision == "float16":
            return torch.float16
        elif precision == "bfloat16":
            return torch.bfloat16
        else:
            return torch.float32
    
    def _detect_model_type(self, model_path: str) -> str:
        """Auto-detect model type from file extension and structure."""
        path = Path(model_path)
        
        if path.suffix == ".safetensors":
            return "safetensors"
        elif path.suffix in [".ckpt", ".pth", ".pt"]:
            return "checkpoint"
        elif path.suffix == ".onnx":
            return "onnx"
        elif path.is_dir():
            # Check for diffusers structure
            if (path / "model_index.json").exists():
                return "diffusers"
        
        return "checkpoint"  # Default fallback
    
    def _load_model_by_type(self, 
                           model_path: str, 
                           model_type: str, 
                           device: str, 
                           dtype: torch.dtype,
                           config: Dict) -> Tuple[Any, str]:
        """Load model based on its type."""
        
        if model_type == "safetensors":
            return self._load_safetensors(model_path, device, dtype)
        elif model_type == "checkpoint":
            return self._load_checkpoint(model_path, device, dtype)
        elif model_type == "diffusers":
            return self._load_diffusers(model_path, device, dtype, config)
        elif model_type == "onnx":
            return self._load_onnx(model_path, config)
        else:
            raise ValueError(f"Unsupported model type: {model_type}")
    
    def _load_safetensors(self, model_path: str, device: str, dtype: torch.dtype) -> Tuple[Any, str]:
        """Load SafeTensors model."""
        try:
            from safetensors.torch import load_file
            model_dict = load_file(model_path, device=device)
            model_info = f"SafeTensors model loaded: {len(model_dict)} tensors"
            return (model_dict, model_info)
        except ImportError:
            raise ImportError("safetensors package not installed")
    
    def _load_checkpoint(self, model_path: str, device: str, dtype: torch.dtype) -> Tuple[Any, str]:
        """Load PyTorch checkpoint."""
        model_dict = torch.load(model_path, map_location=device)
        
        # Convert to target dtype
        if isinstance(model_dict, dict):
            for key, tensor in model_dict.items():
                if isinstance(tensor, torch.Tensor):
                    model_dict[key] = tensor.to(dtype=dtype)
        
        model_info = f"Checkpoint loaded with {len(model_dict)} keys"
        return (model_dict, model_info)
    
    def _load_diffusers(self, model_path: str, device: str, dtype: torch.dtype, config: Dict) -> Tuple[Any, str]:
        """Load Diffusers model."""
        try:
            from diffusers import DiffusionPipeline
            pipeline = DiffusionPipeline.from_pretrained(
                model_path,
                torch_dtype=dtype,
                device_map=device,
                **config
            )
            model_info = f"Diffusers pipeline loaded: {type(pipeline).__name__}"
            return (pipeline, model_info)
        except ImportError:
            raise ImportError("diffusers package not installed")
    
    def _load_onnx(self, model_path: str, config: Dict) -> Tuple[Any, str]:
        """Load ONNX model."""
        try:
            import onnxruntime as ort
            session = ort.InferenceSession(model_path, **config)
            model_info = f"ONNX model loaded with {len(session.get_inputs())} inputs"
            return (session, model_info)
        except ImportError:
            raise ImportError("onnxruntime package not installed")
    
    def _apply_memory_optimizations(self, model: Any, device: str) -> Any:
        """Apply memory optimizations to the model."""
        try:
            if hasattr(model, 'eval'):
                model.eval()
            
            if device == "cuda" and hasattr(model, 'half'):
                model = model.half()
            
            if hasattr(torch.backends.cudnn, 'benchmark'):
                torch.backends.cudnn.benchmark = True
                
        except Exception as e:
            print(f"Memory optimization warning: {e}")
        
        return model
    
    def _generate_metadata(self, model_path: str, model_type: str, device: str, dtype: torch.dtype) -> Dict[str, Any]:
        """Generate comprehensive metadata for the loaded model."""
        metadata = {
            "model_path": model_path,
            "model_type": model_type,
            "device": device,
            "dtype": str(dtype),
            "file_size_mb": os.path.getsize(model_path) / (1024 * 1024),
            "load_timestamp": torch.tensor([0.0]),  # Placeholder
            "success": True,
        }
        
        # Add device-specific information
        if device == "cuda":
            metadata["cuda_memory_allocated"] = torch.cuda.memory_allocated() / (1024**3)  # GB
            metadata["cuda_memory_reserved"] = torch.cuda.memory_reserved() / (1024**3)   # GB
        
        return metadata


# Node Registration
NODE_CLASS_MAPPINGS = {
    "{{NodeName}}": {{NodeName}}
}

NODE_DISPLAY_NAME_MAPPINGS = {
    "{{NodeName}}": "{{DisplayName}} (Model Loader)"
}