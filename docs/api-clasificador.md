# API del clasificador

La API es interna y no debe publicarse en Internet. Si
`WSP_CLASSIFIER_TOKEN` está configurado, las solicitudes deben enviar:

```http
Authorization: Bearer TOKEN
```

## Estado

```http
GET /healthz
```

Respuesta:

```json
{"status":"ok"}
```

## Clasificar un mensaje

```http
POST /v1/classify
Content-Type: application/json
```

Solicitud:

```json
{
  "message_id": "3EB0...",
  "sender_id": "56911111111@s.whatsapp.net",
  "kind": "image",
  "mime_type": "image/jpeg",
  "text": "descripción opcional",
  "media_base64": "/9j/4AAQ..."
}
```

`kind` acepta `text`, `image`, `video` o `sticker`. `media_base64` es obligatorio
para imágenes, videos y stickers.

Respuesta:

```json
{
  "sexual_score": 0.92,
  "sexual_minors_score": 0.0,
  "uncertain": false,
  "sites": [
    {
      "domain": "example.com",
      "categories": ["Pornography"],
      "nsfw": true,
      "score": 1.0,
      "provider": "cloudflare",
      "cached": false
    }
  ]
}
```

Los puntajes deben estar entre 0 y 1. `sites` está vacío cuando el texto no
contiene enlaces.

## Clasificar un sitio

```http
POST /v1/classify-site
Content-Type: application/json
```

Solicitud:

```json
{"url":"https://example.com/ruta"}
```

Respuesta:

```json
{
  "domain": "example.com",
  "categories": ["visual_nsfw"],
  "nsfw": true,
  "score": 0.88,
  "provider": "playwright",
  "cached": false
}
```

Un fallo de todos los proveedores devuelve HTTP 503. Una solicitud inválida
devuelve HTTP 422.
