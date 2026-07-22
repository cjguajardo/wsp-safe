# Operación

## Modos

### Vinculación

```env
WSP_MODE=link
```

También puede ejecutarse de forma nativa:

```bash
go run ./cmd/wsp-safe --link
```

### Moderación

```env
WSP_MODE=run
```

En este modo se procesan todos los mensajes nuevos de la cuenta vinculada.

## Registros de diagnóstico

Activa temporalmente:

```env
WSP_LOG_DECISIONS=true
CLASSIFIER_LOG_REQUESTS=true
```

El cliente registra recepción, identificador del remitente, tipo, decisión y
puntuaciones. El clasificador registra ruta, estado, duración y resultado.

No se registran texto, descripciones, contenido base64 ni bytes multimedia.
Desactiva estas variables después del diagnóstico.

## Archivo cifrado de eliminados

Genera una clave:

```bash
openssl rand -base64 32
```

Configura:

```env
WSP_ARCHIVE_DELETED=true
WSP_ARCHIVE_KEY=clave_generada
WSP_ARCHIVE_DIR=/data/deleted
```

Cada registro se cifra con AES-256-GCM y se escribe con permisos restrictivos.
La clave no se almacena junto a los archivos.

## Revisión manual

Para descifrar un registro dentro del contenedor:

```bash
docker compose exec wsp-safe wsp-safe-review \
  -file /data/deleted/REGISTRO.wsp-safe \
  -output /tmp/revision-privada
docker compose cp wsp-safe:/tmp/revision-privada ./revision-privada
docker compose exec wsp-safe rm -rf /tmp/revision-privada
```

La herramienta crea `mensaje.json` y, cuando corresponde, un archivo
multimedia. El directorio resultante contiene información sensible sin cifrar y
debe eliminarse después de la revisión.

Si WhatsApp no permite descargar el archivo o supera el límite configurado, el
registro puede contener únicamente metadatos.

## Verificación

Pruebas Go:

```bash
go test ./...
```

Pruebas del clasificador:

```bash
PYTHONPATH=classifier python -m unittest discover \
  -s classifier/tests -p 'test_*.py'
```

## Diagnóstico del gateway

El punto de conexión `/v1/classify-site` permite verificar una URL de manera
controlada. Consulta el contrato en [API del clasificador](api-clasificador.md).
