#!/bin/bash

# All-Chat Setup Script
# This script helps set up the development environment

set -e

echo "🚀 All-Chat Setup Script"
echo "========================"
echo ""

# Check prerequisites
echo "📋 Checking prerequisites..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.23 or higher."
    exit 1
fi
echo "✅ Go $(go version | awk '{print $3}')"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker."
    exit 1
fi
echo "✅ Docker $(docker --version | awk '{print $3}' | sed 's/,//')"

# Check Docker Compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not installed."
    exit 1
fi
echo "✅ Docker Compose available"

echo ""
echo "📦 Installing Go dependencies..."
go mod download
go mod tidy
echo "✅ Dependencies installed"

echo ""
echo "🔧 Setting up environment..."

# Check if .env exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found. Copying from .env.example..."
    cp .env.example .env
    echo "✅ .env file created"
    echo ""
    echo "⚠️  IMPORTANT: Please edit .env and add your Twitch credentials:"
    echo "   - TWITCH_CLIENT_ID"
    echo "   - TWITCH_CLIENT_SECRET"
    echo "   - TWITCH_BOT_USERNAME"
    echo "   - TWITCH_BOT_OAUTH"
    echo ""
    echo "   Get them from: https://dev.twitch.tv/console/apps"
    echo "   OAuth token: https://twitchapps.com/tmi/"
    echo ""
    read -p "Press enter once you've updated .env..."
else
    echo "✅ .env file exists"
fi

echo ""
echo "🏗️  Building services..."
make build
echo "✅ All services built"

echo ""
echo "🐳 Starting Docker services..."
cd deployments
docker-compose up -d postgres redis
echo "✅ PostgreSQL and Redis started"

echo ""
echo "⏳ Waiting for PostgreSQL to be ready..."
sleep 5

echo ""
echo "🗄️  Running database migrations..."
cd ..
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -f migrations/001_initial_schema.sql 2>/dev/null || echo "⚠️  Migrations may have already been applied"

echo ""
echo "✅ Setup complete!"
echo ""
echo "📚 Next steps:"
echo "   1. Verify your .env file has Twitch credentials"
echo "   2. Start all services: make docker-up"
echo "   3. View logs: make docker-logs"
echo "   4. Access API: http://localhost:8080"
echo ""
echo "📖 Documentation:"
echo "   - README.md - Setup guide"
echo "   - docs/PROJECT_SUMMARY.md - Overview"
echo "   - docs/NEXT_STEPS_GUIDE.md - Complete remaining services"
echo ""
echo "🎉 Happy coding!"
