# Performance Benchmark Dataset Setup

This document records the reproducible dataset setup for Lumen SDK ↔ Lumen Hub performance benchmarks.

The benchmark plan covers two tasks:

- `semantic_image_embed` using SigLIP base (`siglip2-base-patch16-224`)
- `bioclip_classify` using BioCLIP (`bioclip-2`)

The datasets are intentionally stored outside the repository. Do not commit downloaded images or generated benchmark samples.

## Dataset Choices

| Task | Dataset | Sample Size | Purpose |
|---|---:|---:|---|
| `semantic_image_embed` | MS COCO 2017 validation images | 500 | General semantic image embedding workload |
| `bioclip_classify` | CUB-200-2011 Birds | 500 | Biological image classification workload |
| mixed task benchmark | COCO 250 + CUB 250 | 500 | Concurrent mixed-task workload |

Why these datasets:

- **COCO val2017** is a standard general-purpose real-photo dataset with varied scenes and image sizes.
- **CUB-200-2011** is a compact biological image dataset suitable for BioCLIP performance testing without the size and metadata complexity of iNaturalist.
- The mixed set uses deterministic 250/250 samples to simulate concurrent Photos-style semantic + biological workloads.

## Directory Layout

Set a dataset root outside the repository:

```bash
export LUMEN_BENCH_DATA_ROOT="${HOME}/lumen-bench-data"
```

The setup commands below create this layout:

```text
${LUMEN_BENCH_DATA_ROOT}/
  downloads/
  siglip-coco-val2017/
    all/
    sample-500/
    sample-250/
  bioclip-cub200/
    all/
    sample-500/
    sample-250/
  mixed/
    sample-500/
```

## 1. Download and Sample MS COCO 2017 Validation Images

Reference page:

```text
https://cocodataset.org/#download
```

Image archive:

```text
http://images.cocodataset.org/zips/val2017.zip
```

Download and extract:

```bash
export LUMEN_BENCH_DATA_ROOT="${LUMEN_BENCH_DATA_ROOT:-${HOME}/lumen-bench-data}"

mkdir -p "${LUMEN_BENCH_DATA_ROOT}/downloads"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/all"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/sample-500"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/sample-250"

curl -L \
  "http://images.cocodataset.org/zips/val2017.zip" \
  -o "${LUMEN_BENCH_DATA_ROOT}/downloads/coco-val2017.zip"

unzip -q "${LUMEN_BENCH_DATA_ROOT}/downloads/coco-val2017.zip" \
  -d "${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/all"
```

Create deterministic 500-image and 250-image samples:

```bash
python3 - <<'PY'
import os
import random
import shutil
from pathlib import Path

root = Path(os.environ.get("LUMEN_BENCH_DATA_ROOT", Path.home() / "lumen-bench-data"))
src = root / "siglip-coco-val2017" / "all" / "val2017"
sample500 = root / "siglip-coco-val2017" / "sample-500"
sample250 = root / "siglip-coco-val2017" / "sample-250"

for directory in [sample500, sample250]:
    directory.mkdir(parents=True, exist_ok=True)

images = sorted(
    path for path in src.iterdir()
    if path.suffix.lower() in {".jpg", ".jpeg", ".png"}
)
if len(images) < 500:
    raise SystemExit(f"not enough COCO images: {len(images)}")

random.seed(20260612)
picked500 = random.sample(images, 500)
picked250 = picked500[:250]

for path in picked500:
    shutil.copy2(path, sample500 / path.name)

for path in picked250:
    shutil.copy2(path, sample250 / path.name)

print("COCO total:", len(images))
print("COCO sample-500:", len(list(sample500.iterdir())))
print("COCO sample-250:", len(list(sample250.iterdir())))
PY
```

## 2. Download and Sample CUB-200-2011 Birds

Reference page:

```text
https://data.caltech.edu/records/65de6-vp158
```

Archive:

```text
CUB_200_2011.tgz
```

Download and extract:

```bash
export LUMEN_BENCH_DATA_ROOT="${LUMEN_BENCH_DATA_ROOT:-${HOME}/lumen-bench-data}"

mkdir -p "${LUMEN_BENCH_DATA_ROOT}/downloads"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/all"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/sample-500"
mkdir -p "${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/sample-250"

curl -L \
  "https://data.caltech.edu/records/65de6-vp158/files/CUB_200_2011.tgz?download=1" \
  -o "${LUMEN_BENCH_DATA_ROOT}/downloads/CUB_200_2011.tgz"

tar -xzf "${LUMEN_BENCH_DATA_ROOT}/downloads/CUB_200_2011.tgz" \
  -C "${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/all"
```

