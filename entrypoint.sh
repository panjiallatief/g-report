#!/bin/sh

# entrypoint.sh - Manages SSL certificates and starts the app

CERT_DIR="/app/cert"

# Check if real certificates exist (mounted or bundled)
if [ -f "$CERT_DIR/fullchain.pem" ] && [ -f "$CERT_DIR/privkey.pem" ]; then
    echo "✅ Real SSL certificates found in $CERT_DIR"
    echo "🚀 Starting G-Report with trusted HTTPS..."
    exec ./main
fi

# Check if HTTPS is disabled
if [ "$DISABLE_HTTPS" = "true" ]; then
    echo "⚠️ HTTPS disabled via DISABLE_HTTPS=true"
    echo "🌐 Starting in HTTP mode..."
    rm -f /app/server.crt /app/server.key
    exec ./main
fi

# Generate self-signed certificates if no real certs and HTTPS not disabled
SELF_CERT_DIR="/app/certs"
CERT_FILE="$SELF_CERT_DIR/server.crt"
KEY_FILE="$SELF_CERT_DIR/server.key"

mkdir -p "$SELF_CERT_DIR"

if [ -f "$CERT_FILE" ] && [ -f "$KEY_FILE" ]; then
    echo "✅ Self-signed SSL certificates found"
else
    echo "🔐 Generating self-signed SSL certificates..."
    
    SERVER_IP="${SERVER_IP:-}"
    SAN="DNS:localhost,DNS:*.localhost,IP:127.0.0.1,IP:::1,IP:0.0.0.0"
    if [ -n "$SERVER_IP" ]; then
        SAN="$SAN,IP:$SERVER_IP"
        echo "   Including IP: $SERVER_IP"
    fi
    
    openssl req -x509 -nodes -days 365 -newkey ec:<(openssl ecparam -name prime256v1) \
        -keyout "$KEY_FILE" \
        -out "$CERT_FILE" \
        -subj "/C=ID/ST=DKI Jakarta/L=Jakarta/O=IT Broadcast Ops/CN=localhost" \
        -addext "subjectAltName=$SAN" \
        2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "✅ Self-signed SSL certificates generated"
    else
        echo "⚠️ Failed to generate certificates, starting without HTTPS..."
        exec ./main
    fi
fi

# Copy self-signed certs to app root
cp "$CERT_FILE" /app/server.crt
cp "$KEY_FILE" /app/server.key
echo "✅ Certificates ready for HTTPS"

# Start the application
echo "🚀 Starting G-Report..."
exec ./main
