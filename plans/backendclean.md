# Backend Upgrade Plan: Full Clean Architecture Adoption

## Goal
To refactor the backend to fully embrace Clean Architecture principles, enhancing testability, maintainability, flexibility, and scalability by strictly adhering to the Dependency Rule.

## Current State Assessment
The backend already has a good foundation with `internal/domain/`, `internal/service/`, and `internal/repository/` (interfaces). The primary missing piece is an explicit Infrastructure layer for concrete implementations of repository interfaces.

## Architectural Layers & Their Mapping

*   **Entities Layer:** `backend/internal/domain/`
*   **Use Cases Layer:** `backend/internal/service/`
*   **Gateways/Repositories Layer (Interfaces):** `backend/internal/repository/`
*   **External/Delivery Layer:** `backend/cmd/`
*   **Infrastructure Layer (NEW):** `backend/infrastructure/`
*   **Shared Utilities:** `backend/pkg/`

## Implementation Plan

### Phase 1: Preparation & Setup

1.  **Create the Infrastructure Directory:**
    *   Create a new directory: `backend/infrastructure/`. This will house all concrete implementations of interfaces defined in `internal/repository/`.
    *   Example: `mkdir -p backend/infrastructure`

2.  **Review Existing Repository Interfaces:**
    *   Examine all interfaces currently defined in `backend/internal/repository/`. Ensure they are technology-agnostic (e.g., `SaveNode(node domain.Node)` instead of `SavePostgresNode(node *PostgresNode)`).
    *   Identify all methods that interact with external systems (database, external APIs, file system).

3.  **Identify Current Database Interaction Points:**
    *   Locate all code that directly interacts with the database (e.g., `gorm.DB` instances, raw SQL queries, `ent` client calls). These are the candidates for moving to the new `infrastructure` layer.

### Phase 2: Infrastructure Layer Implementation

This phase involves creating the concrete implementations for each repository interface.

1.  **For each interface in `backend/internal/repository/` (e.g., `NodeRepository`, `CategoryRepository`):**
    *   **Create a Concrete Implementation File:**
        *   Inside `backend/infrastructure/`, create a new file for the concrete implementation.
        *   **Example:** For `NodeRepository` interface, create `backend/infrastructure/postgres_node_repository.go`.
    *   **Define the Concrete Struct:**
        *   Define a struct that will hold the database client (e.g., `gorm.DB` instance, `ent.Client`).
        *   **Example (`postgres_node_repository.go`):**
            ```go
            package infrastructure

            import (
                "gorm.io/gorm"
                "your_project/backend/internal/domain" // Assuming domain is here
                "your_project/backend/internal/repository" // Assuming repository is here
            )

            type postgresNodeRepository struct {
                db *gorm.DB
            }

            // NewPostgresNodeRepository creates a new instance of the concrete repository
            func NewPostgresNodeRepository(db *gorm.DB) repository.NodeRepository {
                return &postgresNodeRepository{db: db}
            }

            // Implement the NodeRepository interface methods
            func (r *postgresNodeRepository) Save(node *domain.Node) error {
                // ... GORM/database specific logic here ...
                return r.db.Save(node).Error
            }

            func (r *postgresNodeRepository) FindByID(id string) (*domain.Node, error) {
                // ... GORM/database specific logic here ...
                var node domain.Node
                if err := r.db.First(&node, "id = ?", id).Error; err != nil {
                    return nil, err
                }
                return &node, nil
            }
            // ... implement other methods like Delete, List, etc.
            ```
    *   **Move Database Logic:**
        *   Cut and paste the database interaction code from the current `internal/service/` or `cmd/` files into the corresponding methods of the new concrete repository implementation.
        *   Ensure all database-specific imports (e.g., `gorm.io/gorm`, `github.com/go-sql-driver/mysql`) are now only in the `backend/infrastructure/` files.

### Phase 3: Use Case Layer (`backend/internal/service/`) Refinement

