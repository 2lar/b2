/**
 * ErrorBoundary Component - Graceful Error Handling and Recovery
 * 
 * Purpose:
 * Catches JavaScript errors anywhere in the child component tree and displays
 * a fallback UI instead of crashing the entire application. Provides error
 * recovery mechanisms and user-friendly error messages.
 * 
 * Key Features:
 * - Catches and handles React component errors
 * - Displays user-friendly fallback UI
 * - Provides retry functionality for recovery
 * - Error reporting to monitoring services
 * - Different fallback UI based on error context
 * - Prevents complete application crashes
 * 
 * Usage:
 * - Wrap critical sections of the app (Dashboard, Graph, Lists)
 * - Can provide custom fallback components
 * - Integrates with error monitoring services
 * - Supports different error severity levels
 */

import React, { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
    /** Child components to protect with error boundary */
    children: ReactNode;
    /** Custom fallback component to render on error */
    fallback?: (error: Error, retry: () => void) => ReactNode;
    /** Callback function called when an error occurs */
    onError?: (error: Error, errorInfo: ErrorInfo) => void;
    /** Name/identifier for this error boundary */
    name?: string;
    /** Whether to show detailed error information in development */
    showErrorDetails?: boolean;
}

interface State {
    hasError: boolean;
    error: Error | null;
    errorInfo: ErrorInfo | null;
}

export class ErrorBoundary extends Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = { 
            hasError: false, 
            error: null,
            errorInfo: null
        };
    }
    
    static getDerivedStateFromError(error: Error): Partial<State> {
        // Update state so the next render will show the fallback UI
        return { 
            hasError: true, 
            error 
        };
    }
    
    componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        console.error(`Error caught by boundary ${this.props.name || 'unnamed'}:`, error, errorInfo);
        
        this.setState({
            errorInfo
        });
        
        // Call custom error handler if provided
        this.props.onError?.(error, errorInfo);
        
        // Send to error tracking service in production
        if (process.env.NODE_ENV === 'production') {
            this.reportError(error, errorInfo);
        }
    }
    
    private reportError = (error: Error, errorInfo: ErrorInfo) => {
        // Integration with error tracking services
        try {
            // Example: Sentry integration
            if (typeof window !== 'undefined' && (window as any).Sentry) {
                (window as any).Sentry.captureException(error, {
                    contexts: { 
                        react: { 
                            componentStack: errorInfo.componentStack 
                        },
                        boundary: {
                            name: this.props.name || 'unnamed'
                        }
                    },
                });
            }
            
            // Example: Custom analytics/monitoring
            if (typeof window !== 'undefined' && (window as any).analytics) {
                (window as any).analytics.track('Error Boundary Triggered', {
                    error: error.message,
                    stack: error.stack,
                    componentStack: errorInfo.componentStack,
                    boundaryName: this.props.name
                });
            }
        } catch (reportingError) {
            console.error('Failed to report error:', reportingError);
        }
    };
    
    retry = () => {
        this.setState({ 
            hasError: false, 
            error: null,
            errorInfo: null
        });
    };
    
    render() {
        if (this.state.hasError && this.state.error) {
            // Custom fallback component
            if (this.props.fallback) {
                return this.props.fallback(this.state.error, this.retry);
            }
            
            // Default fallback UI
            return (
                <DefaultErrorFallback 
                    error={this.state.error} 
                    errorInfo={this.state.errorInfo}
                    retry={this.retry}
                    boundaryName={this.props.name}
                    showErrorDetails={this.props.showErrorDetails}
                />
            );
        }
        
        return this.props.children;
    }
}

interface FallbackProps {
    error: Error;
    errorInfo: ErrorInfo | null;
    retry: () => void;
    boundaryName?: string;
    showErrorDetails?: boolean;
}

