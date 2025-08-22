/**
 * SmartMemoryInput Component - Intelligent Memory Input with Auto-Transition
 * 
 * Purpose:
 * Provides an intelligent input system that automatically transitions between simple textarea
 * and rich document editor based on content length. Offers the best of both worlds:
 * quick input for short thoughts and powerful editing for longer content.
 * 
 * Key Features:
 * - Auto-transition logic based on configurable thresholds
 * - Gentle transition hints with progress indicators
 * - Seamless content preservation during mode switches
 * - Manual mode switching available anytime
 * - Smooth animations for all transitions
 * - Character counter with threshold indicators
 * - Toast notifications for mode changes
 * 
 * Transition Logic:
 * - 0-800 chars: Simple input mode
 * - 800-1200 chars: Show suggestion hint with manual switch option
 * - 1200+ chars: Auto-transition to document mode (with undo option)
 * - 1500+ chars: Prevent return to inline mode (too large)
 */

import React, { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { DocumentEditor } from './DocumentEditor';
import { nodesApi } from '../features/memories/api/nodes';

interface SmartMemoryInputProps {
  /** Callback function called after successful memory creation */
  onMemoryCreated: () => void;
  /** Whether to render in compact mode (for overlay) */
  isCompact?: boolean;
}

// Configuration thresholds (should match backend constants)
const THRESHOLDS = {
  SUGGEST_DOCUMENT: 800,    // Show suggestion hint
  AUTO_OPEN_DOCUMENT: 1200, // Auto-transition to document mode
  MAX_INLINE: 1500,         // Absolute max for inline input
  MAX_CONTENT: 20000,       // Maximum total content (20KB)
};

export function SmartMemoryInput({ onMemoryCreated, isCompact = false }: SmartMemoryInputProps) {
  const [content, setContent] = useState('');
  const [title, setTitle] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [isDocumentMode, setIsDocumentMode] = useState(false);
  const [showTransitionHint, setShowTransitionHint] = useState(false);
  const [autoTransitioned, setAutoTransitioned] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [status, setStatus] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
  
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const lastContentRef = useRef('');
  
  // Show status message
  const showStatus = (message: string, type: 'success' | 'error') => {
    setStatus({ message, type });
    setTimeout(() => setStatus(null), 3000);
  };
  
  // Monitor content length for transitions
  useEffect(() => {
    const length = content.length;
    
    // Show hint when approaching document threshold
    if (length > THRESHOLDS.SUGGEST_DOCUMENT && length < THRESHOLDS.AUTO_OPEN_DOCUMENT) {
      setShowTransitionHint(true);
    } else {
      setShowTransitionHint(false);
    }
    
    // Auto-transition to document mode
    if (length > THRESHOLDS.AUTO_OPEN_DOCUMENT && !isDocumentMode && !autoTransitioned) {
      lastContentRef.current = content;
      setAutoTransitioned(true);
      
      // Small delay for smooth transition
      setTimeout(() => {
        setIsDocumentMode(true);
        showStatus('Switched to document mode for better editing experience', 'success');
      }, 100);
    }
  }, [content, isDocumentMode, autoTransitioned]);
  
  // Manual switch to document mode
  const switchToDocumentMode = () => {
    lastContentRef.current = content;
    setIsDocumentMode(true);
  };
  
  // Switch back to inline mode
  const switchToInlineMode = () => {
    if (content.length > THRESHOLDS.MAX_INLINE) {
      showStatus('Content too long for inline mode. Please use document editor.', 'error');
      return;
    }
    setIsDocumentMode(false);
    setAutoTransitioned(false);
  };
  
  // Handle document editor close
  const handleDocumentClose = (savedContent: string, savedTitle: string) => {
    setContent(savedContent);
    setTitle(savedTitle);
    setIsDocumentMode(false);
    setAutoTransitioned(false);
  };
  
  // Handle document save
  const handleDocumentSave = async (savedContent: string, savedTitle?: string) => {
    setContent(savedContent);
    setTitle(savedTitle || '');
    
    try {
      const newNode = await nodesApi.createNode(savedContent, tags.length > 0 ? tags : undefined);
      
      // Auto-categorize if we have a valid nodeId
      if (newNode && newNode.nodeId && newNode.nodeId !== 'undefined') {
        try {
          await nodesApi.categorizeNode(newNode.nodeId);
        } catch (categorizationError) {
          console.warn('Auto-categorization failed:', categorizationError);
        }
      }
      
      showStatus('Document saved successfully!', 'success');
      
      // Reset form
      setContent('');
      setTitle('');
      setTags([]);
      setTagInput('');
      setIsDocumentMode(false);
      setAutoTransitioned(false);
      
      onMemoryCreated();
    } catch (error) {
      showStatus('Failed to save document. Please try again.', 'error');
      console.error('Error creating memory:', error);
    }
  };
  
  // Tag management functions
  const addTag = (tag: string) => {
    const trimmedTag = tag.trim().toLowerCase();
    if (trimmedTag && !tags.includes(trimmedTag)) {
      setTags([...tags, trimmedTag]);
    }
    setTagInput('');
  };
  
  const removeTag = (tagToRemove: string) => {
    setTags(tags.filter(tag => tag !== tagToRemove));
  };
  
  const handleTagInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      if (tagInput.trim()) {
        addTag(tagInput);
      }
    } else if (e.key === 'Backspace' && !tagInput && tags.length > 0) {
      setTags(tags.slice(0, -1));
    }
  };
  
  // Handle regular form submission (for inline mode)
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    const trimmedContent = content.trim();
    if (!trimmedContent) return;
    
    setIsSubmitting(true);
    
    try {
      const newNode = await nodesApi.createNode(trimmedContent, tags.length > 0 ? tags : undefined);
      
      if (newNode && newNode.nodeId && newNode.nodeId !== 'undefined') {
        try {
          await nodesApi.categorizeNode(newNode.nodeId);
        } catch (categorizationError) {
          console.warn('Auto-categorization failed:', categorizationError);
        }
      }
      
      showStatus('Memory saved successfully!', 'success');
      setContent('');
      setTitle('');
      setTags([]);
      setTagInput('');
      onMemoryCreated();
    } catch (error) {
      showStatus('Failed to save memory. Please try again.', 'error');
      console.error('Error creating memory:', error);
    } finally {
      setIsSubmitting(false);
    }
  };
  
  // Handle keyboard shortcuts in textarea
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (!isSubmitting) {
        handleSubmit(e as any);
      }
    }
  };
  
  // Document mode render
  if (isDocumentMode) {
    return (
      <AnimatePresence mode="wait">
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.95 }}
          transition={{ duration: 0.2 }}
          className="document-mode-container"
        >
          <DocumentEditor
            initialContent={content}
            initialTitle={title}
            onClose={handleDocumentClose}
            onSave={handleDocumentSave}
            mode="embedded"
          />
        </motion.div>
      </AnimatePresence>
    );
  }
  
  // Inline mode render
  return (
    <div className="smart-input-container">
      {/* Transition Hint */}
      <AnimatePresence>
        {showTransitionHint && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 10 }}
            className="transition-hint"
          >
            <div className="hint-content">
              <span className="hint-icon">📝</span>
              <span>Your note is getting long!</span>
              <button 
                onClick={switchToDocumentMode}
                className="switch-btn"
              >
                Switch to Document Mode
              </button>
            </div>
            <div className="progress-bar">
              <div 
                className="progress-fill"
                style={{ 
                  width: `${(content.length / THRESHOLDS.AUTO_OPEN_DOCUMENT) * 100}%` 
                }}
              />
            </div>
          </motion.div>
        )}
      </AnimatePresence>
      
      {/* Main Input Area */}
      <div className="input-wrapper">
        <form onSubmit={handleSubmit}>
          <textarea
            ref={inputRef}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="What's on your mind? Start typing and we'll help you organize your thoughts..."
            className={`memory-input ${showTransitionHint ? 'with-hint' : ''}`}
            rows={4}
            maxLength={THRESHOLDS.MAX_INLINE}
            disabled={isSubmitting}
          />
          
          {/* Tag Input Section */}
          <div className="tag-input-section">
            <label htmlFor="tag-input">Tags (optional)</label>
            <div className="tag-input-container">
              {tags.map((tag, index) => (
                <span key={index} className="tag-pill">
                  {tag}
                  <button
                    type="button"
                    className="tag-remove"
                    onClick={() => removeTag(tag)}
                    disabled={isSubmitting}
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                id="tag-input"
                type="text"
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={handleTagInputKeyDown}
                placeholder={tags.length === 0 ? "Add tags (press Enter or comma to add)" : "Add tag..."}
                disabled={isSubmitting}
                className="tag-input"
              />
            </div>
          </div>
          
          {/* Input Footer */}
          <div className="input-footer">
            <div className="char-counter">
              <span className={content.length > THRESHOLDS.SUGGEST_DOCUMENT ? 'warning' : ''}>
                {content.length}
              </span>
              {content.length > THRESHOLDS.SUGGEST_DOCUMENT && (
                <span className="threshold-indicator">
                  / {THRESHOLDS.AUTO_OPEN_DOCUMENT} until document mode
                </span>
              )}
            </div>
            
            {/* Manual Document Mode Button */}
            <button
              type="button"
              onClick={switchToDocumentMode}
              className="document-mode-btn"
              title="Open document editor"
            >
              📄 Document Mode
            </button>
            
            <button 
              type="submit"
              className="save-btn primary"
              disabled={isSubmitting || !content.trim()}
            >
              {isSubmitting ? 'Saving...' : 'Save Memory'}
            </button>
          </div>
        </form>
        
        {/* Status Messages */}
        {status && (
          <div className={`status-message ${status.type}`}>
            {status.message}
          </div>
        )}
      </div>
    </div>
  );
}