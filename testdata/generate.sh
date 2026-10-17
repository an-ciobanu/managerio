#!/usr/bin/env bash
# Regenerates all invoice test images from the SVG sources using ImageMagick.
# Requires: magick (ImageMagick 7). Run from anywhere; it cd's to its own dir.
set -euo pipefail
cd "$(dirname "$0")"

FONT="/System/Library/Fonts/Supplemental/Arial.ttf"
DENS=120
BG='#e8e8e8'

render() { # svg -> jpg  (clean, high quality)
  magick -density "$DENS" -background white -font "$FONT" "$1" -quality 92 "$2"
}

persp() { # W H -> perspective control-point string (mild skew)
  local W=$1 H=$2
  local tlx=$((W*5/100))  tly=$((H*3/100))
  local trx=$((W*98/100)) try=$((H*1/100))
  local blx=$((W*1/100))  bly=$((H*98/100))
  local brx=$((W*95/100)) bry=$((H*100/100))
  echo "0,0 ${tlx},${tly}  ${W},0 ${trx},${try}  0,${H} ${blx},${bly}  ${W},${H} ${brx},${bry}"
}

degrade_combined() { # src out  (worst-case phone snap: downscale+light+noise+skew+rotate)
  local src=$1 out=$2 tmp=variants/_tmp.jpg
  magick "$src" -resize 820x -quality 100 "$tmp"
  local W H; read -r W H < <(magick identify -format "%w %h\n" "$tmp")
  magick "$tmp" \
    \( -size "${W}x${H}" gradient:'#ffffff'-'#8a8a8a' -rotate 25 -resize "${W}x${H}!" \) \
    -compose multiply -composite \
    -blur 0x0.6 -attenuate 0.8 +noise Gaussian \
    -virtual-pixel background -background "$BG" -distort Perspective "$(persp "$W" "$H")" \
    -background "$BG" -rotate -6 +repage \
    -quality 50 "$out"
  rm -f "$tmp"
}

echo "Rendering clean documents..."
render invoice1_metro.svg    invoice1_metro.jpg
render invoice2_selgros.svg  invoice2_selgros.jpg
render bon_fiscal1.svg       bon_fiscal1.jpg

SRC=invoice1_metro.jpg
read -r W H < <(magick identify -format "%w %h\n" "$SRC")

echo "Generating variants..."
# 1. rotation only
magick "$SRC" -background "$BG" -rotate -7 +repage -quality 85 variants/inv1_rotate.jpg
# 2. perspective skew only
magick "$SRC" -virtual-pixel background -background "$BG" \
  -distort Perspective "$(persp "$W" "$H")" +repage -quality 85 variants/inv1_perspective.jpg
# 3. uneven lighting only
magick "$SRC" \( -size "${W}x${H}" gradient:'#ffffff'-'#8f8f8f' -rotate 20 -resize "${W}x${H}!" \) \
  -compose multiply -composite -quality 85 variants/inv1_lighting.jpg
# 4. blur + sensor noise + low JPEG quality
magick "$SRC" -blur 0x0.9 -attenuate 0.7 +noise Gaussian -quality 55 variants/inv1_noise.jpg
# 5. combined worst case (Metro)
degrade_combined "$SRC" variants/inv1_combined.jpg
# 6. combined worst case (Selgros, different layout)
degrade_combined invoice2_selgros.jpg variants/inv2_combined.jpg
# 7. combined worst case (bon fiscal / receipt)
degrade_combined bon_fiscal1.jpg variants/bon1_combined.jpg

echo "Done. Images:"
ls -1 *.jpg variants/*.jpg
