---
title: Node Development
nav_order: 5
---

# Node Development

## Scaffolding with create-node

`create-node` prompts for a template, node name, author, and publisher ID (author and publisher default from `CUSTOM_NODES_AUTHOR` and `CUSTOM_NODES_PUBID` in `.env`), then copies a template tree into the active environment's `custom_nodes/` with all placeholders substituted, including in file names. A starter example workflow JSON is generated alongside the node, and the node is recorded in the environment's registry entry.

### The five templates

| Template | Contents |
|----------|----------|
| **basic** | One node with the common widget input types, plus pyproject, README, license, and a JS extension stub. The best starting point. |
| **advanced** | Adds a utils module, CSS, and a richer JS extension for custom UI work. |
| **api** | Structured for nodes that call external APIs. |
| **model** | Structured for nodes that load and manage model files. |
| **webapi** | A floating web panel served by ComfyUI plus HTTP endpoints (`api.py`), with settings JS and a build guide in `docs/`. |

### The V3 schema

All templates target the ComfyUI V3 node API from `comfy_api.latest`, which requires ComfyUI 0.10.0 or newer:

```python
from comfy_api.latest import IO

class MyNode(IO.ComfyNode):
    @classmethod
    def define_schema(cls):
        return IO.Schema(
            node_id="MyNode",
            category="custom",
            inputs=[IO.String.Input("text", default="hi")],
            outputs=[IO.String.Output("result")],
        )

    @classmethod
    def execute(cls, text) -> IO.NodeOutput:
        return IO.NodeOutput(text.upper())
```

Registration happens in the package `__init__.py` through a `ComfyExtension` subclass returned from `comfy_entrypoint()`. There are no `NODE_CLASS_MAPPINGS`, no `INPUT_TYPES`, and no instance methods. Semantic validation goes in `validate_inputs`, and cache control in `fingerprint_inputs` (the V3 equivalents of `VALIDATE_INPUTS` and `IS_CHANGED`). The scaffolded basic node demonstrates each of these.

Templates are embedded in the binary, so a node scaffolded by an old binary reflects that binary's templates. If your generated nodes look wrong, check that the `comfy-chair` on your PATH is current.

## Live reload

`reload` starts ComfyUI (if not already running), tails its log into your terminal, and watches the opted-in node directories for changes.

- Watching is opt-in: run `watch-nodes` to select which directories under `custom_nodes/` trigger restarts. The selection is saved per environment, and nothing is watched by default.
- A change restarts ComfyUI immediately when things have been quiet. Changes that land inside the debounce window (default 5 seconds, `COMFY_RELOAD_DEBOUNCE`) are coalesced into one deferred restart rather than dropped.
- Only files matching `COMFY_RELOAD_EXTS` (default `.py,.js,.css`) trigger restarts.
- A watched directory may be a symlink to somewhere outside `custom_nodes/`; events from the real location are handled correctly.
- New directories created while watching are added to the watch set automatically.
- `__pycache__`, `.git`, editor swap files, build output, and anything matched by a `.gitignore` are ignored.

Stop watching with Ctrl+C. The graceful path sends SIGTERM first and only force-kills when the process does not exit within the wait window.

## Updating nodes

`update-nodes` runs `git pull` in each selected node that is a git checkout, then installs its `requirements.txt` into the environment's venv using uv. When uv is unavailable the fallback is the venv's own interpreter (`python -m pip`), never the system Python. Honors `--dry-run`.

## Packing and sharing workflows

- `pack-node` zips a node directory for distribution. Errors during packing remove the partial zip instead of leaving a corrupt archive.
- `node-workflows` copies a node's bundled workflows (from its `example_workflows/` or `workflows/` directory) into the environment's main workflows folder under a `NodeName__` prefix, tracks what it added in `.node_workflows.json`, and removes exactly those files on request.
