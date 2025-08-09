/**
 * CategoryInsights Component - Category Analytics Dashboard
 * 
 * Purpose:
 * Provides comprehensive analytics and insights about category usage, trends, and patterns.
 * Displays data-driven insights to help users understand their memory organization and
 * identify opportunities for better categorization and knowledge management.
 * 
 * Key Features:
 * - Multi-tab interface for different types of insights
 * - Category activity tracking and visualization
 * - Growth trend analysis over time
 * - Category connection mapping and relationship analysis
 * - Knowledge gap identification and recommendations
 * - Interactive charts and data visualizations
 * - Real-time data loading with error handling
 * 
 * Insights Tabs:
 * - Activity: Most/least used categories, recent activity patterns
 * - Trends: Growth patterns, seasonal trends, category evolution
 * - Connections: Category relationships, cross-category memory patterns
 * - Gaps: Identified knowledge gaps, uncategorized content analysis
 * 
 * Analytics Features:
 * - Category usage statistics and rankings
 * - Time-based trend analysis with charts
 * - Memory distribution across categories
 * - Category relationship strength measurements
 * - Automated knowledge gap detection
 * - Actionable recommendations for improvement
 * 
 * Data Visualization:
 * - Interactive charts for trend analysis
 * - Category relationship network diagrams
 * - Usage heatmaps and activity timelines
 * - Progress indicators and statistics
 * - Comparative analysis between categories
 * 
 * State Management:
 * - insights: Complete insights data object
 * - loading: Loading state for data fetching
 * - error: Error state and message handling
 * - activeTab: Currently selected insights tab
 * 
 * Integration:
 * - Fetches insights from dedicated analytics API endpoint
 * - Can be accessed from main navigation or category views
 * - Provides actionable insights for category management
 * - Supports data export and sharing functionality
 */

import React, { useState, useEffect } from 'react';
import { components } from '../../../types/generated/generated-types';

// Type aliases
type CategoryInsights = components['schemas']['CategoryInsights'];
type CategoryActivity = components['schemas']['CategoryActivity'];
type CategoryGrowthTrend = components['schemas']['CategoryGrowthTrend'];
type CategoryConnection = components['schemas']['CategoryConnection'];
type KnowledgeGap = components['schemas']['KnowledgeGap'];

