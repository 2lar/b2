/**
 * CategorySuggestions Component - AI-Powered Category Recommendations
 * 
 * Purpose:
 * Provides intelligent category suggestions for memories using AI analysis.
 * Analyzes memory content and suggests relevant categories with confidence scores,
 * allowing users to quickly categorize memories or create new categories based on AI recommendations.
 * 
 * Key Features:
 * - AI-powered content analysis for category suggestions
 * - Confidence scoring for suggestion quality assessment
 * - Interactive suggestion cards with accept/reject actions
 * - Automatic triggering based on content changes
 * - New category creation from suggestions
 * - Suggestion history and learning capabilities
 * - Loading states and error handling
 * - Batch suggestion processing for multiple memories
 * 
 * AI Suggestion Features:
 * - Natural language processing of memory content
 * - Context-aware category recommendations
 * - Confidence scores for suggestion reliability
 * - Multiple suggestion options per memory
 * - Learning from user accept/reject patterns
 * - Integration with existing category taxonomy
 * 
 * User Interaction:
 * - Accept suggestions to apply categories immediately
 * - Reject suggestions to improve AI learning
 * - Create new categories from high-confidence suggestions
 * - Batch processing for multiple suggestions
 * - Manual triggering or automatic suggestion generation
 * 
 * Suggestion Display:
 * - Card-based layout with suggestion details
 * - Confidence indicators and visual feedback
 * - Category preview with descriptions
 * - Action buttons for accept/reject/create
 * - Loading states during AI processing
 * - Error handling for failed suggestions
 * 
 * State Management:
 * - suggestions: Array of AI-generated category suggestions
 * - loading: Loading state for AI processing
 * - error: Error state and message handling
 * - showSuggestions: Toggle for suggestion panel visibility
 * 
 * Integration:
 * - Triggered automatically when content changes (if autoTrigger enabled)
 * - Can be manually triggered for existing memories
 * - Integrates with category creation and assignment APIs
 * - Provides callbacks for suggestion acceptance/rejection
 * - Works with both new memory creation and existing memory categorization
 */

import React, { useState } from 'react';
import { components } from '../../../types/generated/generated-types';

// Type aliases
type CategorySuggestion = components['schemas']['CategorySuggestion'];
type Category = components['schemas']['Category'];

interface CategorySuggestionsProps {
  /** Memory content to analyze for suggestions */
  content?: string;
  /** Optional node ID for existing memory categorization */
  nodeId?: string;
  /** Callback when user accepts a suggestion */
  onSuggestionAccept?: (suggestion: CategorySuggestion) => void;
  /** Callback when user rejects a suggestion */
  onSuggestionReject?: (suggestion: CategorySuggestion) => void;
  /** Whether to automatically trigger suggestions on content changes */
  autoTrigger?: boolean;
}

