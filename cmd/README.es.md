# qgo

`qgo` es el CLI de QuicksGo para generar servicios nuevos con Gin o gRPC.

## Instalacion

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
```

## Comandos

Crear un servicio Gin:

```bash
qgo new gin
```

Crear un servicio gRPC:

```bash
qgo new grpc
```

El generador te pedira:

- el nombre del modulo o paquete Go
- el valor de `app.name`
- el formato de configuracion: `yaml` o `json`

Restricciones:

- el nombre del modulo o paquete no puede contener espacios ni caracteres especiales no soportados
- `app.name` no puede contener espacios ni caracteres especiales

Luego crea:

- `main.go`
- `application.yaml` o `application.json`
- `go.mod`
- una carpeta del proyecto con el nombre de `app.name` por defecto, salvo que uses `--dir`

Y al final ejecuta:

```bash
go mod init <tu-modulo>
go mod tidy
```

Eso instala las dependencias automaticamente.
