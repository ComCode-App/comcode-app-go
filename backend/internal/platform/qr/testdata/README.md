# QR design golden fixtures

Each `*.json` file is one case shared by the Go and TypeScript design engines:

```json
{
  "name": "body-08-fluid",
  "matrix": ["111111101...", "..."],
  "size": 512,
  "logo_href": "",
  "style": { "foreground": "#000000", "body_shape": "fluid", "...": "..." },
  "svg": "<svg ...>...</svg>"
}
```

- `go test ./internal/platform/qr -run TestGoldenFixtures` (backend) and `pnpm test`
  (`frontend/src/features/route-qr/lib/qr-engine.test.mjs`) render `matrix` + `style` and require
  the result to equal `svg` exactly.
- The cases are defined in `fixtures_test.go` (`fixtureCases`). Regenerate after an intentional
  geometry change with `make qr-fixtures` from the repository root, which runs
  `go test ./internal/platform/qr -run TestGoldenFixtures -update` and then the TypeScript test.

Do not edit these files by hand. See `docs/QR_DESIGN.md`.
