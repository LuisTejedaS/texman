# Texman

Texman es un cliente HTTP de terminal para trabajar con colecciones de requests en formato JSON. Permite navegar colecciones, ejecutar requests, editar método, URL, headers y body, crear o borrar requests, importar/exportar colecciones y revisar respuestas guardadas.

La interfaz está hecha con [Bubble Tea](https://github.com/charmbracelet/bubbletea), así que corre dentro de una terminal interactiva.

## Requisitos

- Go 1.25 o superior
- Acceso a internet la primera vez que se descargan dependencias

## Instalación

Desde la raíz del proyecto:

```bash
go mod download
```

## Ejecutar en desarrollo

```bash
go run .
```

Texman lee las colecciones desde `./collections` y guarda las respuestas en `./responses`, por eso conviene ejecutarlo desde la raíz del repositorio.

## Compilar

Para compilar el binario local:

```bash
go build -o texman .
```

Luego ejecútalo con:

```bash
./texman
```

Para compilar un ejecutable de Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o texman.exe .
```

## Uso

Al abrir la app, la pantalla se divide en:

- `Collections`: lista de colecciones y requests.
- `Request`: detalle del request seleccionado.
- `Response`: resultado del último request ejecutado.

### Navegación general

| Tecla | Acción |
| --- | --- |
| `j` / `down` | Mover selección hacia abajo |
| `k` / `up` | Mover selección hacia arriba |
| `tab` | Cambiar foco entre sidebar y panel de detalle |
| `enter` | Ejecutar request seleccionado o expandir/colapsar colección |
| `space` | Expandir/colapsar colección |
| `r` | Ejecutar request; si estás sobre una colección, renombrarla |
| `q` / `ctrl+c` | Salir |

Cuando el foco está en el panel de respuesta:

| Tecla | Acción |
| --- | --- |
| `j` / `down` | Scroll hacia abajo |
| `k` / `up` | Scroll hacia arriba |
| `f` / `pgdown` | Avanzar página |
| `b` / `pgup` | Retroceder página |

### Edición de requests

Selecciona un request y usa:

| Tecla | Acción |
| --- | --- |
| `n` | Crear un request nuevo en la colección actual |
| `m` | Editar método HTTP |
| `u` | Editar URL |
| `h` | Editar headers |
| `e` | Editar body |
| `d` | Borrar request seleccionado |

Dentro de los editores:

| Tecla | Acción |
| --- | --- |
| `enter` | Confirmar el paso actual o guardar campos simples |
| `ctrl+s` | Guardar cambios |
| `esc` | Cancelar o volver al paso anterior |

En el editor de headers:

| Tecla | Acción |
| --- | --- |
| `enter` | Editar header seleccionado |
| `a` | Agregar header |
| `d` | Borrar header |
| `ctrl+s` | Guardar todos los headers |

### Colecciones

| Tecla | Acción |
| --- | --- |
| `C` | Crear colección |
| `r` | Renombrar colección seleccionada |
| `D` | Borrar colección seleccionada |
| `i` | Importar una colección desde un archivo JSON |
| `x` | Exportar la colección seleccionada a un archivo JSON |

### Respuestas guardadas

Cada request ejecutado guarda automáticamente una respuesta en `./responses` con el formato:

```text
YYYY-MM-DD_HH-MM-SS_nombre-del-request.txt
```

Para ver respuestas guardadas:

| Tecla | Acción |
| --- | --- |
| `v` | Cambiar entre colecciones y respuestas guardadas |
| `enter` | Abrir respuesta seleccionada |
| `d` | Borrar archivo de respuesta seleccionado |

## Formato de colecciones

Cada archivo `*.json` dentro de `collections` representa una colección:

```json
{
  "name": "st-location-service",
  "requests": [
    {
      "name": "ip-info",
      "method": "GET",
      "url": "https://example.com/api/v1/ip-info",
      "headers": {
        "Authorization": "Bearer token"
      },
      "body": ""
    }
  ]
}
```

Campos de un request:

- `name`: nombre visible en la lista.
- `method`: método HTTP, por ejemplo `GET`, `POST`, `PUT`, `PATCH` o `DELETE`.
- `url`: URL completa del endpoint.
- `headers`: mapa de headers HTTP.
- `body`: cuerpo del request como texto.

## Estructura del proyecto

```text
.
├── collections/          # Colecciones HTTP en JSON
├── responses/            # Respuestas guardadas automáticamente
├── internal/
│   ├── collection/       # Carga y guardado de colecciones
│   ├── httpclient/       # Ejecución de requests HTTP
│   ├── model/            # Estado y lógica de Bubble Tea
│   ├── responses/        # Guardado y listado de respuestas
│   └── ui/               # Render de paneles y estilos
├── main.go               # Punto de entrada
├── go.mod
└── go.sum
```

## Notas

- El cliente HTTP usa timeout de 30 segundos.
- Si la respuesta es JSON válido, Texman la muestra formateada.
- Los cambios a colecciones se guardan en el archivo JSON correspondiente.
- Los binarios generados, como `texman` o `texman.exe`, no son necesarios para desarrollar si usas `go run .`.
