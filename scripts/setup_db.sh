#!/bin/bash
# Tracehound Indexer - Database Setup Script
# 🐕 Sets up PostgreSQL database and runs migrations
#
# Usage:
#   ./scripts/setup_db.sh
#
# Environment variables:
#   POSTGRES_HOST     - Database host (default: localhost)
#   POSTGRES_PORT     - Database port (default: 5432)
#   POSTGRES_USER     - Database user (default: hound)
#   POSTGRES_PASSWORD - Database password (default: trackhound123)
#   POSTGRES_DB       - Database name (default: tracehound)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
POSTGRES_HOST=${POSTGRES_HOST:-localhost}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_USER=${POSTGRES_USER:-hound}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-trackhound123}
POSTGRES_DB=${POSTGRES_DB:-tracehound}
MIGRATIONS_DIR="internal/storage/migrations"

echo ""
echo "     /\_/\\"
echo "    ( o.o )  ~~~ 💨📊"
echo "     > ^ <"
echo "    /|   |\\"
echo "   (_|   |_)"
echo ""
echo "  Tracehound Database Setup"
echo "  ━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if psql is installed
if ! command -v psql &> /dev/null; then
    echo -e "${RED}❌ Error: psql is not installed${NC}"
    echo "Please install PostgreSQL client tools."
    exit 1
fi

# Wait for PostgreSQL to be ready
echo -e "${YELLOW}⏳ Waiting for PostgreSQL to be ready...${NC}"
max_attempts=30
attempt=0

while ! PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d postgres -c '\q' 2>/dev/null; do
    attempt=$((attempt + 1))
    if [ $attempt -ge $max_attempts ]; then
        echo -e "${RED}❌ Error: PostgreSQL is not ready after $max_attempts attempts${NC}"
        exit 1
    fi
    echo "Attempt $attempt/$max_attempts..."
    sleep 2
done

echo -e "${GREEN}✅ PostgreSQL is ready${NC}"

# Check if database exists, create if not
echo -e "${YELLOW}🗄️ Checking database...${NC}"
if ! PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -lqt | cut -d \| -f 1 | grep -qw $POSTGRES_DB; then
    echo "Creating database: $POSTGRES_DB"
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d postgres -c "CREATE DATABASE $POSTGRES_DB;"
    echo -e "${GREEN}✅ Database created${NC}"
else
    echo -e "${GREEN}✅ Database already exists${NC}"
fi

# Run migrations
echo -e "${YELLOW}🔄 Running migrations...${NC}"
for migration in $(ls -v $MIGRATIONS_DIR/*.sql 2>/dev/null); do
    echo "Applying: $migration"
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -f "$migration"
done

echo -e "${GREEN}✅ Migrations complete${NC}"

# Verify setup
echo -e "${YELLOW}🔍 Verifying setup...${NC}"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c "\dt"

echo ""
echo -e "${GREEN}🐕 Database setup complete!${NC}"
echo ""
echo "Connection details:"
echo "  Host:     $POSTGRES_HOST"
echo "  Port:     $POSTGRES_PORT"
echo "  Database: $POSTGRES_DB"
echo "  User:     $POSTGRES_USER"
echo ""
echo "Connection string:"
echo "  postgres://$POSTGRES_USER:****@$POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable"
echo ""
