#!/usr/bin/env python3
"""Run every test invoice image through the Fireworks vision model (same request
the app's vision.go builds) and score the extraction against expected/*.json.

Usage: python3 check_extract.py
Env:   VISION_API_KEY (or reads ../fireworks.txt), VISION_MODEL (default qwen3p8-max)
"""
import base64, json, os, sys, urllib.request, urllib.error

HERE = os.path.dirname(os.path.abspath(__file__))
KEY = os.environ.get("VISION_API_KEY") or open(os.path.join(HERE, "..", "fireworks.txt")).read().strip()
MODEL = os.environ.get("VISION_MODEL", "accounts/fireworks/models/qwen3p8-max")
BASE = os.environ.get("VISION_BASE_URL", "https://api.fireworks.ai/inference/v1")

# (image, expected json) — variants of invoice1 reuse invoice1 ground truth.
CASES = [
    ("invoice1_metro.jpg",            "expected/invoice1.json"),
    ("variants/inv1_rotate.jpg",      "expected/invoice1.json"),
    ("variants/inv1_perspective.jpg", "expected/invoice1.json"),
    ("variants/inv1_lighting.jpg",    "expected/invoice1.json"),
    ("variants/inv1_noise.jpg",       "expected/invoice1.json"),
    ("variants/inv1_combined.jpg",    "expected/invoice1.json"),
    ("invoice2_selgros.jpg",          "expected/invoice2.json"),
    ("variants/inv2_combined.jpg",    "expected/invoice2.json"),
]

PROMPT = (
    "You are reading a Romanian supplier invoice from Metro or Selgros.\n"
    "Extract every product line item. For each item return:\n"
    "- name: the product name exactly as printed (Romanian)\n"
    "- qty: the quantity as a number\n"
    "- unit: the unit of measure (e.g. kg, buc, l)\n"
    "- unit_price: the price per unit as a number\n"
    "Return only the structured data as JSON matching the schema."
)
SCHEMA = {
    "type": "object",
    "properties": {"items": {"type": "array", "items": {"type": "object", "properties": {
        "name": {"type": "string"}, "qty": {"type": "number"},
        "unit": {"type": "string"}, "unit_price": {"type": "number"}},
        "required": ["name", "qty", "unit", "unit_price"]}}},
    "required": ["items"],
}


def extract(img_path):
    b64 = base64.b64encode(open(os.path.join(HERE, img_path), "rb").read()).decode()
    body = {"model": MODEL, "max_tokens": 4096, "messages": [{"role": "user", "content": [
        {"type": "text", "text": PROMPT},
        {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64," + b64}}]}],
        "response_format": {"type": "json_schema", "json_schema": {"name": "invoice_items", "schema": SCHEMA}}}
    req = urllib.request.Request(BASE + "/chat/completions", data=json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + KEY, "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        resp = json.load(r)
    return json.loads(resp["choices"][0]["message"]["content"]).get("items", [])


def norm(s):
    return " ".join(str(s).strip().lower().split())


def num_eq(a, b, tol=0.01):
    try:
        return abs(float(a) - float(b)) <= tol
    except (TypeError, ValueError):
        return False


def score(got, want):
    """Positional compare. Returns (exact_rows, total, field_errors)."""
    errs, exact = [], 0
    n = max(len(got), len(want))
    for i in range(n):
        g = got[i] if i < len(got) else {}
        w = want[i] if i < len(want) else {}
        row_ok = True
        if not w:
            errs.append(f"  row {i+1}: EXTRA {g.get('name')!r}"); continue
        if not g:
            errs.append(f"  row {i+1}: MISSING {w.get('name')!r}"); continue
        if norm(g.get("name")) != norm(w.get("name")):
            errs.append(f"  row {i+1} name: got {g.get('name')!r} want {w.get('name')!r}"); row_ok = False
        if not num_eq(g.get("qty"), w.get("qty")):
            errs.append(f"  row {i+1} qty: got {g.get('qty')!r} want {w.get('qty')!r}"); row_ok = False
        if norm(g.get("unit")) != norm(w.get("unit")):
            errs.append(f"  row {i+1} unit: got {g.get('unit')!r} want {w.get('unit')!r}"); row_ok = False
        if not num_eq(g.get("unit_price"), w.get("unit_price")):
            errs.append(f"  row {i+1} price: got {g.get('unit_price')!r} want {w.get('unit_price')!r}"); row_ok = False
        if row_ok:
            exact += 1
    return exact, len(want), errs


def main():
    print(f"Model: {MODEL}\n")
    total_exact = total_rows = 0
    for img, exp in CASES:
        want = json.load(open(os.path.join(HERE, exp)))["items"]
        try:
            got = extract(img)
        except urllib.error.HTTPError as e:
            print(f"❌ {img:<34} HTTP {e.code}: {e.read().decode()[:120]}"); continue
        except Exception as e:
            print(f"❌ {img:<34} ERROR: {e}"); continue
        exact, nrows, errs = score(got, want)
        total_exact += exact; total_rows += nrows
        mark = "✅" if exact == nrows else "⚠️ "
        print(f"{mark} {img:<34} {exact}/{nrows} rows exact  ({len(got)} items returned)")
        for line in errs:
            print(line)
    print(f"\nOVERALL: {total_exact}/{total_rows} rows fully correct "
          f"({100*total_exact/total_rows:.0f}%)")


if __name__ == "__main__":
    main()
