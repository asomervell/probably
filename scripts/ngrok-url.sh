#!/bin/bash
# Print the current public ngrok HTTPS URL.
# Usage: bash scripts/ngrok-url.sh
# Returns empty string if ngrok is not running.

url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null \
    | grep -o '"public_url":"https://[^"]*"' \
    | head -1 \
    | cut -d'"' -f4)

if [ -n "$url" ]; then
    echo "$url"
else
    echo "ngrok is not running (start with: scripts/cloud-setup.sh)" >&2
    exit 1
fi
