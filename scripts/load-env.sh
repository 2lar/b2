#!/bin/bash

# Brain2 Environment Loader
# ==============================================================================
# This script loads environment variables from the root .env file and exports
# them for use by different components (frontend, backend, infrastructure).
#
# USAGE:
#   source ./scripts/load-env.sh                    # Load all variables
#   source ./scripts/load-env.sh frontend          # Load frontend variables only
#   source ./scripts/load-env.sh backend           # Load backend variables only
#   source ./scripts/load-env.sh infra             # Load infrastructure variables only
#   source ./scripts/load-env.sh development       # Load with development overrides
#   source ./scripts/load-env.sh production        # Load with production overrides
# ==============================================================================

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[ENV-LOADER]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[ENV-LOADER]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[ENV-LOADER]${NC} $1"
}

log_error() {
    echo -e "${RED}[ENV-LOADER]${NC} $1"
}

# Get the project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

# Check if .env file exists
if [[ ! -f "$ENV_FILE" ]]; then
    log_error ".env file not found at $ENV_FILE"
    log_info "Create one by copying .env.example:"
    log_info "  cp .env.example .env"
    return 1
fi

# Function to load environment variables from .env file
load_env_file() {
    local env_file="$1"
    local filter_prefix="$2"
    local count=0
    
    if [[ ! -f "$env_file" ]]; then
        log_error "Environment file not found: $env_file"
        return 1
    fi
    
    log_info "Loading environment variables from $env_file"
    
    # Process .env file line by line
    while IFS= read -r line || [[ -n "$line" ]]; do
        # Skip empty lines and comments
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        
        # Parse KEY=VALUE format
        if [[ "$line" =~ ^[[:space:]]*([^=]+)=[[:space:]]*(.*)[[:space:]]*$ ]]; then
            local key="${BASH_REMATCH[1]}"
            local value="${BASH_REMATCH[2]}"
            
            # Remove leading/trailing whitespace
            key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            
            # Remove quotes if present
            if [[ "$value" =~ ^\"(.*)\"$ ]] || [[ "$value" =~ ^\'(.*)\'$ ]]; then
                value="${BASH_REMATCH[1]}"
            fi
            
            # Handle variable substitution (e.g., ${SUPABASE_URL})
            while [[ "$value" =~ \$\{([^}]+)\} ]]; do
                local var_name="${BASH_REMATCH[1]}"
                local var_value="${!var_name:-}"
                value="${value/\$\{${var_name}\}/$var_value}"
            done
            
            # Apply filter if specified
            if [[ -n "$filter_prefix" ]]; then
                if [[ "$key" =~ ^${filter_prefix} ]]; then
                    export "$key"="$value"
                    ((count++))
                fi
            else
                export "$key"="$value"
                ((count++))
            fi
        fi
    done < "$env_file"
    
    if [[ -n "$filter_prefix" ]]; then
        log_success "Loaded $count variables with prefix '$filter_prefix'"
    else
        log_success "Loaded $count environment variables"
    fi
}

# Function to load frontend-specific variables
load_frontend_env() {
    log_info "Loading frontend environment variables..."
    
    # Load VITE_ prefixed variables for frontend
    load_env_file "$ENV_FILE" "VITE_"
    
    # Also load some general variables that frontend might need
    local general_vars=("PROJECT_ENV" "PROJECT_NAME" "AWS_REGION" "SUPABASE_URL" "SUPABASE_ANON_KEY")
    
    for var in "${general_vars[@]}"; do
        if grep -q "^${var}=" "$ENV_FILE"; then
            local value=$(grep "^${var}=" "$ENV_FILE" | cut -d'=' -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            # Remove quotes if present
            if [[ "$value" =~ ^\"(.*)\"$ ]] || [[ "$value" =~ ^\'(.*)\'$ ]]; then
                value="${BASH_REMATCH[1]}"
            fi
            export "$var"="$value"
            log_info "Loaded general variable: $var"
        fi
    done
    
    # Set frontend-specific derived variables
    if [[ "${PROJECT_ENV:-}" == "development" ]]; then
        export VITE_API_BASE_URL="${VITE_API_BASE_URL_LOCAL:-http://localhost:8080}"
        log_info "Set VITE_API_BASE_URL to local development URL"
    fi
}

# Function to load backend-specific variables
load_backend_env() {
    log_info "Loading backend environment variables..."
    
    # Backend needs most variables, but exclude VITE_ prefixed ones
    local count=0
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        
        if [[ "$line" =~ ^[[:space:]]*([^=]+)=[[:space:]]*(.*)[[:space:]]*$ ]]; then
            local key="${BASH_REMATCH[1]}"
            local value="${BASH_REMATCH[2]}"
            
            key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            
            # Skip VITE_ prefixed variables (frontend only)
            if [[ "$key" =~ ^VITE_ ]]; then
                continue
            fi
            
            # Remove quotes and handle variable substitution
            if [[ "$value" =~ ^\"(.*)\"$ ]] || [[ "$value" =~ ^\'(.*)\'$ ]]; then
                value="${BASH_REMATCH[1]}"
            fi
            
            while [[ "$value" =~ \$\{([^}]+)\} ]]; do
                local var_name="${BASH_REMATCH[1]}"
                local var_value="${!var_name:-}"
                value="${value/\$\{${var_name}\}/$var_value}"
            done
            
            export "$key"="$value"
            ((count++))
        fi
    done < "$ENV_FILE"
    
    log_success "Loaded $count backend environment variables"
    
    # Set backend-specific derived variables
    export ENV="${BACKEND_ENV:-${PROJECT_ENV:-development}}"
    export TABLE_NAME="${DB_TABLE_NAME:-${TABLE_NAME:-brain2-dev}}"
    export INDEX_NAME="${DB_GSI_NAME:-${INDEX_NAME:-GSI1}}"
    
    log_info "Set backend-specific derived variables"
}

# Function to load infrastructure/CDK variables
load_infra_env() {
    log_info "Loading infrastructure environment variables..."
    
    # Load all variables for infrastructure (CDK needs access to everything)
    load_env_file "$ENV_FILE"
    
    # Set CDK-specific derived variables
    export CDK_DEFAULT_ACCOUNT="${CDK_DEFAULT_ACCOUNT:-}"
    export CDK_DEFAULT_REGION="${AWS_REGION:-us-west-2}"
    
    # Set stack naming based on environment
    local env_suffix=""
    if [[ "${PROJECT_ENV:-}" != "production" ]]; then
        env_suffix="-${PROJECT_ENV:-dev}"
    fi
    
    export STACK_NAME="${STACK_NAME_PREFIX:-brain2}${env_suffix}"
    
    log_info "Set infrastructure stack name: $STACK_NAME"
}

# Function to apply environment-specific overrides
apply_environment_overrides() {
    local environment="$1"
    
    case "$environment" in
        "development"|"dev")
            log_info "Applying development environment overrides..."
            export PROJECT_ENV="development"
            export DEBUG="true"
            export LOG_LEVEL="debug"
            export MONITORING_ENABLED="false"
            export WAF_ENABLED="false"
            export VITE_DEBUG="true"
            export TABLE_NAME="${PROJECT_NAME:-brain2}-dev"
            export STACK_NAME="${PROJECT_NAME:-brain2}-dev"
            ;;
        "staging")
            log_info "Applying staging environment overrides..."
            export PROJECT_ENV="staging"
            export DEBUG="false"
            export LOG_LEVEL="info"
            export MONITORING_ENABLED="true"
            export TABLE_NAME="${PROJECT_NAME:-brain2}-staging"
            export STACK_NAME="${PROJECT_NAME:-brain2}-staging"
            ;;
        "production"|"prod")
            log_info "Applying production environment overrides..."
            export PROJECT_ENV="production"
            export DEBUG="false"
            export LOG_LEVEL="warn"
            export MONITORING_ENABLED="true"
            export WAF_ENABLED="true"
            export VITE_DEBUG="false"
            export TABLE_NAME="${PROJECT_NAME:-brain2}-prod"
            export STACK_NAME="${PROJECT_NAME:-brain2}"
            ;;
    esac
}

# Function to validate required variables
validate_environment() {
    local component="$1"
    local missing_vars=()
    
    case "$component" in
        "frontend")
            local required_vars=("VITE_SUPABASE_URL" "VITE_SUPABASE_ANON_KEY" "VITE_API_BASE_URL")
            ;;
        "backend") 
            local required_vars=("SUPABASE_URL" "SUPABASE_SERVICE_ROLE_KEY" "TABLE_NAME" "INDEX_NAME" "AWS_REGION")
            ;;
        "infra")
            local required_vars=("AWS_REGION" "SUPABASE_URL" "SUPABASE_SERVICE_ROLE_KEY")
            ;;
        *)
            local required_vars=("PROJECT_ENV" "AWS_REGION" "SUPABASE_URL")
            ;;
    esac
    
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            missing_vars+=("$var")
        fi
    done
    
    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        log_error "Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            log_error "  - $var"
        done
        log_info "Please check your .env file and ensure all required variables are set."
        return 1
    fi
    
    log_success "All required variables for $component are set"
}

