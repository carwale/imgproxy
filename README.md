### imgproxy (Fork) — Master image pipeline, IPC URL format, and media tooling

This is a fork of `imgproxy` tailored for high-traffic media delivery. It introduces a master-image pipeline on S3, a simplified IPC-style URL format, built‑in watermark and artifact overlays, a refresh endpoint to (re)materialize masters, and sensible defaults for web formats, caching, and security.

If you’re familiar with upstream `imgproxy`, start with “What’s different?” below.

## What’s different vs upstream

- **Master image workflow (S3‑to‑S3):**
  - Requests fetch from a dedicated master bucket; on miss, the service builds the master from the original bucket and uploads it for future hits.
  - Buckets are configured via `IMGPROXY_ORIGINAL_BUCKET` and `IMGPROXY_MASTER_BUCKET`.
- **IPC URL format:**
  - Paths look like `WIDTHxHEIGHT/<object-key>` and accept a small set of intuitive query params (e.g. `wm`, `art`, `fmt`, `qp`, `fit`, `sh`).
  - Media paths get guardrails (e.g. max source resolution for media prefixes).
- **Watermarks and artifacts:**
  - Watermarks and multiple artifact overlays are preloaded from storage and can be toggled per request.
- **Master refresh endpoint:**
  - `POST /master/refresh` to force (re)materialization of a master from original storage.
- **Sane defaults:**
  - Web‑first formats, year‑long cache headers on successful responses, and safe processing/security defaults.

## Quick start

### Run with Docker

```bash
docker build -t imgproxy-fork .
docker run --rm -p 5000:5000 \
  -e IMGPROXY_ORIGINAL_BUCKET=m-aeplimages-dev \
  -e IMGPROXY_MASTER_BUCKET=m-aeplimagesmaster-v2-dev \
  -e AWS_REGION=ap-south-1 \
  imgproxy-fork
```

### Health and version

- Health: `imgproxy health` (CLI) or set `IMGPROXY_HEALTH_CHECK_PATH` and probe HTTP.
- Version: `imgproxy version`

## Request model (IPC URL format)

- Path: `/{width}x{height}/{key...}`
  - Example: `/300x200/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png`
- Query parameters (subset):
  - **qp**: quality (0–100). Example: `?qp=80`
  - **wm**: watermark type (1..3). Example: `?wm=2`
  - **art**: artifact type (1..9), size inferred from `{width}x{height}`. Example: `?art=5`
  - **fmt**: format by numeric id (see Formats). Example: `?fmt=13`
  - **fit**: switch resizing mode to `fit` (default is `fill-down`). Example: `?fit=1`
  - **sh**: sharpening amount. Example: `?sh=0` (off) or `?sh=1`

Notes:

- Arguments use `:` internally; you can change the separator via `IMGPROXY_ARGUMENTS_SEPARATOR`.
- Media paths are recognized by `IMGPROXY_MEDIA_PATH_PREFIXES` (default: `media/`, `dev/media/`, `staging/media/`) and automatically attach a max-source-resolution guard.

## Master image workflow

1. For a request to `/{W}x{H}/{path}`, we first try: `s3://$IMGPROXY_MASTER_BUCKET/{path}`.
2. If the master is missing or not fresh:
   - We normalize to `0x0/{path}` for master creation (no upscaling on build).
   - Download original: `s3://$IMGPROXY_ORIGINAL_BUCKET/{path}`.
   - Process to canonical master and upload to master bucket.
3. Respond using the master (and apply final request‑specific transforms).

### Force refresh

`POST /master/refresh` with JSON body:

```json
{ "path": "n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png" }
```

Behavior:

- Normalizes to `0x0/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png` and re‑creates master.
- Returns `200` with `true` on success. Returns specific 4xx/5xx when errors are known (e.g., 404 from original).
- Timeout is ~60s per refresh; queued work respects concurrency limits.

## Watermarks and artifacts

Preloaded on startup from the URIs configured in the environment.

- Watermarks
  - Types: `1` (color), `2` (BW), `3` (BW v2)
  - Enable per request: `?wm=1` (position, opacity, scaling are sensible defaults in code)
- Artifacts
  - Types: `1..9` mapped to configured S3 path patterns containing `*` placeholder for size.
  - Size is inferred from `{width}x{height}` in the URL; if a size asset is missing, artifact is skipped.
  - Enable per request: `?art=5`

Configure sources via `IMGPROXY_WATERMARK_PATHS`, `IMGPROXY_ARTIFACTS`, and `IMGPROXY_ARTIFACTS_SIZES_MAP` (see Environment).

## Formats

Preferred formats default to WebP and JPEG, with AVIF/JXL auto‑detection enabled by default:

- Automatic negotiation (Accept header): `IMGPROXY_AUTO_WEBP`, `IMGPROXY_AUTO_AVIF`, `IMGPROXY_AUTO_JXL`
- Enforce a specific one: `IMGPROXY_ENFORCE_WEBP`, `IMGPROXY_ENFORCE_AVIF`, `IMGPROXY_ENFORCE_JXL`
- Explicit format: `?fmt=<id>` (numeric id as in upstream `imagetype.Formats`)

## Caching and ETags

- Successful responses set year‑long cache headers: `Cache-Control: max-age=31536000, public`, and `Expires`/`Last-Modified`.
- Optional ETag/Last‑Modified support: `IMGPROXY_USE_ETAG`, `IMGPROXY_USE_LAST_MODIFIED`.
- `IMGPROXY_TTL` and `IMGPROXY_CACHE_CONTROL_PASSTHROUGH` can tune behavior for origin‑driven caching.

