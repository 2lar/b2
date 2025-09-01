#!/bin/bash

# Update all references from repository.CategoryRepository to category.CategoryRepository
# and from repository.GraphRepository to shared.GraphRepository

FILES=(
    "/home/wsl/b2/backend/internal/infrastructure/persistence/dynamodb/unit_of_work.go"
    "/home/wsl/b2/backend/internal/infrastructure/persistence/dynamodb/category_repository.go"
    "/home/wsl/b2/backend/internal/infrastructure/persistence/decorator_chain.go"
    "/home/wsl/b2/backend/internal/di/initialization/repositories.go"
    "/home/wsl/b2/backend/internal/di/contracts.go"
    "/home/wsl/b2/backend/internal/di/containers_clean.go"
    "/home/wsl/b2/backend/internal/di/wire_providers.go"
    "/home/wsl/b2/backend/internal/di/transaction/category_wrapper.go"
    "/home/wsl/b2/backend/internal/di/transaction/factory.go"
    "/home/wsl/b2/backend/internal/interfaces/contracts.go"
    "/home/wsl/b2/backend/internal/application/services/transaction_manager.go"
    "/home/wsl/b2/backend/internal/application/queries/category_query_service.go"
)

for file in "${FILES[@]}"; do
    echo "Processing $file..."
    
    # Add imports if not present
    if ! grep -q '"brain2-backend/internal/domain/category"' "$file"; then
        # Add category import after repository import
        sed -i '/\"brain2-backend\/internal\/repository\"/a\\t"brain2-backend/internal/domain/category"' "$file"
    fi
    
    # Replace repository.CategoryRepository with category.CategoryRepository
    sed -i 's/repository\.CategoryRepository/category.CategoryRepository/g' "$file"
done

# Now handle GraphRepository
echo "Updating GraphRepository references..."
find /home/wsl/b2/backend/internal -name "*.go" -type f -exec grep -l "repository\.GraphRepository" {} \; | while read file; do
    echo "Processing $file for GraphRepository..."
    
    # Add shared import if not present  
    if ! grep -q '"brain2-backend/internal/domain/shared"' "$file"; then
        # Add shared import after repository import
        sed -i '/\"brain2-backend\/internal\/repository\"/a\\t"brain2-backend/internal/domain/shared"' "$file"
    fi
    
    # Replace repository.GraphRepository with shared.GraphRepository
    sed -i 's/repository\.GraphRepository/shared.GraphRepository/g' "$file"
done

echo "Done!"