This phase focuses on making the service layer dependent only on interfaces.

1.  **Update Service Structs for Dependency Injection:**
    *   Modify the constructor functions and structs in `backend/internal/service/` to accept repository *interfaces* as dependencies, typically via constructor injection.
    *   **Example (`internal/service/node_service.go`):**
        ```go
        package service

        import (
            "your_project/backend/internal/domain"
            "your_project/backend/internal/repository" // Import the interface
        )

        type NodeService struct {
            nodeRepo repository.NodeRepository // Use the interface type
            // ... other dependencies
        }

        // NewNodeService accepts the interface
        func NewNodeService(nodeRepo repository.NodeRepository) *NodeService {
            return &NodeService{nodeRepo: nodeRepo}
        }

        func (s *NodeService) CreateNode(content string, tags []string) (*domain.Node, error) {
            node := domain.NewNode(content, tags)
            // Call the interface method
            if err := s.nodeRepo.Save(node); err != nil {
                return nil, err
            }
            return node, nil
        }
        // ... update other methods
        ```
2.  **Remove Direct Database Imports:**
    *   Ensure that no files in `backend/internal/service/` directly import database drivers or ORM libraries. All such interactions must now go through the `repository` interfaces.

### Phase 4: External/Delivery Layer (`backend/cmd/`) Updates

This phase connects the top layer to the newly structured layers.

1.  **Instantiate Concrete Implementations:**
    *   In your `backend/cmd/main/main.go` (or other `cmd` entry points), instantiate the concrete `backend/infrastructure/` implementations.
    *   **Example (`backend/cmd/main/main.go`):**
        ```go
        package main

        import (
            "gorm.io/gorm"
            "your_project/backend/infrastructure" // Import the infrastructure package
            "your_project/backend/internal/service" // Import the service package
            // ... other imports
        )

        func main() {
            // 1. Initialize database connection (this stays in the outermost layer)
            db, err := gorm.Open( /* ... your database config ... */ )
            if err != nil {
                // handle error
            }

            // 2. Instantiate concrete repository implementations
            nodeRepo := infrastructure.NewPostgresNodeRepository(db)
            categoryRepo := infrastructure.NewPostgresCategoryRepository(db) // Assuming you have one

            // 3. Instantiate services, injecting the repository interfaces
            nodeService := service.NewNodeService(nodeRepo)
            categoryService := service.NewCategoryService(categoryRepo) // Assuming you have one

            // 4. Set up HTTP handlers/WebSocket handlers, injecting services
            // e.g., http.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
            //     // Call nodeService methods
            // })
            // ...
        }
        ```
2.  **Ensure `cmd` Only Calls Services:**
    *   Verify that your HTTP handlers, WebSocket handlers, or CLI commands in `backend/cmd/` only interact with the `backend/internal/service/` layer. They should not directly call repository methods or interact with the database.

### Phase 5: Testing

1.  **Unit Tests for Use Cases (`backend/internal/service/`):**
    *   Create mock implementations of the `backend/internal/repository/` interfaces.
    *   Write unit tests for your services, injecting these mocks. This allows you to test business logic without needing a running database.
2.  **Integration Tests for Infrastructure (`backend/infrastructure/`):
    *   Write integration tests for your concrete repository implementations. These tests will require a real (or test) database instance to verify correct data persistence.
3.  **End-to-End Tests:**
    *   Maintain or create end-to-end tests that test the entire application flow, from the `cmd` layer down to the database.

### Phase 6: Cleanup & Verification

1.  **Remove Redundant Code:** Delete any old database interaction code that was moved to `backend/infrastructure/`.
2.  **Run All Tests:** Ensure all unit, integration, and end-to-end tests pass.
3.  **Build and Run:** Build the backend and verify that it functions correctly.
4.  **Code Review:** Conduct a code review to ensure adherence to the new architectural boundaries.

This detailed plan should provide a clear roadmap for implementing Clean Architecture in your backend.
