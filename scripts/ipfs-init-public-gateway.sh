#!/bin/sh
# Configure kubo to serve path-mode gateway for the operator's public hostname.
# Without this, kubo redirects /ipfs/<CID> requests to subdomain mode
# (<CID>.ipfs.<host>) which breaks unless the operator has wildcard DNS + TLS.
#
# Set MIRROR_HOST in docker-compose (e.g. MIRROR_HOST=cjp.mirror.example) to
# the hostname your reverse proxy serves on. Defaults to no public gateway
# config — only the localhost gateway works (which is fine for local-only).
#
# This script runs once at container startup via kubo's /container-init.d/.

set -eu

if [ -z "${MIRROR_HOST:-}" ]; then
  echo "ipfs-init: MIRROR_HOST not set, skipping public gateway config"
  exit 0
fi

echo "ipfs-init: configuring path-mode public gateway for $MIRROR_HOST"
ipfs config --json "Gateway.PublicGateways" "{
  \"$MIRROR_HOST\": {
    \"Paths\": [\"/ipfs\", \"/ipns\"],
    \"UseSubdomains\": false
  }
}"