## Metrics and error reporting

- Prometheus exporter: `IMGPROXY_PROMETHEUS_BIND` (default `0.0.0.0:9421`).
- Optional providers: DataDog, New Relic, OpenTelemetry, CloudWatch (+ multiple error reporters: Bugsnag, Honeybadger, Sentry, Airbrake). See Environment.

## Security and limits

- Source validation: `IMGPROXY_ALLOWED_SOURCES`, allowlists for address classes.
- Resolution and size caps: `IMGPROXY_MAX_SRC_RESOLUTION`, `IMGPROXY_MAX_MEDIA_SRC_RESOLUTION`, `IMGPROXY_MAX_SRC_FILE_SIZE`, animation frame limits.
- SVG handling and sanitization: `IMGPROXY_SANITIZE_SVG`, `IMGPROXY_ALWAYS_RASTERIZE_SVG`, `IMGPROXY_SVG_FIX_UNSUPPORTED`.

## Environment configuration

Only commonly customized variables are listed here; see `config/config.go` for the full matrix.

- **Server**

  - `IMGPROXY_BIND` (default `:5000`)
  - `IMGPROXY_CONCURRENCY` / `IMGPROXY_WORKERS` (processing concurrency)
  - `IMGPROXY_REQUESTS_QUEUE_SIZE`, `IMGPROXY_MAX_CLIENTS`, timeouts (read/write/keep‑alive)

- **Buckets (fork feature)**

  - `IMGPROXY_ORIGINAL_BUCKET` (default `m-aeplimages`)
  - `IMGPROXY_MASTER_BUCKET` (default `m-aeplimagesmaster-v2`)

- **URL behavior**

  - `IMGPROXY_ARGUMENTS_SEPARATOR` (default `:`)
  - `IMGPROXY_MEDIA_PATH_PREFIXES` (array via env file; default: `media/`, `dev/media/`, `staging/media/`)
  - `IMGPROXY_MAX_MEDIA_SRC_RESOLUTION` (scaled by 1e6 internally; default 50 → 50MP)

- **Quality and formats**

  - `IMGPROXY_QUALITY` (default 80), `IMGPROXY_FORMAT_QUALITY_*` per format
  - `IMGPROXY_AUTO_WEBP|AVIF|JXL` (default true), `IMGPROXY_ENFORCE_*` (default false)
  - `IMGPROXY_PREFERRED_FORMATS` (default WEBP, JPEG)

- **Watermarks & artifacts (fork feature)**

  - `IMGPROXY_WATERMARK_PATHS` map (keys: `cw_watermark`, `bw_watermark`, `bw_watermark_v2`)
  - `IMGPROXY_WATERMARK_OPACITY` (global scale, default `1.0`)
  - `IMGPROXY_ARTIFACTS` map (keys `1..9` → `s3://.../template_*.png`)
  - `IMGPROXY_ARTIFACTS_SIZES_MAP` map (e.g., `"5": ["642x361", ...]`)

- **S3 / cloud storage**

  - `IMGPROXY_USE_S3` (default true), `IMGPROXY_S3_REGION`, `IMGPROXY_S3_ENDPOINT`, `IMGPROXY_S3_ENDPOINT_USE_PATH_STYLE`

- **Caching**

  - `IMGPROXY_TTL` (default 31536000), `IMGPROXY_CACHE_CONTROL_PASSTHROUGH`, `IMGPROXY_SET_CANONICAL_HEADER`
  - `IMGPROXY_USE_ETAG`, `IMGPROXY_ETAG_BUSTER`, `IMGPROXY_USE_LAST_MODIFIED`

- **Fallback image**

  - `IMGPROXY_FALLBACK_IMAGE_URL|PATH|DATA`, `IMGPROXY_FALLBACK_IMAGE_HTTP_CODE`, `IMGPROXY_FALLBACK_IMAGE_TTL`

- **Client hints & headers**

  - `IMGPROXY_ENABLE_CLIENT_HINTS`, `IMGPROXY_ENABLE_DEBUG_HEADERS`

- **Security**

  - `IMGPROXY_ALLOW_SECURITY_OPTIONS` (default true), SVG options listed above, allowed sources, etc.

- **Telemetry & reporting**
  - Prometheus: `IMGPROXY_PROMETHEUS_BIND`, `IMGPROXY_PROMETHEUS_NAMESPACE`
  - OpenTelemetry: `IMGPROXY_OPEN_TELEMETRY_*`
  - DataDog/NewRelic/CloudWatch: `IMGPROXY_DATADOG_*`, `IMGPROXY_NEW_RELIC_*`, `IMGPROXY_CLOUD_WATCH_*`
  - Error reporters: `IMGPROXY_BUGSNAG_*`, `IMGPROXY_HONEYBADGER_*`, `IMGPROXY_SENTRY_*`, `IMGPROXY_AIRBRAKE_*`

## Examples

### Basic resize

```text
/300x200/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png
```

### Change quality and output format

```text
/800x600/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png?qp=75&fmt=13
```

### Add watermark and artifact overlay

```text
/642x361/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png?wm=2&art=5
```

### Fit instead of fill‑down and sharpen

```text
/1200x800/n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png?fit=1&sh=1
```

### Force refresh master

```bash
curl -X POST http://localhost:5000/master/refresh \
  -H 'Content-Type: application/json' \
  -d '{"path":"n/cw/ec/159099/swift-exterior-right-front-three-quarter-31.png"}'
```
