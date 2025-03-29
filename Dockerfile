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
RUN chmod +x app

# Imagen final más pequeña
FROM alpine:latest

WORKDIR /app

# Instalar dependencias necesarias incluyendo postgresql-client
RUN apk add --no-cache tzdata ca-certificates postgresql-client

# Copiar binario compilado desde el builder
COPY --from=builder /app/app .

# Copiar plantillas y scripts asegurando que se copien las subcarpetas
COPY templates/ /app/templates/
COPY .env /app/.env

# Crear el script de espera para PostgreSQL directamente en el contenedor
RUN echo '#!/bin/sh\n\
set -e\n\
\n\
echo "Esperando a que PostgreSQL esté disponible..."\n\
export PGPASSWORD=$DB_PASSWORD\n\
\n\
until psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;" > /dev/null 2>&1; do\n\
  echo "PostgreSQL no está disponible aún - esperando..."\n\
  sleep 1\n\
done\n\
\n\
echo "PostgreSQL está listo!"\n\
exec "$@"' > /app/wait-for-postgres.sh

RUN chmod +x /app/wait-for-postgres.sh

# Exponer puerto
EXPOSE 3001

# Comando para iniciar la aplicación
CMD ["./app"]