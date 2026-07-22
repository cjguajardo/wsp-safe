# Configuración

El programa no carga `.env` automáticamente. Docker Compose sí utiliza el
archivo `.env` ubicado junto a `docker-compose.yml`.

## Cliente de WhatsApp

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `WSP_MODE` | `link` para vincular o `run` para moderar | `run` |
| `WSP_PAIR_PHONE` | Número internacional para vincular mediante código | vacío |
| `WSP_SESSION_DB` | Base SQLite de la sesión vinculada | `file:wsp-safe.db?_foreign_keys=on` |
| `WSP_WORKERS` | Clasificaciones simultáneas, entre 1 y 8 | `1` |
| `WSP_MAX_MEDIA_BYTES` | Tamaño máximo de multimedia | `20971520` |

No existe una variable para seleccionar grupos o chats. El alcance es global.

## Clasificador y política

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `WSP_CLASSIFIER_URL` | URL absoluta del clasificador | requerida |
| `WSP_CLASSIFIER_TOKEN` | Token Bearer compartido | vacío |
| `WSP_SEXUAL_THRESHOLD` | Umbral sexual entre 0 y 1 | `0.25` |
| `WSP_DELETE_UNCERTAIN` | Elimina resultados dudosos | `true` |
| `WSP_DELETE_ON_ERROR` | Elimina cuando el análisis falla | `true` |
| `CLASSIFIER_VIDEO_SAMPLES` | Fotogramas analizados por video | `6` |
| `GOOGLE_SAFESEARCH_API_KEY` | Activa Google SafeSearch | vacío |

## Clasificación de sitios

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `IPQS_API_KEY` | Activa IPQualityScore URL Scanner | vacío |
| `CLOUDFLARE_ACCOUNT_ID` | Identificador de cuenta de Cloudflare | vacío |
| `CLOUDFLARE_API_TOKEN` | Token de Cloudflare URL Scanner | vacío |
| `SITE_PLAYWRIGHT_ENABLED` | Activa Chromium mediante Playwright | `true` |
| `SITE_NSFW_THRESHOLD` | Umbral aplicado a la captura del sitio | `0.25` |
| `SITE_MAX_URLS_PER_MESSAGE` | Máximo de URLs analizadas por mensaje | `3` |
| `SITE_REQUEST_TIMEOUT` | Tiempo máximo por proveedor, en segundos | `30` |
| `SITE_DB_PATH` | Base SQLite categorizada | `/data/site-categories.db` |

IPQualityScore se activa solo con `IPQS_API_KEY`. Cloudflare requiere tanto la
cuenta como el token. Playwright está habilitado de forma predeterminada.

## Archivo cifrado

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `WSP_ARCHIVE_DELETED` | Conserva mensajes eliminados cifrados | `false` |
| `WSP_ARCHIVE_KEY` | Clave AES de 32 bytes codificada en base64 | requerida al activar el archivo |
| `WSP_ARCHIVE_DIR` | Directorio de registros cifrados | `/data/deleted` |

Genera una clave con:

```bash
openssl rand -base64 32
```

La clave debe guardarse como secreto y fuera del volumen de datos.

## Registros

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `WSP_LOG_DECISIONS` | Registra recepción, remitente y decisión | `false` |
| `CLASSIFIER_LOG_REQUESTS` | Registra solicitudes y puntuaciones del clasificador | `false` |

Los registros de diagnóstico no incluyen texto, descripciones ni bytes
multimedia, pero sí pueden incluir el identificador del remitente.

## Recursos de Compose

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `WSP_APP_CPUS` | Límite de CPU del cliente | `0.5` |
| `WSP_APP_MEMORY` | Límite de memoria del cliente | `256m` |
| `CLASSIFIER_CPUS` | Límite de CPU del clasificador | `1.0` |
| `CLASSIFIER_MEMORY` | Límite de memoria del clasificador | `3g` |

Consulta `.env.example` para obtener una plantilla completa.
