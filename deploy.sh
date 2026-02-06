#!/bin/bash

# Distributed Proxy System - Deployment Script
set -e

echo "==================================="
echo "Distributed Proxy System Deployment"
echo "==================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    print_info "Prerequisites check passed ✓"
}

# Generate encryption keys
generate_keys() {
    print_info "Generating encryption keys..."
    
    if command -v openssl &> /dev/null; then
        ENCRYPTION_KEY=$(openssl rand -hex 32)
        echo "ENCRYPTION_KEY=$ENCRYPTION_KEY" > .env
        print_info "Encryption key generated and saved to .env ✓"
    else
        print_warn "OpenSSL not found. Please manually generate a 32-byte hex key."
    fi
}

# Create directory structure
setup_directories() {
    print_info "Setting up directory structure..."
    
    mkdir -p config
    mkdir -p logs
    mkdir -p data
    
    print_info "Directory structure created ✓"
}

# Create Dockerfiles for each service
create_dockerfiles() {
    print_info "Creating Dockerfiles..."
    
    # Copy template to each service directory
    for dir in upstream-server central-proxy downstream-server relay-node starlink-gateway; do
        if [ -d "$dir" ]; then
            cp Dockerfile.template "$dir/Dockerfile"
            print_info "Created Dockerfile for $dir"
        fi
    done
    
    print_info "Dockerfiles created ✓"
}

# Validate configuration files
validate_configs() {
    print_info "Validating configuration files..."
    
    required_configs=(
        "config/upstream.yaml"
        "config/central.yaml"
        "config/downstream.yaml"
        "config/gateway.yaml"
        "config/relay.yaml"
    )
    
    for config in "${required_configs[@]}"; do
        if [ ! -f "$config" ]; then
            print_error "Missing configuration file: $config"
            exit 1
        fi
    done
    
    print_info "Configuration validation passed ✓"
}

# Build Docker images
build_images() {
    print_info "Building Docker images..."
    
    docker-compose build
    
    if [ $? -eq 0 ]; then
        print_info "Docker images built successfully ✓"
    else
        print_error "Failed to build Docker images"
        exit 1
    fi
}

# Start services
start_services() {
    print_info "Starting services..."
    
    docker-compose up -d
    
    if [ $? -eq 0 ]; then
        print_info "Services started successfully ✓"
    else
        print_error "Failed to start services"
        exit 1
    fi
}

# Wait for services to be ready
wait_for_services() {
    print_info "Waiting for services to be ready..."
    
    services=(
        "localhost:8001"  # upstream1
        "localhost:8080"  # central-proxy
        "localhost:8443"  # downstream1
        "localhost:9000"  # gateway
    )
    
    for service in "${services[@]}"; do
        max_attempts=30
        attempt=0
        
        while [ $attempt -lt $max_attempts ]; do
            if curl -s -f "http://${service}/health" > /dev/null 2>&1; then
                print_info "$service is ready ✓"
                break
            fi
            
            attempt=$((attempt + 1))
            if [ $attempt -eq $max_attempts ]; then
                print_error "$service failed to start"
                exit 1
            fi
            
            sleep 2
        done
    done
    
    print_info "All services are ready ✓"
}

# Display service status
show_status() {
    echo ""
    echo "==================================="
    echo "Service Status"
    echo "==================================="
    docker-compose ps
    echo ""
    
    echo "==================================="
    echo "Health Check Results"
    echo "==================================="
    
    services=(
        "Upstream-1:http://localhost:8001/health"
        "Upstream-2:http://localhost:8002/health"
        "Upstream-3:http://localhost:8003/health"
        "Central-Proxy:http://localhost:8080/health"
        "Downstream-1:http://localhost:8443/health"
        "Downstream-2:http://localhost:8444/health"
        "Downstream-3:http://localhost:8445/health"
        "Relay-1:http://localhost:8500/health"
        "Relay-2:http://localhost:8501/health"
        "Gateway:http://localhost:9000/health"
    )
    
    for service in "${services[@]}"; do
        name="${service%%:*}"
        url="${service#*:}"
        
        if response=$(curl -s "$url" 2>/dev/null); then
            echo -e "${GREEN}✓${NC} $name - Healthy"
        else
            echo -e "${RED}✗${NC} $name - Unavailable"
        fi
    done
    
    echo ""
}

# Show usage information
show_usage() {
    echo ""
    echo "==================================="
    echo "Quick Start Guide"
    echo "==================================="
    echo ""
    echo "View logs:"
    echo "  docker-compose logs -f [service-name]"
    echo ""
    echo "Stop services:"
    echo "  docker-compose down"
    echo ""
    echo "Restart services:"
    echo "  docker-compose restart"
    echo ""
    echo "Test the proxy:"
    echo "  ./scripts/test-proxy.sh"
    echo ""
    echo "Monitor services:"
    echo "  watch -n 5 './deploy.sh status'"
    echo ""
}

# Main deployment function
deploy() {
    check_prerequisites
    setup_directories
    generate_keys
    create_dockerfiles
    validate_configs
    build_images
    start_services
    wait_for_services
    show_status
    show_usage
    
    print_info "Deployment completed successfully! 🚀"
}

# Handle command line arguments
case "${1:-deploy}" in
    deploy)
        deploy
        ;;
    status)
        show_status
        ;;
    stop)
        print_info "Stopping services..."
        docker-compose down
        print_info "Services stopped ✓"
        ;;
    restart)
        print_info "Restarting services..."
        docker-compose restart
        print_info "Services restarted ✓"
        ;;
    clean)
        print_warn "This will remove all containers and volumes!"
        read -p "Are you sure? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            docker-compose down -v
            print_info "Cleanup completed ✓"
        fi
        ;;
    logs)
        docker-compose logs -f "${2:-}"
        ;;
    *)
        echo "Usage: $0 {deploy|status|stop|restart|clean|logs [service]}"
        exit 1
        ;;
esac
