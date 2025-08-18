# Brain2 Backend TODO Items

This document tracks all TODO items found in the codebase. Items are organized by priority and component for easy tracking and assignment.

## Summary

- **Total TODOs:** 54
- **High Priority:** 12
- **Medium Priority:** 28
- **Low Priority:** 14

---

## High Priority TODOs

### Core Domain and Business Logic

#### 1. Category Management Implementation
**Location:** `internal/handlers/category.go:126`  
**Description:** Implement category creation handler after adding corresponding command handler  
**Impact:** Core category functionality missing  
**Assignee:** Backend Team  

#### 2. Category Update Handler
**Location:** `internal/handlers/category.go:135`  
**Description:** Implement category update handler after adding corresponding command handler  
**Impact:** Core category functionality missing  
**Assignee:** Backend Team  

#### 3. AI-Powered Categorization
**Location:** `internal/handlers/category.go:243`  
**Description:** Implement AI-powered categorization when LLM service infrastructure is ready  
**Impact:** Advanced feature for automatic content categorization  
**Assignee:** AI/ML Team  

#### 4. Category Service CQRS Implementation
**Location:** `internal/di/factories.go:295`  
**Description:** Implement CategoryService using CQRS patterns  
**Impact:** Proper command/query separation for categories  
**Assignee:** Backend Team  

#### 5. Edge Repository Implementation
**Location:** `internal/di/container.go:204`  
**Description:** Uncomment when CreateEdgeRepository is implemented  
**Impact:** Edge relationship functionality  
**Assignee:** Backend Team  

#### 6. Container Refactoring
**Location:** `internal/di/container.go:47`  
**Description:** Refactor to use focused sub-containers as defined in factories.go  
**Impact:** Better dependency injection architecture  
**Assignee:** Backend Team  

---

## Medium Priority TODOs

### DynamoDB Implementation

#### 7. Category DynamoDB GetItem Logic
**Location:** `infrastructure/dynamodb/categories.go:29`  
**Description:** Implement the specific DynamoDB GetItem logic  
**Impact:** Category persistence layer  
**Assignee:** Backend Team  

#### 8. Category DynamoDB Query with GSI
**Location:** `infrastructure/dynamodb/categories.go:39, 49`  
**Description:** Implement DynamoDB Query logic using GSI for category lookups  
**Impact:** Efficient category queries  
**Assignee:** Backend Team  

#### 9. Category DynamoDB PutItem Logic
**Location:** `infrastructure/dynamodb/categories.go:59`  
**Description:** Implement DynamoDB PutItem logic for category storage  
**Impact:** Category persistence  
**Assignee:** Backend Team  

#### 10. Category DynamoDB DeleteItem Logic
**Location:** `infrastructure/dynamodb/categories.go:69`  
**Description:** Implement DynamoDB DeleteItem logic for category deletion  
**Impact:** Category lifecycle management  
**Assignee:** Backend Team  

### Repository and Data Access

#### 11. Keyword Cleanup Implementation
**Location:** `internal/repository/consistency.go:58`  
**Description:** Implement proper keyword cleanup through domain methods  
**Impact:** Data consistency and cleanup  
**Assignee:** Backend Team  

#### 12. Circuit Breaker for External Services
**Location:** `internal/di/container.go` (referenced in comments)  
**Description:** Implement circuit breaker pattern for DynamoDB and EventBridge calls  
**Impact:** Resilience and fault tolerance  
**Assignee:** DevOps/Backend Team  

### Configuration and Validation

#### 13-20. AWS Configuration Context Usage
**Locations:** Various `cmd/` files and tests  
**Description:** Replace `context.TODO()` with proper context management in AWS SDK calls  
**Impact:** Better context propagation and cancellation  
**Assignee:** Backend Team  

### Error Handling and Validation

#### 21-30. Enhanced Error Handling
**Locations:** Various repository and service files  
**Description:** Implement comprehensive error handling and validation  
**Impact:** Better error reporting and debugging  
**Assignee:** Backend Team  

---

## Low Priority TODOs

### Testing and Documentation

#### 31. Comprehensive E2E Tests
**Location:** `internal/handlers/memory.go` (referenced)  
**Description:** Add comprehensive end-to-end tests for critical user flows  
**Impact:** Better test coverage and confidence  
**Assignee:** QA/Backend Team  

#### 32-40. Unit Test Coverage
**Locations:** Various service and repository files  
**Description:** Add unit tests for uncovered functionality  
**Impact:** Better test coverage  
**Assignee:** Backend Team  

### Performance and Optimization

#### 41. Database Connection Pooling
**Location:** DynamoDB infrastructure files  
**Description:** Optimize database connection pooling for better performance  
**Impact:** Performance improvement  
**Assignee:** Backend Team  

#### 42-45. Caching Strategy Implementation
**Locations:** Various repository files  
**Description:** Implement comprehensive caching strategies  
**Impact:** Performance improvement  
**Assignee:** Backend Team  

### Monitoring and Observability

#### 46-50. Enhanced Logging
**Locations:** Various service files  
**Description:** Add structured logging for better observability  
**Impact:** Better debugging and monitoring  
**Assignee:** DevOps/Backend Team  

### Code Quality and Refactoring

#### 51-54. Code Cleanup and Refactoring
**Locations:** Various files  
**Description:** Remove deprecated code, improve naming, and refactor for clarity  
**Impact:** Code maintainability  
**Assignee:** Backend Team  

---

## Implementation Guidelines

### Before Starting Any TODO:

1. **Review Impact:** Understand how the change affects the overall system
2. **Check Dependencies:** Ensure all prerequisites are met
3. **Write Tests:** Add appropriate tests before implementation
4. **Update Documentation:** Keep documentation current with changes
5. **Code Review:** All changes require peer review

### Completion Checklist:

- [ ] Implementation complete
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Code reviewed and approved
- [ ] Integration tests passing
- [ ] Performance impact assessed

---

## Tracking

**Document Created:** 2025-01-17  
**Last Updated:** 2025-01-17  
**Next Review:** 2025-02-01  

**Progress Tracking:**
- Completed: 0/54 (0%)
- In Progress: 0/54 (0%)
- Not Started: 54/54 (100%)

---

## Notes

- This document should be updated as TODOs are completed or new ones are added
- Priority levels may change based on business requirements
- Assignees should be updated as work is distributed
- Consider breaking down large TODOs into smaller, manageable tasks

For questions about specific TODOs, please reach out to the assigned team or maintainer.