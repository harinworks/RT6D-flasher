# RT880 Radio Flasher

Un conjunto de herramientas para flashear firmwares en radios RT880 a través de conexión serial.

## Descripción

Este proyecto incluye dos programas principales:

1. **rt880-flasher** (`main.go`) - Flasheador principal para subir firmware a radios RT880
2. **hex2bin** (`hex2bin.go`) - Convertidor de archivos Intel HEX a binario

## Construcción

### Prerequisitos

- Go 1.21 o superior
- Dependencia: `go.bug.st/serial` (se descarga automáticamente)

### Compilar ambos binarios

```bash
# Compilar el flasheador principal
go build -o rt880-flasher main.go

# Compilar el convertidor hex2bin
go build -o hex2bin hex2bin.go
```

### Descargar dependencias

```bash
go mod download
```

## Uso

### RT880-Flasher

```bash
./rt880-flasher <puerto_serie> <archivo_firmware>
```

**Ejemplos:**
```bash
# En Linux/macOS
./rt880-flasher /dev/cu.wchusbserial112410 firmware.hex
./rt880-flasher /dev/ttyUSB0 firmware.bin

# En Windows
./rt880-flasher COM3 firmware.hex
```

**Formatos de firmware soportados:**
- Intel HEX (`.hex`)
- Binario (`.bin`)

**Procedimiento de flasheo:**
1. Conectar el cable de datos al radio
2. Apagar completamente el radio
3. Mantener presionado el botón PTT
4. Mientras se mantiene PTT, encender el radio
5. Mantener PTT por 2-3 segundos después del encendido
6. Soltar PTT - el radio debe estar en modo programación
7. Presionar Enter para iniciar la actualización

### Hex2Bin Converter

```bash
./hex2bin <archivo_hex_entrada> <archivo_bin_salida>
```

**Ejemplo:**
```bash
./hex2bin allcode.txt firmware_converted.bin
```

## Características

### RT880-Flasher
- Detección automática de puertos serie disponibles
- Soporte para archivos Intel HEX y binarios
- Protocolo de comunicación con retries y timeouts
- Verificación de checksums
- Progreso en tiempo real
- Manejo robusto de errores

### Hex2Bin
- Conversión precisa de Intel HEX a binario
- Soporte para registros extendidos de dirección
- Mapeo de direcciones ARM específico para RT880
- Validación de formato

## Archivos del Proyecto

- `main.go` - Código fuente del flasheador principal
- `hex2bin.go` - Código fuente del convertidor
- `go.mod` / `go.sum` - Configuración de dependencias Go
- `RT880G-V1_12.HEX` / `RT880G-V1_12.BIN` - Archivos de firmware de ejemplo
- `allcode.txt` - Archivo Intel HEX de ejemplo
- `convert.sh` - Script de conversión auxiliar

## Configuración Serial

- **Baudios:** 115200
- **Bits de datos:** 8
- **Paridad:** Ninguna
- **Bits de parada:** 1

## Solución de Problemas

1. **Puerto no encontrado:** Verificar que el dispositivo esté conectado y los drivers instalados
2. **Error de comunicación:** Asegurar que el radio esté en modo programación (seguir procedimiento PTT)
3. **Checksum error:** Verificar integridad del archivo de firmware
4. **Timeout:** Revisar conexión del cable y estado del radio

## Compilación en Diferentes Plataformas

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o rt880-flasher-linux main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o rt880-flasher.exe main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o rt880-flasher-macos-intel main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o rt880-flasher-macos-m1 main.go
```