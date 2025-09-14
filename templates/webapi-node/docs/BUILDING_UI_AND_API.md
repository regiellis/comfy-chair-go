This is a shortened copy of the architecture guide. For full details see the project specs/.coding-nodes.md.

Key points:
- __init__.py registers API routes safely.
- api.py exposes generate/health/version + batch endpoints.
- web/js contains floating panel script + optional settings script.
- nodes/<name>.py holds business logic (ComfyUI class inputs/outputs).
- requirements.txt lists runtime deps (aiohttp only by default).

Extend as needed following the patterns in the full guide.
