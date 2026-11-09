#!/bin/bash

# ABAP ADT API Hurl Test Runner
# Convenience script for running common ADT API workflows

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if environment variables are set
check_env() {
    echo -e "${BLUE}Checking environment variables...${NC}"

    if [ -z "$SAP_HOST" ]; then
        echo -e "${RED}Error: SAP_HOST environment variable not set${NC}"
        echo "Example: export SAP_HOST='https://vhcalnplci.local:8000'"
        exit 1
    fi

    if [ -z "$SAP_USERNAME" ]; then
        echo -e "${RED}Error: SAP_USERNAME environment variable not set${NC}"
        echo "Example: export SAP_USERNAME='DEVELOPER'"
        exit 1
    fi

    if [ -z "$SAP_PASSWORD" ]; then
        echo -e "${RED}Error: SAP_PASSWORD environment variable not set${NC}"
        echo "Example: export SAP_PASSWORD='your_password'"
        exit 1
    fi

    if [ -z "$SAP_CLIENT" ]; then
        echo -e "${RED}Error: SAP_CLIENT environment variable not set${NC}"
        echo "Example: export SAP_CLIENT='100'"
        exit 1
    fi

    echo -e "${GREEN}✓ All environment variables are set${NC}"
    echo "  SAP_HOST: $SAP_HOST"
    echo "  SAP_USERNAME: $SAP_USERNAME"
    echo "  SAP_CLIENT: $SAP_CLIENT"
    echo ""
}

# Generate Hurl configuration from environment variables
generate_hurl_config() {
    cat > hurl.config << EOF
# Hurl Variables File - Auto-generated from environment variables
SAP_HOST=$SAP_HOST
SAP_USERNAME=$SAP_USERNAME
SAP_PASSWORD=$SAP_PASSWORD
SAP_CLIENT=$SAP_CLIENT
SAP_LANGUAGE=${SAP_LANGUAGE:-EN}
EOF
}

# Generate config and run authentication
authenticate() {
    echo -e "${BLUE}Running authentication...${NC}"
    generate_hurl_config
    hurl --variables-file hurl.config 01_authentication_discovery.hurl
    echo -e "${GREEN}✓ Authentication completed${NC}"
    echo ""
}

# Run all endpoints
run_all() {
    echo -e "${BLUE}Running all ADT API endpoints...${NC}"
    generate_hurl_config
    hurl --variables-file hurl.config all_endpoints.hurl
    echo -e "${GREEN}✓ All endpoints completed${NC}"
}

# Test repository functions
test_repository() {
    echo -e "${BLUE}Testing repository management...${NC}"
    authenticate
    hurl --variables-file hurl.config 02_repository_management.hurl
    echo -e "${GREEN}✓ Repository management tests completed${NC}"
}

# Test object management
test_objects() {
    echo -e "${BLUE}Testing object management...${NC}"
    authenticate
    hurl --variables-file hurl.config 03_object_management.hurl
    echo -e "${GREEN}✓ Object management tests completed${NC}"
}

# Test code development tools
test_development() {
    echo -e "${BLUE}Testing code development tools...${NC}"
    authenticate
    hurl --variables-file hurl.config 06_code_development.hurl
    echo -e "${GREEN}✓ Code development tests completed${NC}"
}

# Test quality tools
test_quality() {
    echo -e "${BLUE}Testing quality and ATC tools...${NC}"
    authenticate
    hurl --variables-file hurl.config 10_atc_quality.hurl
    echo -e "${GREEN}✓ Quality tools tests completed${NC}"
}

