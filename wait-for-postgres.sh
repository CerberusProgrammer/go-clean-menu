#!/bin/sh
# Script para esperar a que PostgreSQL esté listo

set -e

echo "Esperando a que PostgreSQL esté disponible..."
# Use hardcoded password from environment variables
export PGPASSWORD=postgres

# Use explicit values instead of environment variables
until psql -h db -U postgres -d go_clean_menu -c "SELECT 1;" > /dev/null 2>&1; do
  echo "PostgreSQL no está disponible aún - esperando..."
  sleep 1
done

echo "PostgreSQL está listo!"
exec "$@"