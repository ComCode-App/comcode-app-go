"""Keeps docs/openapi.yaml in step with the live Go routes.

  python3 scripts/openapi_routes.py          check: exit 1 if a route is undocumented or a spec op has no route
  python3 scripts/openapi_routes.py --write  add minimal operations for undocumented routes (then refine by hand)

Rules (read from the route registration, not guessed):
  mux.HandleFunc(...)              -> no bearer: /v1/internal/* uses the adapter key, others are public
  mux.Handle(..., Chain(...))      -> bearer; an rbac.Perm* in the registration is the required permission
Existing operations are never touched.
"""
import glob, re, sys

import os
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WRITE = "--write" in sys.argv
spec_path = f"{ROOT}/docs/openapi.yaml"
spec = open(spec_path).read()

# rbac constant -> slug
perms = dict(re.findall(r'(Perm\w+)\s*=\s*"([^"]+)"', open(f"{ROOT}/internal/platform/rbac/rbac.go").read()))

routes = {}
for f in sorted(glob.glob(f"{ROOT}/internal/**/*.go", recursive=True)):
    if f.endswith("_test.go"):
        continue
    lines = open(f).read().splitlines()
    for i, line in enumerate(lines):
        m = re.search(r'mux\.(Handle|HandleFunc)\("(GET|POST|PUT|PATCH|DELETE) (/[^"]*)"', line)
        if not m:
            continue
        kind, method, path = m.groups()
        if path.startswith("/docs"):  # the docs UI serving this file
            continue
        path = re.sub(r"\{(\w+)\.\.\.\}", r"{\1}", path)
        window = " ".join(lines[i:i + 4])
        perm = re.search(r"rbac\.(Perm\w+)", window)
        routes[(method, path)] = {
            "auth": "bearer" if kind == "Handle" else ("adapter" if path.startswith("/v1/internal/") else "public"),
            "perm": perms.get(perm.group(1)) if perm else None,
            "file": f.replace(ROOT + "/", ""),
        }

# existing spec ops
existing, cur = {}, None
for line in spec.splitlines():
    m = re.match(r"^  (/[^:]+):\s*$", line)
    if m:
        cur = m.group(1); existing.setdefault(cur, set()); continue
    m = re.match(r"^    (get|post|put|patch|delete):", line)
    if m and cur:
        existing[cur].add(m.group(1).upper())
    if re.match(r"^[a-z]", line):
        cur = None
norm = lambda p: re.sub(r"\{[^}]+\}", "{}", p)
spec_norm = {(m, norm(p)) for p, ms in existing.items() for m in ms}
spec_paths = {norm(p): p for p in existing}

TAGS = {
    "billing": "Billing", "internal": "Internal", "notifications": "Notifications", "storage": "Storage",
    "products": "Products", "public": "Public", "activity": "Activity", "exports": "Exports",
    "imports": "Imports", "log-rules": "Logs", "logs": "Logs", "payment": "Payment", "permissions": "Roles",
    "roles": "Roles", "settings": "Settings", "users": "Users",
}

def tag_for(path):
    parts = [p for p in path.split("/") if p and not p.startswith("{")][1:]
    if parts and parts[0] == "platform":
        parts = parts[1:]
    for p in parts:
        if p in TAGS:
            return TAGS[p]
    return parts[0].capitalize() if parts else "Misc"

def op_id(method, path):
    words = [w for w in re.split(r"[/{}\-_.]+", path) if w and w != "v1"]
    return method.lower() + "".join(w[:1].upper() + w[1:] for w in words)