# Run individual category
run_category() {
    local category=$1
    echo -e "${BLUE}Running category: $category${NC}"

    case $category in
        1|auth|authentication)
            generate_hurl_config
            hurl --variables-file hurl.config 01_authentication_discovery.hurl
            ;;
        2|repo|repository)
            test_repository
            ;;
        3|obj|objects)
            test_objects
            ;;
        4|activation)
            authenticate
            hurl --variables-file hurl.config 04_activation.hurl
            ;;
        5|transport)
            authenticate
            hurl --variables-file hurl.config 05_transport_management.hurl
            ;;
        6|dev|development)
            test_development
            ;;
        7|test|testing)
            authenticate
            hurl --variables-file hurl.config 07_testing.hurl
            ;;
        8|data)
            authenticate
            hurl --variables-file hurl.config 08_data_services.hurl
            ;;
        9|debug|debugger)
            authenticate
            hurl --variables-file hurl.config 09_debugger.hurl
            ;;
        10|atc|quality)
            test_quality
            ;;
        11|traces|performance)
            authenticate
            hurl --variables-file hurl.config 11_traces_performance.hurl
            ;;
        12|feeds|runtime)
            authenticate
            hurl --variables-file hurl.config 12_feeds_runtime.hurl
            ;;
        *)
            echo -e "${RED}Unknown category: $category${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Show help
show_help() {
    echo -e "${YELLOW}ABAP ADT API Hurl Test Runner${NC}"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  check           Check environment variables"
    echo "  auth            Run authentication only"
    echo "  all             Run all endpoints"
    echo "  category <num>  Run specific category (1-12)"
    echo ""
    echo "Category shortcuts:"
    echo "  repo            Repository management"
    echo "  objects         Object management"
    echo "  dev             Code development"
    echo "  quality         ATC & Quality checks"
    echo ""
    echo "Available categories:"
    echo "  1  auth          🔐 Authentication & Discovery"
    echo "  2  repo          📂 Repository Management"
    echo "  3  objects       📄 Object Management"
    echo "  4  activation    ⚡ Activation"
    echo "  5  transport     🚚 Transport Management"
    echo "  6  dev           💻 Code Development"
    echo "  7  testing       🧪 Testing"
    echo "  8  data          📊 Data & Services"
    echo "  9  debug         🐞 Debugger"
    echo "  10 quality       🔍 ATC & Quality"
    echo "  11 traces        📊 Traces & Performance"
    echo "  12 feeds         📋 Feeds & Runtime"
    echo ""
    echo "Environment variables required:"
    echo "  SAP_HOST        SAP server URL (e.g., https://vhcalnplci.local:8000)"
    echo "  SAP_USERNAME    SAP username"
    echo "  SAP_PASSWORD    SAP password"
    echo "  SAP_CLIENT      SAP client number (e.g., 100)"
    echo ""
    echo "Examples:"
    echo "  $0 check                    # Check environment setup"
    echo "  $0 auth                     # Run authentication only"
    echo "  $0 all                      # Run all endpoints"
    echo "  $0 category 2               # Run repository management"
    echo "  $0 repo                     # Run repository management (shortcut)"
    echo "  $0 objects                  # Run object management"
    echo "  $0 dev                      # Run code development tools"
    echo ""
    echo "Manual usage with generated config:"
    echo "  hurl --variables-file hurl.config 01_authentication_discovery.hurl"
    echo "  hurl --variables-file hurl.config --test all_endpoints.hurl"
}

# Main script logic
main() {
    if [ $# -eq 0 ]; then
        show_help
        exit 0
    fi

    local command=$1
    shift

    case $command in
        check)
            check_env
            ;;
        auth|authentication)
            check_env
            authenticate
            ;;
        all)
            check_env
            run_all
            ;;
        category)
            if [ $# -eq 0 ]; then
                echo -e "${RED}Error: Category number required${NC}"
                show_help
                exit 1
            fi
            check_env
            run_category $1
            ;;
        repo|repository)
            check_env
            test_repository
            ;;
        obj|objects)
            check_env
            test_objects
            ;;
        dev|development)
            check_env
            test_development
            ;;
        quality|atc)
            check_env
            test_quality
            ;;
        help|-h|--help)
            show_help
            ;;
        *)
            # Try to run as category
            check_env
            run_category $command
            ;;
    esac
}

# Change to the script directory
cd "$(dirname "$0")"

# Run main function
main "$@"
