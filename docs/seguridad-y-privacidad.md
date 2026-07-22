# Seguridad y privacidad

## Secretos

Protege los siguientes valores:

- `WSP_SESSION_DB`: contiene credenciales del dispositivo vinculado.
- `WSP_CLASSIFIER_TOKEN`: protege el clasificador interno.
- `WSP_ARCHIVE_KEY`: permite descifrar mensajes archivados.
- `GOOGLE_SAFESEARCH_API_KEY`.
- `IPQS_API_KEY`.
- `CLOUDFLARE_API_TOKEN`.

No incluyas `.env`, bases SQLite, archivos cifrados ni resultados descifrados en
Git.

## Redes

El clasificador utiliza una red privada para recibir solicitudes del cliente y
una red con salida para contactar servicios externos. No debe tener puertos ni
dominios públicos.

El contenedor utiliza:

- Sistema de archivos de solo lectura.
- Usuario sin privilegios.
- Eliminación de capacidades Linux.
- `no-new-privileges`.
- Directorio temporal en memoria.

Playwright bloquea destinos locales y privados, pero debe permanecer aislado.

## Datos enviados a terceros

- Google SafeSearch recibe imágenes o fotogramas cuando se configura su clave.
- IPQualityScore recibe las URLs que clasifica.
- Cloudflare Radar recibe las URLs y conserva informes según su política.

NudeNet, OpenNSFW2 y el análisis de capturas de Playwright se ejecutan dentro del
clasificador. Playwright sí descarga recursos desde el sitio visitado.

## Registros

Los registros de diagnóstico excluyen contenido textual y multimedia. Sin
embargo, pueden incluir identificadores de mensajes, chats y remitentes. Trátalos
como datos privados.

## Archivo de revisión

El archivo de eliminados está desactivado de forma predeterminada. Cuando se
activa:

- Usa AES-256-GCM.
- Guarda cada registro con permisos `0600`.
- Guarda el directorio con permisos `0700`.
- Requiere una clave separada de 32 bytes.

La pérdida de la clave impide recuperar el contenido. La exposición conjunta de
la clave y el volumen permite descifrarlo.

## Limitaciones

- El mensaje puede aparecer brevemente antes de su eliminación.
- No se revisan mensajes históricos.
- Audios, documentos, ubicaciones, contactos y reacciones no se clasifican.
- Los modelos pueden producir falsos positivos o falsos negativos.
- La caché NSFW por hostname puede bloquear sitios completos de contenido mixto.
- WhatsApp puede cambiar el protocolo utilizado por `DeleteForMe`.
- El uso de `whatsmeow` puede implicar riesgos operativos para la cuenta.

Prueba primero con una conversación controlada y revisa periódicamente la lista
de dispositivos vinculados en WhatsApp.
