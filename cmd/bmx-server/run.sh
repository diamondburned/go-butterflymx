#!/usr/bin/with-contenv bashio

echo "Starting ButterflyMX Proxy Server Addon..."

export HOST="0.0.0.0"
export PORT="80"

if bashio::config.has_value 'api_secret'; then
    export API_SECRET=$(bashio::config 'api_secret')
fi

if bashio::config.has_value 'butterflymx_api_token'; then
    export BUTTERFLYMX_API_TOKEN=$(bashio::config 'butterflymx_api_token')
fi

if bashio::config.has_value 'enable_mtls' && bashio::config.true 'enable_mtls'; then
    export ENABLE_MTLS="true"
fi

if bashio::config.has_value 'tls_cert_path'; then
    export TLS_CERT_PATH=$(bashio::config 'tls_cert_path')
fi

if bashio::config.has_value 'tls_key_path'; then
    export TLS_KEY_PATH=$(bashio::config 'tls_key_path')
fi

if bashio::config.has_value 'client_ca_path'; then
    export CLIENT_CA_PATH=$(bashio::config 'client_ca_path')
fi

echo "Configuration loaded. Launching ButterflyMX Proxy Server on $HOST:$PORT..."
exec /usr/bin/bmx-server
