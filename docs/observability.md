# Logging y observabilidad

## Alcance y arquitectura

TupinambaCLI es un proceso Go de corta duración. Usa `log/slog` de la biblioteca
estándar: el límite del comando inicializa el logger, añade un `correlation_id`
por invocación y establece el logger predeterminado. Los paquetes `client` y
`config` usan `slog.Default()`; no crean backends ni configuran destinos propios.
La lógica de negocio solo emite eventos estructurados y no depende de servicios
de logging externos.

El destino predeterminado es `stderr`, separado de los resultados funcionales
que el CLI escribe en `stdout`. El formato predeterminado es JSON; el formato de
texto está disponible para diagnóstico local. Un archivo local es opcional.
No se requiere conexión, SaaS ni infraestructura adicional. No se implementan
métricas ni OpenTelemetry: las operaciones son síncronas dentro de una sola
invocación, y no hay servidor persistente, colas ni servicios distribuidos.
El `correlation_id` permite reunir los eventos de una ejecución; no se inventan
`trace_id` ni `span_id`.

## Configuración

| Variable | Valores | Predeterminado |
| --- | --- | --- |
| `TUPINAMBA_LOG_LEVEL` | `trace`, `debug`, `info`, `warn`, `error` | `info` |
| `TUPINAMBA_LOG_FORMAT` | `json`, `text` | `json` |
| `TUPINAMBA_ENV` | nombre del entorno | campo omitido |
| `TUPINAMBA_LOG_FILE` | ruta local opcional | `stderr` |

Los logs de archivo se crean con permisos `0600`. Al iniciar una invocación,
un archivo existente de 10 MiB o más se rota a `.1`; solo se mantiene esa copia
y se descarta la anterior. Si el destino no puede abrirse, la aplicación sigue
con `stderr`; si una escritura de archivo falla o queda incompleta, ese evento
se intenta escribir en `stderr`. Un lock advisory de permisos `0600` se mantiene
durante la invocación; si otra instancia ya usa ese destino, la nueva instancia
continúa por `stderr` y no rota el archivo. El logger es síncrono y no mantiene
un buffer que requiera flush explícito al apagar.

Ejemplos:

```sh
TUPINAMBA_LOG_LEVEL=debug tupinamba search cafe 2>tupinamba.log
TUPINAMBA_LOG_FORMAT=text tupinamba version
TUPINAMBA_LOG_FILE="$HOME/.tupinamba/log.jsonl" tupinamba version
```

## Convenciones de eventos

Los eventos usan nombres estables en `event_name` con segmentos separados por
puntos. Se incluyen `service`, `component`, `operation` y, según la operación,
`outcome`, `duration_ms`, `error_type`, `error_code`, `status_code`, cantidades
o antigüedad de caché. El tiempo lo añade `slog` en UTC. Los campos no
aplicables —incluidos `request_id`, `trace_id` y `span_id`— se omiten.

| Flujo | Eventos representativos | Nivel habitual |
| --- | --- | --- |
| Comando CLI | `command.completed`, `command.failed`, `command.missing`, `command.unknown`, `command.panic` | `INFO`, `ERROR`, `WARN`, `ERROR` |
| HTTP externo | `http.client.completed`, `http.client.failed` | `INFO`, `WARN` para 4xx, `ERROR` para 5xx/fallos |
| Catálogo y caché | `catalog.cache.hit`, `catalog.cache.stale`, `catalog.fetch.completed`, `catalog.fetch.failed`, `catalog.cache.save_failed` | `DEBUG`, `ERROR`, `WARN` |
| Configuración | `config.load`, `config.save` | `DEBUG` |
| Sesión/autenticación | `auth.session.saved`, eventos de importación Chrome | `INFO`, `WARN`, `ERROR` |
| Entradas locales | `input.lines.read`, `auth.har.read` | `DEBUG`, `WARN` |
| Destino de logs | `logging.destination.fallback` | `WARN` |

El error HTTP se clasifica mediante códigos estables (`timeout`, `canceled`,
`transport`, `http_status`, `response_read`, `response_too_large`,
`response_decode`, `session_persist`, `response_close`). Las respuestas están
limitadas a 16 MiB y las mayores se rechazan. El logger no almacena mensajes de
error remotos ni cuerpos HTTP.

## Privacidad y seguridad

- No se registran argumentos de comandos, valores de cookies, cabeceras,
  cuerpos HTTP, contenido de archivos ni rutas locales de entradas.
- Los eventos de red registran método, ruta sin query, estado, duración y
  códigos/tipos de error; no registran URL completa ni texto remoto.
- La importación de navegador registra cantidades y tipos de fallo, nunca
  nombres ni valores de cookies.
- Los mensajes y atributos se serializan con los handlers JSON/texto de `slog`,
  que escapan caracteres de control. Las entradas no confiables no se
  interpolan en nombres de evento.
- Un panic inesperado registra su tipo y stack trace en el límite del comando,
  pero no su valor, que puede contener datos confidenciales.
- La auditoría de seguridad de Go no usa supresiones inline. La única exclusión
  de regla es la configuración externa `-exclude=G505`: Chrome macOS requiere
  PBKDF2-HMAC-SHA1 para compatibilidad con Safe Storage.

## Consultar y diagnosticar

Los logs JSON pueden filtrarse por correlación o evento, por ejemplo:

```sh
tupinamba search cafe 2>tupinamba.jsonl
jq 'select(.correlation_id == "<id>")' tupinamba.jsonl
jq 'select(.level == "ERROR" or .level == "WARN")' tupinamba.jsonl
```

Ejemplo real generado al ejecutar `tupinamba version` durante la verificación:

```json
{"time":"2026-11-15T14:37:14.159922Z","level":"INFO","msg":"command completed","service":"tupinamba","correlation_id":"908e64ed2f6a6a33f2a8da76e068cba4","component":"cli","operation":"cli_command","event_name":"command.completed","command":"version","duration_ms":0,"outcome":"success"}
```

Para añadir eventos, utiliza `slog.Default()` desde el componente existente,
mantén nombres y campos estables, elige el nivel por impacto operativo y
registra solo el contexto mínimo necesario. Añade pruebas que inspeccionen los
campos estructurados, verifiquen la redacción y cubran el camino de error.
