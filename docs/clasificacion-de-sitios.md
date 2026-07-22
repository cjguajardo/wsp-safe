# Clasificación de sitios

Esta sección describe el gateway especial utilizado para revisar enlaces
incluidos en mensajes y descripciones de multimedia.

## Objetivo

El gateway evita depender de un único proveedor y reduce consultas repetidas a
servicios externos. La clasificación de una URL contribuye a la misma puntuación
sexual utilizada para texto y multimedia.

## Flujo

```text
URL encontrada
  -> normalización HTTP o HTTPS
  -> búsqueda del hostname en SQLite
  -> si está confirmado como NSFW, devolver la caché
  -> si no está, elegir el siguiente proveedor
  -> si el proveedor falla, intentar los restantes
  -> si el resultado es NSFW, guardar hostname y categorías
  -> devolver puntuación, categorías, proveedor y estado de caché
```

## Rotación

Los proveedores habilitados participan en una rotación de tipo round-robin:

1. IPQualityScore, cuando existe `IPQS_API_KEY`.
2. Cloudflare Radar, cuando existen cuenta y token.
3. Playwright, cuando `SITE_PLAYWRIGHT_ENABLED=true`.

Si el proveedor seleccionado falla, el gateway prueba los restantes en esa
misma solicitud. Si todos fallan, el resultado se marca como dudoso y la política
predeterminada elimina el mensaje.

## Base categorizada

La base `site-categories.db` guarda:

- Hostname normalizado.
- Categorías informadas.
- Puntuación.
- Proveedor que confirmó el resultado.
- Fecha de verificación.

Solo se guardan resultados NSFW. Los resultados seguros no se persisten, por lo
que una consulta futura al mismo hostname utiliza el siguiente proveedor de la
rotación.

La caché opera por hostname, no por ruta. Si una URL de una plataforma mixta se
clasifica como NSFW, otras URLs del mismo hostname se considerarán NSFW. Este
comportamiento es conservador y puede producir falsos positivos.

## IPQualityScore

El proveedor envía la URL completa mediante `POST` y coloca la clave en el
encabezado `IPQS-KEY`. El gateway utiliza el indicador `adult` y conserva la
categoría informada por el servicio.

## Cloudflare Radar

El gateway crea un escaneo con visibilidad `Unlisted` y consulta periódicamente
el resultado. Considera NSFW las categorías adultas configuradas, como
pornografía, desnudez y temas adultos.

`Unlisted` evita que el informe aparezca en listados y búsquedas públicas, pero
no elimina la retención aplicada por Cloudflare.

## Playwright

Chromium abre la URL, espera el documento inicial, toma una captura visible de
1280 × 720 píxeles y la analiza con:

- NudeNet.
- OpenNSFW2.
- Google SafeSearch, cuando está habilitado.

La captura se mantiene en memoria y no se guarda en la base de datos.

## Protección SSRF

Antes de navegar y antes de cargar cada recurso, Playwright valida que:

- El esquema sea HTTP o HTTPS.
- La URL no incluya credenciales.
- El hostname no sea local o interno.
- Todas las direcciones resueltas sean globales.

También bloquea redes privadas, direcciones reservadas y destinos no globales.
Esta defensa reduce el riesgo, pero no reemplaza el aislamiento de red. El
clasificador nunca debe exponerse como navegador público.

## Privacidad

IPQualityScore y Cloudflare reciben la URL consultada. Una URL puede contener
identificadores, parámetros o tokens privados. Evalúa esta exposición antes de
activar proveedores externos.