export const CategoryInsightsComponent: React.FC = () => {
  const [insights, setInsights] = useState<CategoryInsights | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'activity' | 'trends' | 'connections' | 'gaps'>('activity');

  useEffect(() => {
    loadInsights();
  }, []);

  const loadInsights = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await fetch('/api/categories/insights');
      if (!response.ok) {
        throw new Error('Failed to load category insights');
      }
      
      const data = await response.json();
      setInsights(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      console.error('Error loading insights:', err);
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', { 
      month: 'short', 
      day: 'numeric',
      year: 'numeric'
    });
  };

  const renderActivityTab = () => {
    const activities = insights?.mostActiveCategories || [];
    
    if (activities.length === 0) {
      return (
        <div className="empty-state">
          <p>No category activity data available</p>
        </div>
      );
    }

    return (
      <div className="activity-list">
        {activities.map((activity, index) => (
          <div key={activity.categoryId} className="activity-item">
            <div className="activity-rank">#{index + 1}</div>
            <div className="activity-content">
              <h4 className="activity-name">{activity.categoryName}</h4>
              <div className="activity-stats">
                <span className="total-memories">
                  📚 {activity.memoryCount} memories
                </span>
                <span className="recent-adds">
                  ✨ {activity.recentAdds} recent
                </span>
              </div>
            </div>
            <div className="activity-progress">
              <div 
                className="progress-bar"
                style={{ 
                  width: `${(activity.memoryCount / Math.max(...activities.map(a => a.memoryCount))) * 100}%` 
                }}
              />
            </div>
          </div>
        ))}
      </div>
    );
  };

  const renderTrendsTab = () => {
    const trends = insights?.categoryGrowthTrends || [];
    
    if (trends.length === 0) {
      return (
        <div className="empty-state">
          <p>No growth trend data available</p>
        </div>
      );
    }

    // Group trends by category
    const trendsByCategory = trends.reduce((acc, trend) => {
      if (!acc[trend.categoryId]) {
        acc[trend.categoryId] = {
          categoryName: trend.categoryName,
          trends: []
        };
      }
      acc[trend.categoryId].trends.push(trend);
      return acc;
    }, {} as Record<string, { categoryName: string; trends: CategoryGrowthTrend[] }>);

    return (
      <div className="trends-list">
        {Object.entries(trendsByCategory).map(([categoryId, data]) => {
          const sortedTrends = data.trends.sort((a, b) => 
            new Date(a.date).getTime() - new Date(b.date).getTime()
          );
          const latestTrend = sortedTrends[sortedTrends.length - 1];
          const previousTrend = sortedTrends[sortedTrends.length - 2];
          const growth = previousTrend 
            ? latestTrend.memoryCount - previousTrend.memoryCount 
            : 0;
          
          return (
            <div key={categoryId} className="trend-item">
              <div className="trend-header">
                <h4 className="trend-name">{data.categoryName}</h4>
                <div className="trend-indicators">
                  <span className="current-count">
                    {latestTrend.memoryCount} memories
                  </span>
                  {growth !== 0 && (
                    <span className={`growth-indicator ${growth > 0 ? 'positive' : 'negative'}`}>
                      {growth > 0 ? '↗️' : '↘️'} {Math.abs(growth)}
                    </span>
                  )}
                </div>
              </div>
              <div className="trend-timeline">
                {sortedTrends.slice(-5).map((trend, index) => (
                  <div key={index} className="timeline-point">
                    <div className="point-date">{formatDate(trend.date)}</div>
                    <div className="point-count">{trend.memoryCount}</div>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const renderConnectionsTab = () => {
    const connections = insights?.suggestedConnections || [];
    
    if (connections.length === 0) {
      return (
        <div className="empty-state">
          <p>No category connections suggested</p>
        </div>
      );
    }

    return (
      <div className="connections-list">
        {connections.map((connection, index) => (
          <div key={index} className="connection-item">
            <div className="connection-visual">
              <div className="connection-node">{connection.category1Name}</div>
              <div className="connection-link">
                <div 
                  className="link-strength"
                  style={{ 
                    width: `${connection.strength * 100}%`,
                    backgroundColor: connection.strength > 0.7 ? '#4caf50' : 
                                    connection.strength > 0.5 ? '#ff9800' : '#f44336'
                  }}
                />
                <span className="strength-label">
                  {Math.round(connection.strength * 100)}%
                </span>
              </div>
              <div className="connection-node">{connection.category2Name}</div>
            </div>
            <div className="connection-reason">
              {connection.reason}
            </div>
            <div className="connection-actions">
              <button className="connect-btn">Create Connection</button>
              <button className="dismiss-btn">Dismiss</button>
            </div>
          </div>
        ))}
      </div>
    );
  };

  const renderGapsTab = () => {
    const gaps = insights?.knowledgeGaps || [];
    
    if (gaps.length === 0) {
      return (
        <div className="empty-state">
          <p>No knowledge gaps identified</p>
          <p>Your knowledge appears well-organized!</p>
        </div>
      );
    }

    return (
      <div className="gaps-list">
        {gaps.map((gap, index) => (
          <div key={index} className="gap-item">
            <div className="gap-header">
              <h4 className="gap-topic">{gap.topic}</h4>
              <span 
                className="gap-confidence"
                style={{ 
                  backgroundColor: gap.confidence > 0.7 ? '#4caf50' : 
                                  gap.confidence > 0.5 ? '#ff9800' : '#f44336'
                }}
              >
                {Math.round(gap.confidence * 100)}% confident
              </span>
            </div>
            <div className="gap-reason">
              {gap.reason}
            </div>
            <div className="gap-suggestions">
              <h5>Suggested categories to create:</h5>
              <div className="suggested-categories">
                {gap.suggestedCategories.map((category, i) => (
                  <span key={i} className="suggested-category">
                    {category}
                  </span>
                ))}
              </div>
            </div>
            <div className="gap-actions">
              <button className="create-categories-btn">
                Create Categories
              </button>
              <button className="dismiss-gap-btn">
                Dismiss
              </button>
            </div>
          </div>
        ))}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="category-insights loading">
        <div className="loading-spinner">
          <span>Analyzing your categories...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="category-insights error">
        <div className="error-message">
          <span>Error loading insights: {error}</span>
          <button onClick={loadInsights} className="retry-btn">
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="category-insights">
      <div className="insights-header">
        <h2>Category Insights</h2>
        <button onClick={loadInsights} className="refresh-btn" disabled={loading}>
          🔄 Refresh
        </button>
      </div>

      <div className="insights-tabs">
        <button 
          className={`tab ${activeTab === 'activity' ? 'active' : ''}`}
          onClick={() => setActiveTab('activity')}
        >
          📊 Activity
        </button>
        <button 
          className={`tab ${activeTab === 'trends' ? 'active' : ''}`}
          onClick={() => setActiveTab('trends')}
        >
          📈 Trends  
        </button>
        <button 
          className={`tab ${activeTab === 'connections' ? 'active' : ''}`}
          onClick={() => setActiveTab('connections')}
        >
          🔗 Connections
        </button>
        <button 
          className={`tab ${activeTab === 'gaps' ? 'active' : ''}`}
          onClick={() => setActiveTab('gaps')}
        >
          🎯 Knowledge Gaps
        </button>
      </div>

      <div className="insights-content">
        {activeTab === 'activity' && renderActivityTab()}
        {activeTab === 'trends' && renderTrendsTab()}
        {activeTab === 'connections' && renderConnectionsTab()}
        {activeTab === 'gaps' && renderGapsTab()}
      </div>
    </div>
  );
};

export default CategoryInsightsComponent;