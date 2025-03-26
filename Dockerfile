FROM golang:1.24.0-alpine AS builder

WORKDIR /app

# Instalar dependencias necesarias
RUN apk add --no-cache gcc musl-dev

# Copiar archivos de módulos y descargar dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN go build -o app .

# Imagen final más pequeña
FROM alpine:latest

WORKDIR /app

# Instalar dependencias necesarias
RUN apk add --no-cache tzdata ca-certificates

# Copiar binario compilado desde el builder
COPY --from=builder /app/app .

# Copiar plantillas y scripts
COPY --from=builder /app/templates ./templates

# Agregar script para esperar PostgreSQL
COPY wait-for-postgres.sh .
RUN chmod +x wait-for-postgres.sh

# Exponer puerto
EXPOSE 3000

# Comando para iniciar la aplicación
CMD ["./app"]