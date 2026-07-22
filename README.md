# wsp-safe

Servicio personal que revisa los mensajes nuevos recibidos por una cuenta de
WhatsApp y elimina **solo para mí** aquellos que contienen contenido sexual,
adulto o dudoso.

El servicio escucha todos los chats de la cuenta vinculada. No requiere
seleccionar grupos, contactos ni conversaciones específicas.

## Funcionalidades

- Revisa mensajes nuevos de conversaciones individuales, grupos y otros chats.
- Procesa texto, enlaces, imágenes, videos y stickers.
- Combina NudeNet y OpenNSFW2 localmente.
- Puede agregar Google SafeSearch cuando se configura una clave.
- Clasifica sitios mediante IPQualityScore, Cloudflare Radar y Playwright.
- Conserva en SQLite los dominios confirmados como NSFW.
- Elimina el mensaje únicamente en el dispositivo vinculado del usuario.
- Puede archivar de forma cifrada el contenido eliminado para revisión manual.
- Registra decisiones y remitentes sin registrar texto ni archivos multimedia.

## Inicio rápido con Docker Compose

1. Copia `.env.example` como `.env` y configura los secretos necesarios.
2. Vincula la cuenta:

   ```env
   WSP_MODE=link
   WSP_PAIR_PHONE=56912345678
   ```

3. Inicia los servicios y completa la vinculación desde WhatsApp.
4. Activa la moderación:

   ```env
   WSP_MODE=run
   WSP_PAIR_PHONE=
   ```

5. Vuelve a iniciar el servicio para aplicar el cambio.

El clasificador no debe publicarse en Internet. Solo debe estar disponible en
la red privada definida por `docker-compose.yml`.

## Documentación

La documentación detallada se encuentra en [`docs/`](docs/README.md):

- [Arquitectura y flujo de moderación](docs/arquitectura.md)
- [Configuración y variables de entorno](docs/configuracion.md)
- [Despliegue con Dokploy](docs/despliegue-dokploy.md)
- [Clasificación de sitios y gateway rotatorio](docs/clasificacion-de-sitios.md)
- [Operación, registros y revisión de eliminados](docs/operacion.md)
- [Seguridad, privacidad y limitaciones](docs/seguridad-y-privacidad.md)
- [Contrato HTTP del clasificador](docs/api-clasificador.md)

## Requisitos principales

- Docker Compose para el despliegue recomendado.
- Go 1.25 y CGO solamente para ejecución nativa del cliente.
- Aproximadamente 3 GiB de RAM para el clasificador.
- Una cuenta de WhatsApp que pueda vincular un dispositivo adicional.

## Advertencia

WhatsApp entrega el mensaje antes de que un dispositivo vinculado pueda
clasificarlo y eliminarlo. Durante esa ventana, una notificación o vista previa
podría mostrar el contenido. Se recomienda silenciar los chats y desactivar las
vistas previas sensibles.

`whatsmeow` utiliza el protocolo de WhatsApp Web y no es una integración oficial
de Meta. Los cambios del protocolo pueden requerir ajustes futuros.