const DefaultErrorFallback: React.FC<FallbackProps> = ({ 
    error, 
    errorInfo, 
    retry, 
    boundaryName,
    showErrorDetails = process.env.NODE_ENV === 'development'
}) => {
    const handleReportIssue = () => {
        const issueBody = encodeURIComponent(`
**Error Description:**
Something went wrong in the ${boundaryName || 'application'}.

**Error Message:**
${error.message}

**Steps to Reproduce:**
1. [Please describe what you were doing when this error occurred]

**Error Details:**
\`\`\`
${error.stack}
\`\`\`

**Component Stack:**
\`\`\`
${errorInfo?.componentStack}
\`\`\`
        `);
        
        window.open(
            `https://github.com/your-repo/issues/new?title=Error%20in%20${boundaryName || 'Application'}&body=${issueBody}`,
            '_blank'
        );
    };
    
    return (
        <div className="error-boundary-fallback">
            <div className="error-content">
                <div className="error-icon">⚠️</div>
                <h2>Something went wrong</h2>
                <p className="error-message">
                    {boundaryName ? `An error occurred in the ${boundaryName} section.` : 'An unexpected error occurred.'}
                </p>
                
                <div className="error-actions">
                    <button 
                        className="primary-btn retry-btn"
                        onClick={retry}
                    >
                        🔄 Try Again
                    </button>
                    <button 
                        className="secondary-btn refresh-btn"
                        onClick={() => window.location.reload()}
                    >
                        🔄 Refresh Page
                    </button>
                    <button 
                        className="secondary-btn report-btn"
                        onClick={handleReportIssue}
                    >
                        📝 Report Issue
                    </button>
                </div>
                
                {showErrorDetails && (
                    <details className="error-details">
                        <summary>Technical Details</summary>
                        <div className="error-stack">
                            <h4>Error:</h4>
                            <pre>{error.message}</pre>
                            <h4>Stack Trace:</h4>
                            <pre>{error.stack}</pre>
                            {errorInfo && (
                                <>
                                    <h4>Component Stack:</h4>
                                    <pre>{errorInfo.componentStack}</pre>
                                </>
                            )}
                        </div>
                    </details>
                )}
            </div>
            
            <style>{`
                .error-boundary-fallback {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    min-height: 400px;
                    padding: 2rem;
                    background: #fafafa;
                    border: 1px solid #e0e0e0;
                    border-radius: 8px;
                    margin: 1rem;
                }
                
                .error-content {
                    text-align: center;
                    max-width: 500px;
                }
                
                .error-icon {
                    font-size: 3rem;
                    margin-bottom: 1rem;
                }
                
                .error-message {
                    color: #666;
                    margin-bottom: 2rem;
                    line-height: 1.5;
                }
                
                .error-actions {
                    display: flex;
                    gap: 1rem;
                    justify-content: center;
                    flex-wrap: wrap;
                    margin-bottom: 2rem;
                }
                
                .retry-btn, .refresh-btn, .report-btn {
                    padding: 0.75rem 1.5rem;
                    border: none;
                    border-radius: 6px;
                    cursor: pointer;
                    font-size: 0.875rem;
                    font-weight: 500;
                    transition: all 0.2s;
                }
                
                .retry-btn {
                    background: #3b82f6;
                    color: white;
                }
                
                .retry-btn:hover {
                    background: #2563eb;
                }
                
                .refresh-btn, .report-btn {
                    background: #f3f4f6;
                    color: #374151;
                    border: 1px solid #d1d5db;
                }
                
                .refresh-btn:hover, .report-btn:hover {
                    background: #e5e7eb;
                }
                
                .error-details {
                    text-align: left;
                    margin-top: 2rem;
                    padding: 1rem;
                    background: #f8f9fa;
                    border: 1px solid #e9ecef;
                    border-radius: 4px;
                }
                
                .error-stack {
                    font-family: 'Monaco', 'Menlo', monospace;
                    font-size: 0.75rem;
                    max-height: 300px;
                    overflow-y: auto;
                }
                
                .error-stack h4 {
                    margin: 1rem 0 0.5rem 0;
                    color: #495057;
                }
                
                .error-stack pre {
                    background: #212529;
                    color: #f8f9fa;
                    padding: 0.75rem;
                    border-radius: 4px;
                    overflow-x: auto;
                    white-space: pre-wrap;
                    word-break: break-word;
                }
            `}</style>
        </div>
    );
};

// Higher-order component for easy wrapping
export const withErrorBoundary = <P extends object>(
    Component: React.ComponentType<P>,
    errorBoundaryProps?: Omit<Props, 'children'>
) => {
    const WrappedComponent = (props: P) => (
        <ErrorBoundary {...errorBoundaryProps}>
            <Component {...props} />
        </ErrorBoundary>
    );
    
    WrappedComponent.displayName = `withErrorBoundary(${Component.displayName || Component.name})`;
    
    return WrappedComponent;
};

export default ErrorBoundary;