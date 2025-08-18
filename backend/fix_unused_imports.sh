#!/bin/bash

echo "Removing unused imports from repository files..."

# Fix query_builder.go
sed -i '/\t"brain2-backend\/internal\/domain\/edge"/d' internal/repository/query_builder.go
sed -i '/\t"brain2-backend\/internal\/domain\/category"/d' internal/repository/query_builder.go

# Fix transaction.go
sed -i '/\t"brain2-backend\/internal\/domain\/edge"/d' internal/repository/transaction.go
sed -i '/\t"brain2-backend\/internal\/domain\/category"/d' internal/repository/transaction.go
sed -i '/\t"brain2-backend\/internal\/domain\/shared"/d' internal/repository/transaction.go

echo "Fixed unused imports"