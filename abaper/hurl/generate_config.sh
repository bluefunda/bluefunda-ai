#!/bin/bash

# Generate Hurl configuration from environment variables
# This script reads environment variables and creates a hurl.config file

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CONFIG_FILE="hurl/hurl.config"

echo -e "${BLUE}Generating Hurl configuration from environment variables...${NC}"

# Check if environment variables are set
check_env_var() {
    local var_name=$1
    local var_value=${!var_name}
    
    if [ -z "$var_value" ]; then
        echo -e "${RED}Error: $var_name environment variable not set${NC}"
        return 1
    else
        echo -e "${GREEN}✓ $var_name=${var_value}${NC}"
        return 0
    fi
}

# Check all required variables
all_vars_set=true

if ! check_env_var "SAP_HOST"; then
    all_vars_set=false
fi

if ! check_env_var "SAP_USERNAME"; then
    all_vars_set=false
fi

if ! check_env_var "SAP_PASSWORD"; then
    all_vars_set=false
fi

if ! check_env_var "SAP_CLIENT"; then
    all_vars_set=false
fi

if [ "$all_vars_set" = false ]; then
    echo -e "${RED}Please set all required environment variables:${NC}"
    echo "export SAP_HOST='https://your-sap-server:8000'"
    echo "export SAP_USERNAME='DEVELOPER'"
    echo "export SAP_PASSWORD='your_password'"
    echo "export SAP_CLIENT='100'"
    exit 1
fi

# Generate the configuration file
echo -e "${BLUE}Creating $CONFIG_FILE...${NC}"

cat > "$CONFIG_FILE" << EOF
# Hurl Variables File
# Auto-generated from environment variables on $(date)
# Use with: hurl --variables-file hurl.config script.hurl

# SAP System Configuration
SAP_HOST=$SAP_HOST
SAP_USERNAME=$SAP_USERNAME
SAP_PASSWORD=$SAP_PASSWORD
SAP_CLIENT=$SAP_CLIENT
SAP_LANGUAGE=${SAP_LANGUAGE:-EN}
EOF

echo -e "${GREEN}✓ Configuration file created: $CONFIG_FILE${NC}"
echo ""
echo -e "${YELLOW}Usage examples:${NC}"
echo "  hurl --variables-file $CONFIG_FILE hurl/01_authentication_discovery.hurl"
echo "  hurl --variables-file $CONFIG_FILE hurl/all_endpoints.hurl"
echo ""
echo -e "${YELLOW}Test the configuration:${NC}"
echo "  hurl --variables-file $CONFIG_FILE --test hurl/01_authentication_discovery.hurl"
