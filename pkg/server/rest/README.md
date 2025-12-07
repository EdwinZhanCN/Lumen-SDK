# rest package — HTTP REST API (v1)

This package implements the HTTP REST API for Lumen. It exposes a single unified inference endpoint that supports both synchronous and streaming responses.

Key points
- Single endpoint: `POST /v1/infer`
- Routing is based on the `service` field in the request body.
  - Use a plain service name to request a synchronous response (e.g. `"face_detection"`).
  - Use the `_stream` suffix to request a streaming response (e.g. `"face_detection_stream"`).
- Request `payload` is binary data encoded as a base64 string in JSON (Go `[]byte` will be decoded automatically).
- Streaming responses are sent as Server-Sent Events (SSE) with each event body containing a JSON object. Binary results are encoded as base64 in the field `result_b64`.

Supported service names (examples)
- `embedding` / `embedding_stream`
- `classification` / `classification_stream`
- `face_detection` / `face_detection_stream`
- `face_recognition` / `face_recognition_stream`

The router maps these service strings to typed service methods (e.g. `DetectionService.GetFaceDetectionStream`) so you get type-safe request construction on the server side.

Request format (JSON)
- `service` (string) — required, determines which service and whether stream variant is used
- `task` (string) — model/task name used by nodes (may be empty depending on your deployment)
- `payload` (string, base64) — base64-encoded raw payload bytes (image, audio or text bytes)
- `correlation_id` (string, optional) — trace/correlation id
- `metadata` (map[string]string, optional) — task-specific metadata/parameters

Example JSON body (sync)
```json
{
  "service": "face_detection",
  "task": "face_detector_v1",
  "payload": "BASE64_ENCODED_IMAGE_BYTES",
  "correlation_id": "req-123",
  "metadata": {
    "detection_confidence_threshold": "0.5",
    "max_faces": "10"
  }
}
```

Example JSON body (stream)
```json
{
  "service": "face_detection_stream",
  "task": "face_detector_v1",
  "payload": "BASE64_ENCODED_LARGE_IMAGE_OR_VIDEO_CHUNKED_AS_ONE_PAYLOAD",
  "correlation_id": "stream-456",
  "metadata": {
    "detection_confidence_threshold": "0.5",
    "max_faces": "10"
  }
}
```

Notes on `payload`:
- In JSON, the `payload` field should be a base64-encoded string. The server unmarshals into `[]byte`.
- If payloads are large, server/client may automatically chunk them according to configuration (see `Chunk` config in the SDK config). Chunking is performed automatically in the client layer and the server merges the chunks.

## Alternative upload modes (recommended for large/binary payloads)

In addition to JSON+base64, the REST API supports two more efficient upload styles:

1) multipart/form-data (recommended for file uploads)
- Use a file field named `payload` and form fields for `service`, `task`, `metadata`, etc.

cURL example:
```bash
curl -X POST http://localhost:5866/v1/infer \
  -F "service=face_detection" \
  -F "task=face_model_v1" \
  -F "payload=@image.jpg" \
  -F 'metadata={"detection_confidence_threshold":"0.5"}'
```

2) application/octet-stream (raw body)
- Send raw binary in the request body and provide `service`/`task` via query params or headers.

cURL example:
```bash
curl -X POST "http://localhost:5866/v1/infer?service=face_detection&task=face_model_v1" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @image.jpg
```

Notes:
- multipart avoids base64 inflation and is browser-friendly.
- octet-stream is minimal and efficient for programmatic clients.
- The REST handler automatically detects `Content-Type` and parses appropriately. If you use JSON+base64, make sure your server and proxy body-size limits are large enough (see `BodyLimit` in Fiber config and `Connection.MaxMessageSize`).

Face detection metadata keys
- `detection_confidence_threshold` (float, e.g. `"0.5"`)
- `nms_threshold` (float)
- `face_size_min` (float)
- `face_size_max` (float)
- `max_faces` (int, `-1` for unlimited)

Sync usage examples
1) curl (synchronous, expects single final JSON result)
```bash
curl -X POST http://localhost:5866/v1/infer \
  -H "Content-Type: application/json" \
  -d '{
    "service": "face_detection",
    "task": "face_detector_v1",
    "payload": "'"$(base64 -w0 image.jpg)"'",
    "correlation_id": "req-1",
    "metadata": {"detection_confidence_threshold":"0.5"}
  }'
```

