#!/bin/bash
# Shell script wrapper for updating Slovenian addresses using Docker Compose
# This script runs the Python update script inside the ClickHouse container

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default values
CSV_FILE=""
USE_DOCKER=true

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --csv-file|--csv|-f)
            CSV_FILE="$2"
            shift 2
            ;;
        --no-docker)
            USE_DOCKER=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 --csv-file <csv_file> [options]"
            echo ""
            echo "Options:"
            echo "  --csv-file, --csv, -f    Path to CSV file with Slovenian addresses (required)"
            echo "  --no-docker              Run directly (not via docker-compose)"
            echo "  --help, -h               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --csv-file addresses.csv"
            echo "  $0 -f addresses.csv"
            exit 0
            ;;
        *)
            if [ -z "$CSV_FILE" ]; then
                CSV_FILE="$1"
            else
                echo "Unknown option: $1" >&2
                exit 1
            fi
            shift
            ;;
    esac
done

# Check if CSV file is provided
if [ -z "$CSV_FILE" ]; then
    echo "Error: CSV file is required" >&2
    echo "Usage: $0 --csv-file <csv_file>" >&2
    exit 1
fi

# Check if CSV file exists
if [ ! -f "$CSV_FILE" ]; then
    echo "Error: CSV file not found: $CSV_FILE" >&2
    exit 1
fi

# Get absolute path to CSV file
CSV_FILE_ABS="$(cd "$(dirname "$CSV_FILE")" && pwd)/$(basename "$CSV_FILE")"

if [ "$USE_DOCKER" = true ]; then
    # Check if docker-compose is available
    if ! command -v docker-compose &> /dev/null && ! command -v docker &> /dev/null; then
        echo "Error: docker-compose or docker not found" >&2
        exit 1
    fi
    
    # Check if we're in a docker-compose environment
    CLICKHOUSE_RUNNING=false
    if docker-compose ps clickhouse 2>/dev/null | grep -q "Up"; then
        CLICKHOUSE_RUNNING=true
    elif docker ps --filter "name=openpaq-clickhouse" --format "{{.Names}}" | grep -q "openpaq-clickhouse"; then
        CLICKHOUSE_RUNNING=true
    fi
    
    if [ "$CLICKHOUSE_RUNNING" = true ]; then
        echo "Using Docker Compose environment..."
        
        # Copy CSV file to a location accessible by the container
        # We'll mount it as a volume
        CSV_BASENAME="$(basename "$CSV_FILE_ABS")"
        CSV_DIR="$(dirname "$CSV_FILE_ABS")"
        
        # Check if Python script exists
        if [ ! -f "$SCRIPT_DIR/update_slovenian_addresses.py" ]; then
            echo "Error: Python script not found: $SCRIPT_DIR/update_slovenian_addresses.py" >&2
            exit 1
        fi
        
        # Get the network name from the clickhouse container
        CLICKHOUSE_CONTAINER=$(docker-compose ps -q clickhouse 2>/dev/null || docker ps --filter "name=openpaq-clickhouse" -q | head -1)
        if [ -z "$CLICKHOUSE_CONTAINER" ]; then
            echo "Error: ClickHouse container not found" >&2
            exit 1
        fi
        
        # Get the network name dynamically
        NETWORK_NAME=$(docker inspect --format '{{range $net, $conf := .NetworkSettings.Networks}}{{$net}}{{end}}' "$CLICKHOUSE_CONTAINER" 2>/dev/null | head -1)
        if [ -z "$NETWORK_NAME" ]; then
            echo "Error: Could not determine Docker network name" >&2
            exit 1
        fi
        
        echo "Detected network: $NETWORK_NAME"
        echo "Running update script via Docker..."
        
        # Run Python script inside a temporary container with access to both files
        docker run --rm \
            --network "$NETWORK_NAME" \
            -v "$CSV_DIR:/data/csv:ro" \
            -v "$SCRIPT_DIR:/data/scripts:ro" \
            -w /data/scripts \
            python:3.11-slim \
            bash -c "
                set -e
                pip install -q clickhouse-driver 2>/dev/null || true
                python3 update_slovenian_addresses.py /data/csv/$CSV_BASENAME \
                    --host clickhouse \
                    --port 9000 \
                    --user default \
                    --password default
            "
    else
        echo "Error: ClickHouse container not found. Make sure docker-compose is running." >&2
        echo "Run: docker-compose up -d" >&2
        exit 1
    fi
else
    # Run directly (requires clickhouse-client and Python)
    echo "Running update script directly..."
    python3 "$SCRIPT_DIR/update_slovenian_addresses.py" "$CSV_FILE_ABS"
fi

