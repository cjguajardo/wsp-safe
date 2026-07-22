# Arquitectura

## Componentes

### `wsp-safe`

Aplicación Go que vincula la cuenta mediante `whatsmeow`, recibe eventos de
mensajes, descarga la multimedia compatible y solicita una decisión al
clasificador.

Responsabilidades principales:

- Escuchar todos los mensajes nuevos de la cuenta vinculada.
- Ignorar mensajes enviados por la propia cuenta.
- Mapear texto, imágenes, videos y stickers al contrato interno.
- Eliminar localmente el mensaje cuando la política lo indica.
- Archivar opcionalmente el mensaje antes de eliminarlo.

### `classifier`

Servicio privado FastAPI que calcula la puntuación sexual del contenido.

Combina:

- Heurística de términos para texto.
- NudeNet para detecciones corporales.
- OpenNSFW2 para clasificación visual general.
- Google SafeSearch, solo cuando existe una clave configurada.
- Gateway rotatorio para URLs incluidas en el texto o la descripción.

### Almacenamiento

- `wsp-safe-data`: sesión del dispositivo vinculado y archivo cifrado opcional.
- `classifier-data`: base SQLite de dominios confirmados como NSFW.

## Flujo de un mensaje

```text
WhatsApp
  -> whatsmeow recibe el evento
  -> se ignora si fue enviado por la propia cuenta
  -> se mapea texto o multimedia compatible
  -> classifier evalúa texto, enlaces y multimedia
  -> la política compara el resultado con el umbral
  -> se archiva opcionalmente
  -> DeleteForMe elimina el mensaje en la cuenta vinculada
```

## Política predeterminada

La configuración recomendada falla de forma cerrada:

- Elimina cuando la puntuación sexual alcanza el umbral.
- Elimina cuando el resultado es dudoso.
- Elimina cuando el contenido no puede descargarse o clasificarse.

Esta política prioriza evitar la exposición, aunque puede producir falsos
positivos.

## Alcance global

El manejador no filtra por identificador de chat. Cualquier evento de mensaje
nuevo entregado a la cuenta vinculada puede llegar al flujo de moderación.

La moderación no revisa el historial anterior a la conexión y no modifica el
mensaje para otros participantes.

## Eliminación local

`DeleteForMe` se implementa mediante una mutación `regular_high` del estado de
aplicación de WhatsApp Web. Es un detalle de un protocolo no oficial y puede
necesitar cambios si WhatsApp modifica su funcionamiento.
