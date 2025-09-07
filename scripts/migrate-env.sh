#!/bin/bash

# Brain2 Environment Migration Helper
# ==============================================================================
# This script helps migrate from multiple component .env files to the unified
# root .env file system. It will:
# 1. Backup existing .env files
# 2. Merge variables into root .env
# 3. Validate the migration
# 4. Optionally clean up old files
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
    echo -e "${BLUE}[MIGRATE]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[MIGRATE]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[MIGRATE]${NC} $1"
}

log_error() {
    echo -e "${RED}[MIGRATE]${NC} $1"
}

# Get the project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Configuration
ROOT_ENV_FILE=".env"
ROOT_ENV_EXAMPLE=".env.example"
BACKUP_DIR="env-migration-backup-$(date +%Y%m%d-%H%M%S)"

# Component .env files to check
declare -A COMPONENT_FILES=(
    ["frontend"]="frontend/.env"
    ["infra"]="infra/.env"
    ["backend2"]="backend2/.env"
)

# Display banner
echo
log_info "=================================================="
log_info "Brain2 Environment Migration Helper"
log_info "=================================================="
echo

# Function to check if any component .env files exist
check_existing_files() {
    local found_files=()
    
    for component in "${!COMPONENT_FILES[@]}"; do
        local file="${COMPONENT_FILES[$component]}"
        if [[ -f "$file" ]]; then
            found_files+=("$file")
        fi
    done
    
    if [[ ${#found_files[@]} -eq 0 ]]; then
        log_info "No component .env files found. Migration not needed."
        log_info "To set up environment from scratch, run:"
        log_info "  cp .env.example .env"
        log_info "  # Edit .env with your values"
        exit 0
    fi
    
    log_info "Found ${#found_files[@]} component .env files:"
    for file in "${found_files[@]}"; do
        log_info "  ✓ $file"
    done
    echo
}

# Function to create backup directory
create_backup() {
    log_info "Creating backup directory: $BACKUP_DIR"
    mkdir -p "$BACKUP_DIR"
    
    for component in "${!COMPONENT_FILES[@]}"; do
        local file="${COMPONENT_FILES[$component]}"
        if [[ -f "$file" ]]; then
            log_info "Backing up: $file"
            cp "$file" "$BACKUP_DIR/$(basename $(dirname $file))-$(basename $file)"
        fi
    done
    
    # Also backup existing root .env if it exists
    if [[ -f "$ROOT_ENV_FILE" ]]; then
        log_warning "Existing root .env file found - backing up as root-env-existing"
        cp "$ROOT_ENV_FILE" "$BACKUP_DIR/root-env-existing"
    fi
    
    log_success "Backup created successfully"
    echo
}

# Function to extract variables from a .env file
extract_variables() {
    local file="$1"
    local temp_file=$(mktemp)
    
    if [[ ! -f "$file" ]]; then
        return
    fi
    
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
            
            echo "$key=$value" >> "$temp_file"
        fi
    done < "$file"
    
    echo "$temp_file"
}

# Function to merge variables into root .env
merge_variables() {
    log_info "Merging variables into root .env file..."
    
    # Start with .env.example as base
    if [[ -f "$ROOT_ENV_EXAMPLE" ]]; then
        log_info "Using .env.example as base template"
        cp "$ROOT_ENV_EXAMPLE" "$ROOT_ENV_FILE"
    else
        log_warning "No .env.example found, creating empty .env file"
        touch "$ROOT_ENV_FILE"
    fi
    
    # Track variables we've seen to avoid duplicates
    declare -A seen_vars
    declare -A var_sources
    
    # First pass - collect all variables and their sources
    for component in "${!COMPONENT_FILES[@]}"; do
        local file="${COMPONENT_FILES[$component]}"
        if [[ -f "$file" ]]; then
            log_info "Processing variables from $file..."
            
            while IFS='=' read -r key value; do
                [[ -z "$key" ]] && continue
                
                if [[ -n "${seen_vars[$key]}" && "${seen_vars[$key]}" != "$value" ]]; then
                    log_warning "Variable '$key' has different values:"
                    log_warning "  ${var_sources[$key]}: ${seen_vars[$key]}"
                    log_warning "  $file: $value"
                    log_warning "  Using value from $file (last wins)"
                fi
                
                seen_vars["$key"]="$value"
                var_sources["$key"]="$file"
            done < <(extract_variables "$file" | cat)
        fi
    done
    
    # Second pass - update root .env with collected variables
    local temp_env=$(mktemp)
    local updated_count=0
    
    while IFS= read -r line || [[ -n "$line" ]]; do
        # Handle variable lines
        if [[ "$line" =~ ^[[:space:]]*([^=]+)=[[:space:]]*(.*)[[:space:]]*$ ]]; then
            local key="${BASH_REMATCH[1]}"
            key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            
            # If we have a value from component files, use it
            if [[ -n "${seen_vars[$key]}" ]]; then
                echo "$key=${seen_vars[$key]}" >> "$temp_env"
                log_info "  Updated: $key (from ${var_sources[$key]})"
                ((updated_count++))
                # Mark as processed
                unset seen_vars["$key"]
            else
                # Keep original line from .env.example
                echo "$line" >> "$temp_env"
            fi
        else
            # Keep comments and empty lines as-is
            echo "$line" >> "$temp_env"
        fi
    done < "$ROOT_ENV_FILE"
    
    # Add any remaining variables that weren't in the template
    if [[ ${#seen_vars[@]} -gt 0 ]]; then
        echo "" >> "$temp_env"
        echo "# Additional variables from component .env files" >> "$temp_env"
        for key in "${!seen_vars[@]}"; do
            echo "$key=${seen_vars[$key]}" >> "$temp_env"
            log_info "  Added: $key (from ${var_sources[$key]})"
            ((updated_count++))
        done
    fi
    
    # Replace the .env file
    mv "$temp_env" "$ROOT_ENV_FILE"
    
    log_success "Merged $updated_count variables into root .env"
    echo
}

# Function to validate the migration
validate_migration() {
    log_info "Validating migration..."
    
    # Try to load the environment
    if source ./scripts/load-env.sh all; then
        log_success "Environment loading test passed"
    else
        log_error "Environment loading test failed"
        return 1
    fi
    
    # Check for required variables
    local required_vars=(
        "PROJECT_ENV"
        "AWS_REGION"
        "SUPABASE_URL"
    )
    
    local missing_vars=()
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            missing_vars+=("$var")
        fi
    done
    
    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        log_warning "Some important variables are not set:"
        for var in "${missing_vars[@]}"; do
            log_warning "  - $var"
        done
        log_warning "You may need to manually set these values in .env"
    else
        log_success "All important variables are set"
    fi
    
    echo
}

# Function to show next steps
show_next_steps() {
    log_info "=================================================="
    log_success "Migration completed successfully!"
    log_info "=================================================="
    echo
    
    log_info "Next steps:"
    echo
    log_info "1. Review and edit the root .env file:"
    log_info "   nano .env"
    echo
    log_info "2. Test the build system:"
    log_info "   ./build.sh"
    echo
    log_info "3. Test individual components:"
    log_info "   cd frontend && npm run dev:with-env"
    log_info "   cd infra && npm run synth:with-env"
    echo
    log_info "4. If everything works, clean up old files:"
    log_info "   $0 --cleanup"
    echo
    log_info "Backup location: $BACKUP_DIR"
    log_warning "Keep this backup until you've verified everything works!"
    echo
}

# Function to clean up old files
cleanup_old_files() {
    log_info "Cleaning up old component .env files..."
    
    for component in "${!COMPONENT_FILES[@]}"; do
        local file="${COMPONENT_FILES[$component]}"
        if [[ -f "$file" ]]; then
            log_info "Removing: $file"
            rm "$file"
        fi
    done
    
    log_success "Cleanup completed"
    log_info "Old files are still available in backup: $BACKUP_DIR"
    echo
}

# Function to show help
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Migrate from multiple component .env files to unified root .env"
    echo
    echo "Options:"
    echo "  --cleanup     Clean up old component .env files (run after testing)"
    echo "  --help, -h    Show this help message"
    echo
    echo "Migration process:"
    echo "  1. Backs up existing .env files"
    echo "  2. Merges variables into root .env"
    echo "  3. Validates the migration"
    echo "  4. Shows next steps"
    echo
    echo "Examples:"
    echo "  $0            Run migration"
    echo "  $0 --cleanup  Clean up old files after successful migration"
}

# Function to confirm action
confirm_action() {
    local message="$1"
    local default="${2:-n}"
    
    echo
    log_warning "$message"
    if [[ "$default" == "y" ]]; then
        read -p "Continue? [Y/n]: " -r response
        response=${response:-y}
    else
        read -p "Continue? [y/N]: " -r response
        response=${response:-n}
    fi
    
    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            log_info "Operation cancelled"
            exit 0
            ;;
    esac
}

# Main execution
main() {
    case "${1:-}" in
        --cleanup)
            log_info "Cleanup mode - removing old component .env files"
            confirm_action "This will permanently delete component .env files"
            cleanup_old_files
            ;;
        --help|-h)
            show_help
            ;;
        "")
            # Full migration
            check_existing_files
            confirm_action "This will merge component .env files into root .env" "y"
            create_backup
            merge_variables
            validate_migration
            show_next_steps
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"