def block(method, path, info):
    params = re.findall(r"\{(\w+)\}", path)
    out = [f"    {method.lower()}:"]
    out.append(f"      tags: [{tag_for(path)}]")
    out.append(f"      operationId: {op_id(method, path)}")
    summary = f"{method} {path}"
    out.append(f"      summary: \"{summary}\"")
    desc = f"Registered in `{info['file']}`."
    if info["perm"]:
        desc += f" Requires `{info['perm']}`."
    if info["auth"] == "adapter":
        desc += " NextAuth adapter only: `X-Auth-Adapter-Key` header."
    if info["auth"] == "public":
        desc += " No authentication."
    out.append(f"      description: \"{desc}\"")
    if info["auth"] == "bearer":
        out.append("      security: [{ bearerAuth: [] }]")
    elif info["auth"] == "adapter":
        out.append("      security: [{ adapterKey: [] }]")
    else:
        out.append("      security: []")
    if params:
        out.append("      parameters:")
        for p in params:
            out.append(f"        - {{ name: {p}, in: path, required: true, schema: {{ type: string }} }}")
    if method in ("POST", "PUT", "PATCH"):
        out.append("      requestBody:")
        out.append("        required: false")
        out.append("        content:")
        out.append("          application/json:")
        out.append("            schema: { type: object, additionalProperties: true }")
    out.append("      responses:")
    out.append("        \"200\":")
    out.append("          description: Success envelope (see API_CONVENTIONS.md); some endpoints stream files or redirect")
    out.append("          content:")
    out.append("            application/json:")
    out.append("              schema: { $ref: \"#/components/schemas/EnvelopeGeneric\" }")
    if info["auth"] != "public":
        out.append("        \"401\": { $ref: \"#/components/responses/Unauthenticated\" }")
    if info["auth"] == "bearer" and info["perm"]:
        out.append("        \"403\": { $ref: \"#/components/responses/Forbidden\" }")
    return out

missing = sorted(k for k in routes if (k[0], norm(k[1])) not in spec_norm)
live_norm = {(m, norm(p)) for m, p in routes}
stale = sorted(f"{m} {p}" for p, ms in existing.items() for m in ms if (m, norm(p)) not in live_norm)
if not WRITE:
    for m, p in missing:
        print(f"undocumented route: {m} {p}  ({routes[(m, p)]['file']})")
    for s in stale:
        print(f"spec operation without a route: {s}")
    if missing or stale:
        print("\nRun: python3 scripts/openapi_routes.py --write  (and remove stale operations by hand)")
        sys.exit(1)
    print(f"openapi matches {len(routes)} live routes")
    sys.exit(0)
new_paths, added_to_existing = {}, {}
for method, path in missing:
    sp = spec_paths.get(norm(path))
    if sp:
        added_to_existing.setdefault(sp, []).append((method, path))
    else:
        new_paths.setdefault(path, []).append((method, path))

lines = spec.splitlines()
# 1) add methods under existing path keys (insert right after the path line)
for sp, ops in added_to_existing.items():
    idx = lines.index(f"  {sp}:")
    insert = []
    for method, path in ops:
        insert += block(method, sp, routes[(method, path)])
    lines[idx + 1:idx + 1] = insert
# 2) append new path keys at the end of the paths section
end = next(i for i, l in enumerate(lines) if l.startswith("components:"))
insert = []
for path in sorted(new_paths):
    insert.append(f"  {path}:")
    for method, p in sorted(new_paths[path]):
        insert += block(method, path, routes[(method, p)])
    insert.append("")
lines[end:end] = insert
text = "\n".join(lines) + "\n"
# 3) shared components
if "    EnvelopeGeneric:" not in text:
    text = text.replace("  schemas:\n", "  schemas:\n    EnvelopeGeneric:\n      type: object\n      required: [success, meta]\n      properties:\n        success: { type: boolean }\n        data: {}\n        meta: { $ref: \"#/components/schemas/ResponseMeta\" }\n\n", 1)
if "    adapterKey:" not in text:
    text = text.replace("  securitySchemes:\n", "  securitySchemes:\n    adapterKey:\n      type: apiKey\n      in: header\n      name: X-Auth-Adapter-Key\n      description: Shared secret of the NextAuth adapter (BFF only)\n", 1)
open(spec_path, "w").write(text)
print(f"live routes: {len(routes)}, added ops: {len(missing)} ({len(new_paths)} new paths, {sum(len(v) for v in added_to_existing.values())} on existing paths)")
