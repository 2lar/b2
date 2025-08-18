#!/bin/bash

# Script to fix domain import paths after restructuring

echo "Starting import path fixes..."

# Find all Go files that import the old domain package
echo "Finding files with old domain imports..."
files=$(grep -r "brain2-backend/internal/domain[^/]" internal/ infrastructure/ --include="*.go" -l 2>/dev/null || true)

for file in $files; do
    echo "Processing: $file"
    
    # First, update the import statements
    sed -i 's|"brain2-backend/internal/domain"$|"brain2-backend/internal/domain/node"\n\t"brain2-backend/internal/domain/edge"\n\t"brain2-backend/internal/domain/category"\n\t"brain2-backend/internal/domain/shared"|g' "$file"
    
    # Then update type references
    sed -i 's/\*domain\.Node/\*node.Node/g' "$file"
    sed -i 's/\[\]\*domain\.Node/\[\]\*node.Node/g' "$file"
    sed -i 's/domain\.Node/node.Node/g' "$file"
    
    sed -i 's/\*domain\.Edge/\*edge.Edge/g' "$file"
    sed -i 's/\[\]\*domain\.Edge/\[\]\*edge.Edge/g' "$file"
    sed -i 's/\[\]domain\.Edge/\[\]edge.Edge/g' "$file"
    sed -i 's/domain\.Edge/edge.Edge/g' "$file"
    
    sed -i 's/\*domain\.Graph/\*shared.Graph/g' "$file"
    sed -i 's/domain\.Graph/shared.Graph/g' "$file"
    
    sed -i 's/\*domain\.Category/\*category.Category/g' "$file"
    sed -i 's/\[\]\*domain\.Category/\[\]\*category.Category/g' "$file"
    sed -i 's/\[\]domain\.Category/\[\]category.Category/g' "$file"
    sed -i 's/domain\.Category/category.Category/g' "$file"
    
    # Value object and ID types
    sed -i 's/domain\.NodeID/shared.NodeID/g' "$file"
    sed -i 's/domain\.UserID/shared.UserID/g' "$file"
    sed -i 's/domain\.CategoryID/shared.CategoryID/g' "$file"
    sed -i 's/domain\.EdgeID/shared.EdgeID/g' "$file"
    sed -i 's/domain\.GraphID/shared.GraphID/g' "$file"
    
    # Factory methods and functions
    sed -i 's/domain\.NewNode/node.NewNode/g' "$file"
    sed -i 's/domain\.NewEdge/edge.NewEdge/g' "$file"
    sed -i 's/domain\.NewCategory/category.NewCategory/g' "$file"
    sed -i 's/domain\.NewUserID/shared.NewUserID/g' "$file"
    sed -i 's/domain\.NewNodeID/shared.NewNodeID/g' "$file"
    sed -i 's/domain\.ParseNodeID/shared.ParseNodeID/g' "$file"
    sed -i 's/domain\.ReconstructNodeFromPrimitives/node.ReconstructNodeFromPrimitives/g' "$file"
    
    # Event types
    sed -i 's/domain\.EventBus/shared.EventBus/g' "$file"
    sed -i 's/domain\.DomainEvent/shared.DomainEvent/g' "$file"
    
    # Error types
    sed -i 's/domain\.ErrValidation/shared.ErrValidation/g' "$file"
    sed -i 's/domain\.ErrNotFound/shared.ErrNotFound/g' "$file"
    
    # Category related types
    sed -i 's/domain\.CategorySuggestion/category.CategorySuggestion/g' "$file"
    sed -i 's/domain\.NodeCategory/category.NodeCategory/g' "$file"
    sed -i 's/domain\.CategoryHierarchy/category.CategoryHierarchy/g' "$file"
    
    echo "  ✓ Fixed imports in $file"
done

echo "Import fixes completed!"
echo "Files processed:"
for file in $files; do
    echo "  - $file"
done