// Package config provides comprehensive configuration management for the Brain2 application.
//
// This package demonstrates enterprise-grade configuration management with:
//   - Multiple configuration sources (files, environment variables, secrets)
//   - Environment-specific configurations
//   - Configuration validation with detailed error messages
//   - Type safety and documentation
//   - Hot reloading for development
//   - Secrets management integration
//
// # Architecture
//
// The configuration system follows these design principles:
//   - Configuration as Code: All configuration is versioned and documented
//   - Fail Fast: Invalid configuration causes immediate startup failure
//   - Secure by Default: Production requires explicit security settings
//   - Environment Parity: Similar configuration structure across environments
//   - Observability: Configuration sources and values are logged (excluding secrets)
//
// # Configuration Hierarchy
//
// Configuration is loaded from multiple sources in priority order (highest wins):
//   1. Default values in code (lowest priority)
//   2. base.yaml - Common configuration for all environments
//   3. {environment}.yaml - Environment-specific overrides
//   4. local.yaml - Local developer overrides (gitignored)
//   5. Environment variables (highest priority)
//
// # File Structure
//
//	config/
//	├── base.yaml           # Base configuration for all environments
//	├── development.yaml    # Development environment overrides
//	├── staging.yaml        # Staging environment overrides
//	├── production.yaml     # Production environment overrides
//	├── local.yaml          # Local overrides (gitignored)
//	└── example.yaml        # Documented example with all options
//
// # Usage Examples
//
// Basic usage with environment variable loading:
//
//	cfg := config.LoadConfig()
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal("Invalid configuration:", err)
//	}
//
// Advanced usage with file loading:
//
//	loader := config.NewLoader("config", config.Production)
//	cfg, err := loader.Load()
//	if err != nil {
//	    log.Fatal("Failed to load configuration:", err)
//	}
//	fmt.Printf("Configuration loaded from: %v\n", cfg.LoadedFrom)
//
// Using configuration in your application:
//
//	server := &http.Server{
//	    Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
//	    ReadTimeout:  cfg.Server.ReadTimeout,
//	    WriteTimeout: cfg.Server.WriteTimeout,
//	}
//
// # Environment Variables
//
// All configuration values can be overridden via environment variables.
// The naming convention is SECTION_KEY (uppercase, underscore-separated).
//
// Examples:
//   - SERVER_PORT=8080
//   - DATABASE_TABLE_NAME=brain2-prod
//   - FEATURES_ENABLE_METRICS=true
//   - AWS_REGION=us-west-2
//
// # Secrets Management
//
// Sensitive values should NEVER be committed to version control.
// Use environment variables or AWS Secrets Manager for production.
//
// In configuration files, use placeholders:
//
//	security:
//	  jwt_secret: "${JWT_SECRET}"
//	cache:
//	  redis:
//	    password: "${REDIS_PASSWORD}"
//
// # Validation
//
// Configuration validation happens at multiple levels:
//   1. Struct tags using go-playground/validator
//   2. Custom business rule validation
//   3. Environment-specific validation
//
// Example struct tags:
//
//	type Server struct {
//	    Port int `validate:"required,min=1,max=65535"`
//	    Host string `validate:"required,hostname|ip"`
//	}
//
// # Feature Flags
//
// Feature flags enable gradual rollout and A/B testing:
//
//	if cfg.Features.EnableAIProcessing {
//	    // New AI feature code
//	}
//
// # Environment-Specific Behavior
//
// The configuration system enforces environment-specific rules:
//
// Development:
//   - Debug logging enabled
//   - Authentication optional
//   - Relaxed security settings
//
// Staging:
//   - Production-like configuration
//   - Metrics and tracing enabled
//   - Moderate capacity settings
//
// Production:
//   - Metrics required
//   - Authentication required
//   - Strict security settings
//   - No debug endpoints
//
// # Best Practices
//
//  1. Always validate configuration on startup
//  2. Use structured logging for configuration values (exclude secrets)
//  3. Document all configuration options in example.yaml
//  4. Use feature flags for gradual rollout
//  5. Keep environment configurations similar to avoid surprises
//  6. Use smallest acceptable values for limits and timeouts
//  7. Enable all security features in production
//
// # Configuration Hot Reload (Development Only)
//
// In development, configuration can be reloaded without restart:
//
//	watcher := config.NewWatcher(cfg, "config")
//	watcher.OnChange(func(newCfg *config.Config) {
//	    log.Info("Configuration reloaded")
//	    // Update application with new configuration
//	})
//	watcher.Start()
//	defer watcher.Stop()
//
// # Testing
//
// For testing, use in-memory configuration:
//
//	cfg := &config.Config{
//	    Environment: config.Development,
//	    Server: config.Server{Port: 8080},
//	    // ... other required fields
//	}
//
// Or load test-specific configuration:
//
//	loader := config.NewLoader("testdata/config", config.Development)
//	cfg, _ := loader.Load()
//
// # Monitoring
//
// Configuration loading is instrumented with:
//   - Load time metrics
//   - Validation error counts
//   - Configuration source tracking
//   - Hot reload events (development)
//
// # Security Considerations
//
//  1. Never log sensitive configuration values
//  2. Use environment variables or secrets management for credentials
//  3. Validate all external configuration input
//  4. Use principle of least privilege for defaults
//  5. Require explicit opt-in for dangerous features
//  6. Audit configuration changes in production
//
// # Migration Guide
//
// When updating configuration schema:
//  1. Increment version in base.yaml
//  2. Add migration logic in Validate() if needed
//  3. Update example.yaml with new options
//  4. Document breaking changes in CHANGELOG
//  5. Provide backward compatibility when possible
//
// # Common Issues and Solutions
//
// Issue: Configuration validation fails on startup
// Solution: Check logs for specific validation errors, ensure all required fields are set
//
// Issue: Environment variables not overriding file configuration
// Solution: Verify variable names match convention (SECTION_KEY), check for typos
//
// Issue: Secrets appearing in logs
// Solution: Review logging configuration, ensure sensitive fields are marked with `log:"-"` tag
//
// Issue: Configuration changes not taking effect
// Solution: Restart application (hot reload only works in development with watcher)
//
// Issue: Different behavior between environments
// Solution: Compare environment configurations, ensure feature flags are consistent
package config