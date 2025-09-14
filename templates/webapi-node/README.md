# {{NodeName}} (Web/API Node)

Generated via comfy-chair-go Web/API template.

Provides:
- Floating panel UI (vanilla JS)
- HTTP endpoints (`/{{NodeNameLower}}/generate`, `/{{NodeNameLower}}/health`, `/{{NodeNameLower}}/version`)
- Batch example endpoints

Edit `nodes/{{NodeNameLower}}.py` to customize generation logic.

## Endpoints Quick Test
```bash
curl -s -X POST http://127.0.0.1:8188/{{NodeNameLower}}/generate -H 'Content-Type: application/json' -d '{"topic":"demo"}' | jq .
```
