# Despliegue con Dokploy

## Tipo de proyecto

Crea un proyecto **Docker Compose** que apunte a este repositorio. No utilices
Docker Stack porque el despliegue necesita construir las imágenes definidas por
los Dockerfiles.

No agregues un dominio público al servicio `classifier`.

## Primer despliegue: vinculación

Configura:

```env
WSP_MODE=link
WSP_PAIR_PHONE=56912345678
```

El número debe incluir el código de país. Los espacios y el signo `+` son
opcionales.

1. Despliega los servicios.
2. Revisa los registros de `wsp-safe`.
3. Copia el código de vinculación de ocho caracteres.
4. En el teléfono, abre **WhatsApp → Dispositivos vinculados → Vincular un
   dispositivo → Vincular con número de teléfono**.
5. Introduce el código.

El modo `link` permanece activo después de vincular la cuenta para evitar un
bucle de reinicios en Dokploy.

Si `WSP_PAIR_PHONE` está vacío, el servicio muestra un código QR. Para Dokploy
se recomienda el código de teléfono porque el visor de registros puede alterar
el tamaño o los espacios del QR.

## Segundo despliegue: moderación

Cambia la configuración:

```env
WSP_MODE=run
WSP_PAIR_PHONE=
```

Vuelve a desplegar. El volumen `wsp-safe-data` conserva la sesión vinculada.

## Proveedores externos opcionales

Para activar IPQualityScore:

```env
IPQS_API_KEY=valor_secreto
```

Para activar Cloudflare Radar URL Scanner:

```env
CLOUDFLARE_ACCOUNT_ID=identificador_de_cuenta
CLOUDFLARE_API_TOKEN=token_secreto
```

El token de Cloudflare necesita permisos de lectura y escritura para URL
Scanner. Google SafeSearch requiere una clave asociada a un proyecto con Cloud
Vision API habilitada.

## Recursos recomendados

| Servicio | CPU | RAM |
| --- | ---: | ---: |
| `wsp-safe` | 0.5 | 256 MiB |
| `classifier` | 1.0 | 3 GiB |

En un MacBook Pro 2012 de dos núcleos, mantén `WSP_WORKERS=1`. En un modelo de
cuatro núcleos se puede evaluar `WSP_WORKERS=2` y `CLASSIFIER_CPUS=2.0` después
de medir temperatura, latencia y consumo.

La primera construcción es pesada porque instala ONNX Runtime, OpenCV, NudeNet,
OpenNSFW2, TensorFlow, Chromium, Playwright y FFmpeg. Los despliegues posteriores
pueden reutilizar las capas de Docker.

## Volúmenes

- `wsp-safe-data`: credenciales del dispositivo y archivo cifrado opcional.
- `classifier-data`: base de dominios NSFW.

Respalda ambos volúmenes únicamente en destinos cifrados.