2) simple JavaScript fetch (sync)
```js
const body = {
  service: "embedding",
  task: "lumen_clip_embed",
  payload: btoa(/* binary -> base64 string */),
  correlation_id: "req-embed-1"
};

fetch("/v1/infer", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body)
}).then(r => r.json()).then(console.log);
```

Streaming usage examples (SSE)
- When the `service` has `_stream` suffix, the handler returns an SSE stream where each event body is a JSON object. The JSON contains:
  - `correlation_id` (string)
  - `is_final` (bool)
  - `seq` (uint) — sequence index
  - `total` (uint) — total number of chunks (if known)
  - `meta` (map) — any meta from server
  - `result_b64` (string) — base64 encoded bytes for that chunk/result

1) curl (streaming)
```bash
# -N keeps curl streaming / not buffering
curl -N -X POST http://localhost:5866/v1/infer \
  -H "Content-Type: application/json" \
  -d '{
    "service": "face_detection_stream",
    "task": "face_detector_v1",
    "payload": "'$(base64 -w0 large_image.jpg)'",
    "correlation_id": "stream-1",
    "metadata": {"detection_confidence_threshold":"0.5"}
  }'
```
You will see repeated JSON objects separated by blank lines. Each contains `result_b64` which you can base64-decode.

2) Browser: EventSource example (preferred for browsers)
```js
<script>
const payloadBase64 = "...."; // your base64-encoded payload
fetch("/v1/infer", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    service: "embedding_stream",
    task: "lumen_clip_embed",
    payload: payloadBase64,
    correlation_id: "stream-browser-1"
  })
}).then(res => {
  // Using EventSource is easier if the server exposes a dedicated endpoint for SSE.
  // However we use the fetch response body here as a stream for modern browsers:
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  (async function read() {
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      // split events by blank line
      const parts = buffer.split("\n\n");
      buffer = parts.pop(); // last partial
      for (const part of parts) {
        try {
          const obj = JSON.parse(part);
          const resultBytes = atob(obj.result_b64); // base64 decode
          console.log("chunk:", obj, resultBytes);
          if (obj.is_final) {
            console.log("stream finished");
          }
        } catch (e) {
          console.error("invalid chunk", e);
        }
      }
    }
  })();
});
</script>
```

Go client example (consuming stream)
```go
// Example: POST to /v1/infer and consume streaming SSE-like JSON blocks.
// Use net/http, read body as stream and parse JSON events separated by blank lines.
resp, err := http.Post("http://localhost:5866/v1/infer", "application/json", bytes.NewReader(reqBytes))
// read resp.Body incrementally and split on "\n\n", parse each JSON block.
```

Implementation notes (developer)
- The router uses the `service` string to map to a handler function. Handlers for stream services should return a `(<-chan *pb.InferResponse)` in `interface{}` form — the REST handler will detect that and stream SSE.
- The REST handler currently detects streaming responses by checking whether the router result is `(<-chan *pb.InferResponse)`.
- The client layer implements automatic chunking when payloads exceed configured thresholds (see `pkg/config.ChunkConfig`). Chunking is handled in the client; the server receives chunked messages and merges them by `Seq/Total/Offset` fields.
- Cancellation: when the HTTP client disconnects, the server receives a canceled context and the underlying client stream should be cancelled accordingly.

Troubleshooting
- If you get a `500` saying "stream handler did not return expected channel", ensure you requested a `*_stream` service and that the corresponding service implementation returns `<-chan *pb.InferResponse`.
- If large payloads fail, check the `Chunk` configuration (`pkg/config`) and `Connection.MaxMessageSize`.
- If partial results are not arriving, confirm the node / model supports streaming partials and the client connected to an appropriate model.

Where to look in code
- Router and handlers: `pkg/server/rest/router.go`, `pkg/server/rest/handlers.go`
- DTOs: `pkg/server/rest/dto.go`
- Service interfaces and implementations: `pkg/server/rest/service/interface.go` and `pkg/server/rest/service/*.go`
- Client chunking logic: `pkg/client/chunker.go`, `pkg/client/client.go`
- Config defaults and presets: `pkg/config/defaults.go`, `pkg/config/presets.go`
