#!/bin/bash

# BCRDF encryption key generation script
# Usage: ./scripts/generate-key.sh [algorithm]

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "🔐 BCRDF Encryption Key Generator"
echo "=================================="

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}❌ openssl is not installed${NC}"
    echo "Install openssl to generate secure keys"
    exit 1
fi

# Default algorithm
ALGORITHM=${1:-"aes-256-gcm"}

echo -e "${YELLOW}📋 Generating key for $ALGORITHM${NC}"
echo ""

# Generate a hexadecimal key of 32 bytes (64 hex characters)
KEY_HEX=$(openssl rand -hex 32)

echo -e "${GREEN}✅ Key generated successfully!${NC}"
echo ""
echo "🔑 Hexadecimal key (32 bytes):"
echo "$KEY_HEX"
echo ""
echo "📝 Configuration to add to config.yaml:"
echo "backup:"
echo "  encryption_key: \"$KEY_HEX\""
echo "  encryption_algo: \"$ALGORITHM\""
echo ""
echo -e "${YELLOW}⚠️  IMPORTANT:${NC}"
echo "• Keep this key secure"
echo "• Never share it"
echo "• Use the same key to decrypt your backups"
echo "• Losing this key = permanent data loss"
echo ""
echo -e "${GREEN}💡 Tips:${NC}"
echo "• Store the key in a password manager"
echo "• Make a secure backup of this key"
echo "• Use environment variables for production"