# Function to display current environment info
show_environment_info() {
    echo
    log_info "Current Environment Configuration:"
    echo "  Project Environment: ${PROJECT_ENV:-not set}"
    echo "  AWS Region: ${AWS_REGION:-not set}"
    echo "  Project Name: ${PROJECT_NAME:-not set}"
    echo "  Debug Mode: ${DEBUG:-not set}"
    echo "  Log Level: ${LOG_LEVEL:-not set}"
    
    if [[ -n "${TABLE_NAME:-}" ]]; then
        echo "  Database Table: ${TABLE_NAME}"
    fi
    
    if [[ -n "${VITE_API_BASE_URL:-}" ]]; then
        echo "  Frontend API URL: ${VITE_API_BASE_URL}"
    fi
    
    if [[ -n "${STACK_NAME:-}" ]]; then
        echo "  Infrastructure Stack: ${STACK_NAME}"
    fi
    echo
}

# Main execution logic
main() {
    local component="${1:-all}"
    
    case "$component" in
        "frontend"|"fe")
            load_frontend_env
            validate_environment "frontend"
            ;;
        "backend"|"be")
            load_backend_env
            validate_environment "backend"
            ;;
        "infra"|"infrastructure")
            load_infra_env
            validate_environment "infra"
            ;;
        "development"|"dev"|"staging"|"production"|"prod")
            load_env_file "$ENV_FILE"
            apply_environment_overrides "$component"
            validate_environment "all"
            ;;
        "all"|"")
            load_env_file "$ENV_FILE"
            validate_environment "all"
            ;;
        "help"|"-h"|"--help")
            echo "Usage: source ./scripts/load-env.sh [COMPONENT|ENVIRONMENT]"
            echo
            echo "Components:"
            echo "  frontend, fe          Load frontend-specific variables"
            echo "  backend, be           Load backend-specific variables"
            echo "  infra, infrastructure Load infrastructure-specific variables"
            echo "  all                   Load all variables (default)"
            echo
            echo "Environments (with overrides):"
            echo "  development, dev      Load with development overrides"
            echo "  staging               Load with staging overrides"
            echo "  production, prod      Load with production overrides"
            echo
            echo "Examples:"
            echo "  source ./scripts/load-env.sh                    # Load all"
            echo "  source ./scripts/load-env.sh frontend           # Frontend only"
            echo "  source ./scripts/load-env.sh development        # Development env"
            echo
            return 0
            ;;
        *)
            log_error "Unknown component or environment: $component"
            log_info "Use 'source ./scripts/load-env.sh help' for usage information"
            return 1
            ;;
    esac
    
    show_environment_info
}

# Only run main if script is being sourced directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    log_error "This script must be sourced, not executed directly!"
    log_info "Usage: source ./scripts/load-env.sh [component]"
    exit 1
else
    main "$@"
fi