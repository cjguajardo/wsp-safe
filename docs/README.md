# Documentación de wsp-safe

Esta sección reúne la documentación técnica y operativa del proyecto.

## Contenido

1. [Arquitectura](arquitectura.md): componentes, responsabilidades y recorrido
   de un mensaje.
2. [Configuración](configuracion.md): variables de entorno y valores
   predeterminados.
3. [Despliegue con Dokploy](despliegue-dokploy.md): vinculación, despliegue y
   recursos recomendados.
4. [Clasificación de sitios](clasificacion-de-sitios.md): gateway rotatorio,
   proveedores, base categorizada y protección SSRF.
5. [Operación](operacion.md): registros, diagnóstico, archivo cifrado y
   verificación.
6. [Seguridad y privacidad](seguridad-y-privacidad.md): secretos, exposición de
   datos y limitaciones conocidas.
7. [API del clasificador](api-clasificador.md): contratos HTTP internos.

## Alcance funcional

wsp-safe modera todos los mensajes nuevos entregados a la cuenta vinculada. No
existe una configuración para seleccionar un grupo, contacto o chat.

Los mensajes enviados por la propia cuenta se ignoran. Actualmente se
clasifican texto, enlaces, imágenes, videos y stickers. Otros tipos de mensaje
pueden llegar al dispositivo vinculado, pero no se procesan todavía.
