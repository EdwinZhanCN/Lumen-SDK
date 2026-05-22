# rest package — HTTP REST API (v1)

The REST API accepts the same envelope as the gRPC `InferRequest` stream.

## Endpoint

- `POST /v1/infer`
- Routing uses `meta.service`, not a top-level `service` field.
- `text/plain` JSON payloads are UTF-8 strings.
- Binary JSON payloads are base64 strings.
- Tensor inputs use `application/octet-stream` plus `lumen.*` metadata.

## JSON Envelope

```json
{
  "correlation_id": "req-001",
  "task": "semantic_image_embed",
  "payload_mime": "image/jpeg",
  "payload": "<base64 image bytes>",
  "meta": {
    "service": "clip"
  },
  "seq": 0,
  "total": 1
}
```

For text:

```json
{
  "correlation_id": "text-001",
  "task": "semantic_text_embed",
  "payload_mime": "text/plain",
  "payload": "a photo of a cat",
  "meta": {
    "service": "clip"
  }
}
```

## Tensor Skip Example

```json
{
  "correlation_id": "img-002",
  "task": "semantic_image_embed",
  "payload_mime": "application/octet-stream",
  "payload": "<base64 tensor bytes>",
  "meta": {
    "service": "clip",
    "lumen.input.kind": "tensor",
    "lumen.preprocess.skip": "true",
    "lumen.preprocess.id": "clip_image_preprocess_v1",
    "lumen.tensor.dtype": "fp32",
    "lumen.tensor.shape": "[1,3,224,224]",
    "lumen.tensor.layout": "NCHW",
    "lumen.tensor.format": "contiguous",
    "lumen.tensor.byte_order": "little"
  }
}
```

## Supported Task Names

- `semantic_text_embed`
- `semantic_image_embed`
- `bioclip_classify`
- `ocr`
- `face_recognition`

Service examples are `clip`, `siglip`, `ppocr`, and `insightface`.

Batching is a server-side merge behavior. The SDK validates and forwards the tensor contract required for batching, but REST does not aggregate requests itself.
