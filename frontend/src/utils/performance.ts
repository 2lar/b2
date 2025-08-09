// Performance monitoring utilities for Brain2 frontend
import React, { useEffect, useState } from 'react';

interface PerformanceMetrics {
  operationTimes: Map<string, number>;
  renderTimes: Map<string, number>;
  memoryUsage: number;
  cacheHitRate: number;
  errorCount: number;
}

class PerformanceMonitor {
  private metrics: PerformanceMetrics;
  private timers: Map<string, number>;
  private cacheStats: { hits: number; misses: number };

  constructor() {
    this.metrics = {
      operationTimes: new Map(),
      renderTimes: new Map(),
      memoryUsage: 0,
      cacheHitRate: 0,
      errorCount: 0,
    };
    this.timers = new Map();
    this.cacheStats = { hits: 0, misses: 0 };
  }

  // Timer utilities
  startTimer(operationName: string): void {
    this.timers.set(operationName, performance.now());
  }

  endTimer(operationName: string): number {
    const startTime = this.timers.get(operationName);
    if (!startTime) {
      console.warn(`Timer ${operationName} was not started`);
      return 0;
    }
    
    const duration = performance.now() - startTime;
    this.timers.delete(operationName);
    this.metrics.operationTimes.set(operationName, duration);
    return duration;
  }

  // Cache performance tracking
  recordCacheHit(): void {
    this.cacheStats.hits++;
    this.updateCacheHitRate();
  }

  recordCacheMiss(): void {
    this.cacheStats.misses++;
    this.updateCacheHitRate();
  }

  private updateCacheHitRate(): void {
    const total = this.cacheStats.hits + this.cacheStats.misses;
    this.metrics.cacheHitRate = total > 0 ? this.cacheStats.hits / total : 0;
  }

  // Memory usage tracking
  updateMemoryUsage(): void {
    if ('memory' in performance) {
      this.metrics.memoryUsage = (performance as any).memory.usedJSHeapSize / (1024 * 1024); // MB
    }
  }

  // Error tracking
  recordError(): void {
    this.metrics.errorCount++;
  }

  // Get metrics
  getMetrics(): PerformanceMetrics {
    this.updateMemoryUsage();
    return { ...this.metrics };
  }

  // Get operation statistics
  getOperationStats(operationName: string): {
    count: number;
    averageTime: number;
    totalTime: number;
  } {
    const times = Array.from(this.metrics.operationTimes.entries())
      .filter(([name]) => name.includes(operationName))
      .map(([, time]) => time);

    return {
      count: times.length,
      averageTime: times.length > 0 ? times.reduce((a, b) => a + b, 0) / times.length : 0,
      totalTime: times.reduce((a, b) => a + b, 0),
    };
  }

  // Performance warning thresholds
  checkPerformanceWarnings(): string[] {
    const warnings: string[] = [];
    
    if (this.metrics.memoryUsage > 100) { // 100MB
      warnings.push(`High memory usage: ${this.metrics.memoryUsage.toFixed(1)}MB`);
    }
    
    if (this.metrics.cacheHitRate < 0.7) {
      warnings.push(`Low cache hit rate: ${(this.metrics.cacheHitRate * 100).toFixed(1)}%`);
    }
    
    // Check for slow operations
    this.metrics.operationTimes.forEach((time, operation) => {
      if (time > 2000) { // 2 seconds
        warnings.push(`Slow operation: ${operation} took ${time.toFixed(0)}ms`);
      }
    });
    
    return warnings;
  }

  // Reset metrics
  reset(): void {
    this.metrics = {
      operationTimes: new Map(),
      renderTimes: new Map(),
      memoryUsage: 0,
      cacheHitRate: 0,
      errorCount: 0,
    };
    this.cacheStats = { hits: 0, misses: 0 };
  }

  // Export metrics for analysis
  exportMetrics(): string {
    return JSON.stringify({
      timestamp: new Date().toISOString(),
      metrics: {
        ...this.metrics,
        operationTimes: Array.from(this.metrics.operationTimes.entries()),
        renderTimes: Array.from(this.metrics.renderTimes.entries()),
      },
      warnings: this.checkPerformanceWarnings(),
    }, null, 2);
  }
}

// Global performance monitor instance
export const performanceMonitor = new PerformanceMonitor();

// React hook for performance monitoring
export function usePerformanceMonitor() {
  const [metrics, setMetrics] = useState<PerformanceMetrics | null>(null);

  useEffect(() => {
    const interval = setInterval(() => {
      setMetrics(performanceMonitor.getMetrics());
    }, 5000); // Update every 5 seconds

    return () => clearInterval(interval);
  }, []);

  return {
    metrics,
    startTimer: performanceMonitor.startTimer.bind(performanceMonitor),
    endTimer: performanceMonitor.endTimer.bind(performanceMonitor),
    recordCacheHit: performanceMonitor.recordCacheHit.bind(performanceMonitor),
    recordCacheMiss: performanceMonitor.recordCacheMiss.bind(performanceMonitor),
    recordError: performanceMonitor.recordError.bind(performanceMonitor),
    getOperationStats: performanceMonitor.getOperationStats.bind(performanceMonitor),
    exportMetrics: performanceMonitor.exportMetrics.bind(performanceMonitor),
    warnings: performanceMonitor.checkPerformanceWarnings(),
  };
}

// Performance decorator for measuring function execution time
export function measurePerformance(operationName: string) {
  return function (target: any, propertyName: string, descriptor: PropertyDescriptor) {
    const method = descriptor.value;

    descriptor.value = function (...args: any[]) {
      performanceMonitor.startTimer(operationName);
      
      try {
        const result = method.apply(this, args);
        
        // Handle async functions
        if (result instanceof Promise) {
          return result.finally(() => {
            performanceMonitor.endTimer(operationName);
          });
        }
        
        performanceMonitor.endTimer(operationName);
        return result;
      } catch (error) {
        performanceMonitor.endTimer(operationName);
        performanceMonitor.recordError();
        throw error;
      }
    };

    return descriptor;
  };
}

// Utility to measure React component render time
export function withPerformanceTracking<P extends object>(
  WrappedComponent: React.ComponentType<P>,
  componentName: string
) {
  const PerformanceTrackedComponent = (props: P) => {
    useEffect(() => {
      performanceMonitor.startTimer(`render-${componentName}`);
      return () => {
        performanceMonitor.endTimer(`render-${componentName}`);
      };
    });

    return React.createElement(WrappedComponent, props);
  };

  PerformanceTrackedComponent.displayName = `withPerformanceTracking(${componentName})`;

  return React.memo(PerformanceTrackedComponent);
}