Create deterministic 500-image and 250-image samples:

```bash
python3 - <<'PY'
import os
import random
import shutil
from pathlib import Path

root = Path(os.environ.get("LUMEN_BENCH_DATA_ROOT", Path.home() / "lumen-bench-data"))
src = root / "bioclip-cub200" / "all" / "CUB_200_2011" / "images"
sample500 = root / "bioclip-cub200" / "sample-500"
sample250 = root / "bioclip-cub200" / "sample-250"

for directory in [sample500, sample250]:
    directory.mkdir(parents=True, exist_ok=True)

images = sorted(
    path for path in src.rglob("*")
    if path.suffix.lower() in {".jpg", ".jpeg", ".png"}
)
if len(images) < 500:
    raise SystemExit(f"not enough CUB images: {len(images)}")

random.seed(20260612)
picked500 = random.sample(images, 500)
picked250 = picked500[:250]

for index, path in enumerate(picked500):
    output = sample500 / f"{index:04d}_{path.parent.name}_{path.name}"
    shutil.copy2(path, output)

for index, path in enumerate(picked250):
    output = sample250 / f"{index:04d}_{path.parent.name}_{path.name}"
    shutil.copy2(path, output)

print("CUB total:", len(images))
print("CUB sample-500:", len(list(sample500.iterdir())))
print("CUB sample-250:", len(list(sample250.iterdir())))
PY
```

## 3. Create the Mixed 500-Image Dataset

The mixed set is used when Hub runs both tasks concurrently:

- 250 COCO images for `semantic_image_embed`
- 250 CUB images for `bioclip_classify`

The file prefix encodes the intended task:

- `siglip_*.jpg` → `semantic_image_embed`
- `bioclip_*.jpg` → `bioclip_classify`

```bash
export LUMEN_BENCH_DATA_ROOT="${LUMEN_BENCH_DATA_ROOT:-${HOME}/lumen-bench-data}"

mkdir -p "${LUMEN_BENCH_DATA_ROOT}/mixed/sample-500"

python3 - <<'PY'
import os
import shutil
from pathlib import Path

root = Path(os.environ.get("LUMEN_BENCH_DATA_ROOT", Path.home() / "lumen-bench-data"))
mixed = root / "mixed" / "sample-500"
mixed.mkdir(parents=True, exist_ok=True)

coco = sorted((root / "siglip-coco-val2017" / "sample-250").iterdir())
cub = sorted((root / "bioclip-cub200" / "sample-250").iterdir())

if len(coco) != 250:
    raise SystemExit(f"COCO sample-250 expected 250, got {len(coco)}")
if len(cub) != 250:
    raise SystemExit(f"CUB sample-250 expected 250, got {len(cub)}")

for index, path in enumerate(coco):
    shutil.copy2(path, mixed / f"siglip_{index:04d}_{path.name}")

for index, path in enumerate(cub):
    shutil.copy2(path, mixed / f"bioclip_{index:04d}_{path.name}")

print("mixed sample-500:", len(list(mixed.iterdir())))
PY
```

## 4. Verify the Dataset Counts

```bash
export LUMEN_BENCH_DATA_ROOT="${LUMEN_BENCH_DATA_ROOT:-${HOME}/lumen-bench-data}"

find "${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/sample-500" -type f | wc -l
find "${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/sample-500" -type f | wc -l
find "${LUMEN_BENCH_DATA_ROOT}/mixed/sample-500" -type f | wc -l
```

Expected output:

```text
500
500
500
```

## Benchmark Inputs

Use these directories when running performance benchmarks:

| Scenario | Image Directory | Hub Services | Task Routing |
|---|---|---|---|
| SigLIP single-task | `${LUMEN_BENCH_DATA_ROOT}/siglip-coco-val2017/sample-500` | SigLIP only | all images → `semantic_image_embed` |
| BioCLIP single-task | `${LUMEN_BENCH_DATA_ROOT}/bioclip-cub200/sample-500` | BioCLIP only | all images → `bioclip_classify` |
| Mixed two-task | `${LUMEN_BENCH_DATA_ROOT}/mixed/sample-500` | SigLIP + BioCLIP | `siglip_` prefix → `semantic_image_embed`; `bioclip_` prefix → `bioclip_classify` |

## Notes

- Keep downloaded archives and sampled images out of Git.
- Verify dataset license and usage terms before downloading or sharing results.
- The deterministic sample seed is `20260612`; change it only if intentionally creating a new benchmark dataset version.
- For latency/throughput comparisons, run raw-path and tensor-path scenarios against the same sampled directories.