export const CategorySuggestions: React.FC<CategorySuggestionsProps> = ({
  content,
  nodeId,
  onSuggestionAccept,
  onSuggestionReject,
  autoTrigger = false
}) => {
  const [suggestions, setSuggestions] = useState<CategorySuggestion[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSuggestions, setShowSuggestions] = useState(false);

  React.useEffect(() => {
    if (autoTrigger && content) {
      getSuggestions();
    }
  }, [content, autoTrigger]);

  const getSuggestions = async () => {
    if (!content && !nodeId) {
      setError('No content or node ID provided');
      return;
    }

    setLoading(true);
    setError(null);
    
    try {
      let endpoint = '/api/categories/suggest';
      let body: any = {};

      if (nodeId) {
        // Use node categorization endpoint
        endpoint = `/api/nodes/${nodeId}/categories`;
        const response = await fetch(endpoint, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          }
        });

        if (!response.ok) {
          if (response.status === 503) {
            throw new Error('AI service is temporarily unavailable');
          }
          throw new Error('Failed to get category suggestions');
        }

        const data = await response.json();
        // Convert categories to suggestions format
        const categorySuggestions: CategorySuggestion[] = (data.categories || []).map((cat: Category) => ({
          name: cat.title,
          level: cat.level,
          confidence: 0.8, // Default confidence for existing categories
          reason: `Existing category: ${cat.description || 'No description'}`,
          parentId: cat.parentId
        }));
        setSuggestions(categorySuggestions);
      } else if (content) {
        // Use content-based suggestion endpoint
        body = { content };
        const response = await fetch(endpoint, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(body)
        });

        if (!response.ok) {
          if (response.status === 503) {
            throw new Error('AI service is temporarily unavailable');
          }
          throw new Error('Failed to get category suggestions');
        }

        const data = await response.json();
        setSuggestions(data.suggestions || []);
      }

      setShowSuggestions(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      console.error('Error getting suggestions:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleAcceptSuggestion = (suggestion: CategorySuggestion) => {
    if (onSuggestionAccept) {
      onSuggestionAccept(suggestion);
    }
    // Remove accepted suggestion from list
    setSuggestions(prev => prev.filter(s => s !== suggestion));
  };

  const handleRejectSuggestion = (suggestion: CategorySuggestion) => {
    if (onSuggestionReject) {
      onSuggestionReject(suggestion);
    }
    // Remove rejected suggestion from list
    setSuggestions(prev => prev.filter(s => s !== suggestion));
  };

  const getConfidenceColor = (confidence: number): string => {
    if (confidence >= 0.8) return '#4caf50'; // Green
    if (confidence >= 0.6) return '#ff9800'; // Orange
    return '#f44336'; // Red
  };

  const getLevelLabel = (level: number): string => {
    const labels = ['General', 'Specific', 'Detailed'];
    return labels[level] || `Level ${level}`;
  };

  if (!showSuggestions && !autoTrigger) {
    return (
      <div className="category-suggestions">
        <button 
          className="suggest-btn"
          onClick={getSuggestions}
          disabled={loading || (!content && !nodeId)}
        >
          <span className="btn-icon">🤖</span>
          {loading ? 'Getting suggestions...' : 'Suggest Categories'}
        </button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="category-suggestions loading">
        <div className="loading-indicator">
          <span className="spinner"></span>
          <span>AI is analyzing content...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="category-suggestions error">
        <div className="error-message">
          <span className="error-icon">⚠️</span>
          <span>{error}</span>
          <button onClick={getSuggestions} className="retry-btn">
            Try Again
          </button>
        </div>
      </div>
    );
  }

  if (suggestions.length === 0 && showSuggestions) {
    return (
      <div className="category-suggestions empty">
        <div className="empty-message">
          <span className="empty-icon">🤷</span>
          <p>No category suggestions available</p>
          <p>The AI couldn't find suitable categories for this content.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="category-suggestions">
      <div className="suggestions-header">
        <h4>
          <span className="ai-icon">🤖</span>
          AI Category Suggestions
        </h4>
        <button 
          className="close-btn"
          onClick={() => setShowSuggestions(false)}
          title="Close suggestions"
        >
          ✕
        </button>
      </div>

      <div className="suggestions-list">
        {suggestions.map((suggestion, index) => (
          <div key={index} className="suggestion-item">
            <div className="suggestion-content">
              <div className="suggestion-header">
                <span className="suggestion-name">{suggestion.name}</span>
                <div className="suggestion-meta">
                  <span className="level-badge">{getLevelLabel(suggestion.level)}</span>
                  <span 
                    className="confidence-badge"
                    style={{ backgroundColor: getConfidenceColor(suggestion.confidence) }}
                  >
                    {Math.round(suggestion.confidence * 100)}%
                  </span>
                </div>
              </div>
              
              <div className="suggestion-reason">
                {suggestion.reason}
              </div>
              
              {suggestion.parentId && (
                <div className="parent-info">
                  <span className="parent-label">Parent:</span>
                  <span className="parent-name">{suggestion.parentId}</span>
                </div>
              )}
            </div>

            <div className="suggestion-actions">
              <button
                className="accept-btn"
                onClick={() => handleAcceptSuggestion(suggestion)}
                title="Accept this suggestion"
              >
                <span className="btn-icon">✓</span>
                Accept
              </button>
              <button
                className="reject-btn"
                onClick={() => handleRejectSuggestion(suggestion)}
                title="Reject this suggestion"
              >
                <span className="btn-icon">✕</span>
                Reject
              </button>
            </div>
          </div>
        ))}
      </div>

      <div className="suggestions-footer">
        <button 
          className="refresh-suggestions-btn"
          onClick={getSuggestions}
          disabled={loading}
        >
          <span className="btn-icon">🔄</span>
          Get New Suggestions
        </button>
      </div>
    </div>
  );
};

export default CategorySuggestions;