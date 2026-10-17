# Test data — invoice images

Synthetic Romanian supplier invoices for testing the vision extraction. No real
business data; safe to commit and share.

## Files

Two document directions are covered:
- **Supplier invoices** (input, goods IN): Metro + Selgros.
- **Client bon fiscal** (output, goods OUT): a fiscal receipt.

- `invoice1_metro.svg` / `invoice1_metro.jpg` — clean Metro invoice, 8 line items
- `invoice2_selgros.svg` / `invoice2_selgros.jpg` — clean Selgros invoice, different layout + items
- `bon_fiscal1.svg` / `bon_fiscal1.jpg` — clean client fiscal receipt, 6 line items
- `expected/*.json` — ground-truth line items for each document
- `variants/` — photo-like degradations:
  - `inv1_rotate.jpg` — ~7° rotation
  - `inv1_perspective.jpg` — perspective skew
  - `inv1_lighting.jpg` — uneven diagonal lighting
  - `inv1_noise.jpg` — blur + sensor noise + low JPEG quality
  - `inv1_combined.jpg` / `inv2_combined.jpg` / `bon1_combined.jpg` — worst case: downscale + lighting + noise + skew + rotation

## Regenerate the images

Requires ImageMagick 7 (`magick`):

```bash
./generate.sh
```

Edit the `.svg` sources to change the invoices, then re-run.

## Score extraction against ground truth

Runs every image through the Fireworks vision model (the same request `vision.go`
builds) and compares to `expected/*.json`:

```bash
# uses ../fireworks.txt for the key, or set VISION_API_KEY
python3 check_extract.py
```

Override the model/provider with `VISION_MODEL`, `VISION_BASE_URL`, `VISION_API_KEY`.

The harness picks the invoice vs receipt prompt per case (`is_input`), mirroring the app.

Last run: **76/76 rows exact (100%)** with `qwen3p8-max`, across invoices + bon fiscal and all degraded variants.
