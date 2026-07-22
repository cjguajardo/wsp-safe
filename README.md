# wsp-safe

Filtro personal para eliminar **solo para mí** contenido sexual recibido en un
grupo específico de WhatsApp, sin abandonar el grupo.

El servicio usa [whatsmeow](https://github.com/tulir/whatsmeow) como dispositivo
vinculado. El núcleo está escrito en Go y el clasificador se consume mediante un
contrato HTTP pequeño para poder ejecutarlo localmente o cambiarlo sin tocar la
integración con WhatsApp.

## Estado del MVP

- Filtra exclusivamente el JID configurado.
- Ignora mensajes enviados por la propia cuenta y cualquier otro chat.
- Procesa texto, imágenes, videos y stickers.
- Elimina contenido sexual, dudoso o imposible de analizar según la política.
- Descarta la multimedia después de cada decisión; no la escribe en disco.
- Persiste solamente las credenciales de la sesión de WhatsApp en SQLite.
- Incluye un servicio auxiliar local de NudeNet con muestreo de videos mediante FFmpeg.
- Ejecuta una clasificación simultánea por defecto, configurable hasta ocho.

## Límite importante

WhatsApp recibe el mensaje antes de que un dispositivo vinculado pueda
clasificarlo y eliminarlo. Silencia el grupo y desactiva sus vistas previas para
evitar que una notificación muestre contenido durante esa ventana.

`DeleteForMe` se implementa como una mutación `regular_high` de app-state. Es un
detalle del protocolo de WhatsApp Web y puede necesitar ajustes cuando WhatsApp
cambie el protocolo.

## Despliegue recomendado: Dokploy

El repositorio incluye un `docker-compose.yml` preparado para **Docker Compose
estándar** en Dokploy. No utilices Docker Stack: el despliegue necesita construir
las dos imágenes desde sus Dockerfiles.

La aplicación Go se compila para la arquitectura nativa del contenedor. Esto
permite construirla tanto en servidores Intel (`amd64`) como en equipos ARM64
sin configurar manualmente `GOARCH`.

El Compose crea:

- `wsp-safe`: cliente Go de WhatsApp, con salida a Internet.
- `classifier`: NudeNet privado, sin puertos públicos ni acceso saliente.
- `wsp-safe-data`: volumen nombrado para la sesión de WhatsApp.

No agregues dominios en Dokploy. El clasificador solamente escucha dentro de la
red privada de Compose.

### Primer despliegue: vincular y descubrir el grupo

1. Crea un proyecto **Docker Compose** en Dokploy que apunte a este repositorio.
2. Configura `WSP_MODE=list-groups` en la sección **Environment**.
3. Realiza el despliegue y revisa los registros de `wsp-safe`.
4. Escanea el código QR desde **WhatsApp → Dispositivos vinculados**.
5. Copia el JID que aparece junto al nombre del grupo.
6. Cambia las variables:

```env
WSP_MODE=run
WSP_TARGET_GROUP_JID=120363000000000000@g.us
```

7. Vuelve a realizar el despliegue.

El modo `list-groups` permanece activo después de imprimir los grupos para que
Dokploy no lo reinicie en bucle. El segundo despliegue reutiliza la sesión
guardada en el volumen.

La primera compilación del clasificador puede tardar varios minutos en el equipo
Intel de 2012: descarga Python, ONNX Runtime, OpenCV, NudeNet y FFmpeg. Los
despliegues posteriores deberían reutilizar las capas de Docker mientras no
cambien las dependencias.

### Recursos predeterminados

| Servicio | CPU | RAM |
| --- | ---: | ---: |
| `wsp-safe` | 0.5 | 256 MiB |
| `classifier` | 1.0 | 1 GiB |

Para el MacBook con procesador de dos núcleos, mantén `WSP_WORKERS=1`. En el
modelo de cuatro núcleos se puede probar `WSP_WORKERS=2` y
`CLASSIFIER_CPUS=2.0` después de medir temperatura y latencia.

## Ejecución nativa opcional

Requiere Go 1.25 y CGO para SQLite.

### 1. Vincular la cuenta y descubrir el grupo

```bash
go run ./cmd/wsp-safe --list-groups
```

Escanea el código QR desde **WhatsApp → Dispositivos vinculados**. El comando imprime
cada grupo como `JID<TAB>nombre`. La sesión queda en `wsp-safe.db`; trátala como
un secreto.

### 2. Configurar

Copia `.env.example` a `.env`, completa el JID y exporta las variables. El
programa no carga `.env` automáticamente para evitar sumar magia innecesaria.

Variables principales:

| Variable | Descripción | Valor predeterminado |
| --- | --- | --- |
| `WSP_MODE` | `list-groups` para vincular o `run` para filtrar | `run` |
| `WSP_TARGET_GROUP_JID` | Grupo exacto que será filtrado | requerido |
| `WSP_CLASSIFIER_URL` | Punto de conexión del clasificador | requerido |
| `WSP_CLASSIFIER_TOKEN` | Token Bearer opcional | vacío |
| `WSP_SEXUAL_THRESHOLD` | Umbral entre 0 y 1 | `0.25` |
| `WSP_DELETE_UNCERTAIN` | Borra resultados dudosos | `true` |
| `WSP_DELETE_ON_ERROR` | Falla de forma cerrada | `true` |
| `WSP_MAX_MEDIA_BYTES` | Tamaño máximo antes de borrar por seguridad | 20 MiB |
| `WSP_WORKERS` | Clasificaciones simultáneas | `1` |
| `CLASSIFIER_VIDEO_SAMPLES` | Fotogramas distribuidos por video | `6` |
| `WSP_SESSION_DB` | Base de la sesión vinculada | `wsp-safe.db` |

### 3. Ejecutar

```bash
set -a
source .env
set +a
go run ./cmd/wsp-safe
```

## Contrato del clasificador

El servicio realiza un `POST` JSON a `WSP_CLASSIFIER_URL`:

```json
{
  "message_id": "3EB0...",
  "kind": "image",
  "mime_type": "image/jpeg",
  "text": "descripción opcional",
  "media_base64": "/9j/4AAQ..."
}
```

Respuesta:

```json
{
  "sexual_score": 0.92,
  "sexual_minors_score": 0.01,
  "uncertain": false
}
```

Los puntajes deben estar entre `0` y `1`. Un error HTTP, una respuesta inválida,
una descarga fallida o un archivo demasiado grande se consideran un fallo. Con
la configuración recomendada, el mensaje se elimina localmente.

El servicio auxiliar incluido usa NudeNet `320n`. Para imágenes y stickers toma
el mayor puntaje de las detecciones explícitas. Para videos distribuye varios
fotogramas a lo largo de la duración y conserva el puntaje máximo. El análisis
de texto es una heurística pequeña en español e inglés; NudeNet no es un modelo
de lenguaje.

NudeNet detecta desnudez y regiones corporales, pero no comprende completamente
el contexto de una escena. Por eso el umbral predeterminado es conservador y los
errores técnicos se eliminan localmente en vez de dejar pasar contenido dudoso.

## Verificación

```bash
go test ./...
```

Las pruebas cubren el aislamiento por grupo, la política conservadora, el cliente
HTTP, el mapeo de mensajes y la construcción exacta de la mutación
`DeleteForMe`.

## Privacidad y seguridad

- Nunca subas `wsp-safe.db`: contiene credenciales del dispositivo vinculado.
- Mantén `wsp-safe-data` como volumen nombrado; no utilices rutas absolutas del equipo anfitrión.
- Si activas copias de seguridad del volumen en Dokploy, cifra el destino donde se guardan.
- No registres texto, descripciones ni bytes multimedia.
- No publiques el clasificador mediante **Domains** o `ports`.
- Revisa periódicamente **Dispositivos vinculados** en WhatsApp.
- Pruébalo primero con un grupo controlado antes de configurarlo para el grupo real.
