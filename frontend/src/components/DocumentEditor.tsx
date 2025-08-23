/**
 * DocumentEditor Component - Rich Document Editing Interface
 * 
 * Purpose:
 * Provides a full-featured document editing experience for longer content.
 * Includes auto-save, character/word/page counters, fullscreen mode, and keyboard shortcuts.
 * 
 * Key Features:
 * - Rich text editing with large textarea
 * - Auto-save with visual status indicators
 * - Character, word, and page counters
 * - Fullscreen mode for distraction-free writing
 * - Keyboard shortcuts (Cmd+S to save, Escape to exit fullscreen)
 * - Progress bar showing content limit usage
 * - Multiple display modes (modal, embedded, fullscreen)
 * 
 * Display Modes:
 * - modal: Centered overlay with backdrop
 * - embedded: Inline component for seamless transitions
 * - fullscreen: Takes over entire viewport
 */

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { motion } from 'framer-motion';
import { useAutosave, SaveStatus } from '../hooks/useAutosave';

interface DocumentEditorProps {
  initialContent?: string;
  initialTitle?: string;
  nodeId?: string;
  onClose: (content: string, title: string) => void;
  onSave?: (content: string, title?: string) => Promise<void>;
  mode?: 'fullscreen' | 'modal' | 'embedded';
}

const MAX_CONTENT = 20000; // 20KB limit (4 pages)
const CHARS_PER_PAGE = 5000; // Approximate characters per page

export function DocumentEditor({
  initialContent = '',
  initialTitle = '',
  nodeId,
  onClose,
  onSave,
  mode = 'modal'
}: DocumentEditorProps) {
  const [content, setContent] = useState(initialContent);
  const [title, setTitle] = useState(initialTitle);
  const [isFullscreen, setIsFullscreen] = useState(mode === 'fullscreen');
  const [stats, setStats] = useState({ words: 0, chars: 0, pages: 0 });
  const editorRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  
  // Auto-save functionality
  const { saveStatus, triggerSave } = useAutosave({
    content,
    title,
    nodeId,
    onSave: onSave || (async (c, t) => console.log('Auto-saved:', c.length, 'chars', t ? `title: ${t}` : '')),
    delay: 2000
  });
  
  // Calculate statistics whenever content changes
  useEffect(() => {
    const chars = content.length;
    const words = content.split(/\s+/).filter(Boolean).length;
    const pages = Math.ceil(chars / CHARS_PER_PAGE);
    
    setStats({ words, chars, pages });
    
    // Enforce content limit
    if (chars > MAX_CONTENT) {
      setContent(content.substring(0, MAX_CONTENT));
      // Show warning toast (you could integrate with your toast system here)
      console.warn('Maximum document size reached (4 pages)');
    }
  }, [content]);
  
  // Handle content changes
  const handleContentChange = (newContent: string) => {
    setContent(newContent);
  };
  
  // Handle close with confirmation if unsaved
  const handleClose = () => {
    if (saveStatus === 'unsaved') {
      if (!confirm('You have unsaved changes. Close anyway?')) {
        return;
      }
    }
    onClose(content, title);
  };
  
  // Manual save trigger
  const handleSave = () => {
    console.log('DEBUG DocumentEditor.handleSave - title:', JSON.stringify(title));
    if (onSave) {
      onSave(content, title);
    }
  };
  
  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Cmd/Ctrl + S to save
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        handleSave();
      }
      // Escape to exit fullscreen
      if (e.key === 'Escape' && isFullscreen && mode !== 'fullscreen') {
        setIsFullscreen(false);
      }
    };
    
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [content, isFullscreen, mode, onSave, title]);
  
  // Auto-focus textarea when component mounts
  useEffect(() => {
    if (textareaRef.current && mode !== 'embedded') {
      textareaRef.current.focus();
    }
  }, [mode]);
  
  const editorClass = `document-editor document-editor--${mode} ${isFullscreen ? 'fullscreen' : ''}`;
  
  return (
    <motion.div
      className={editorClass}
      initial={{ opacity: 0, y: mode === 'embedded' ? 20 : 0 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0 }}
      ref={editorRef}
    >
      {/* Header */}
      <div className="editor-header">
        <div className="header-left">
          <input
            type="text"
            placeholder="Untitled Document"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="title-input"
          />
        </div>
        
        <div className="header-right">
          {mode !== 'embedded' && (
            <button
              onClick={() => setIsFullscreen(!isFullscreen)}
              className="fullscreen-btn"
              title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            >
              {isFullscreen ? '⤩' : '⤢'}
            </button>
          )}
          
          <button onClick={handleClose} className="close-btn">
            ✕
          </button>
        </div>
      </div>
      
      {/* Progress Bar */}
      <div className="content-progress">
        <div 
          className="progress-bar"
          style={{
            width: `${(stats.chars / MAX_CONTENT) * 100}%`,
            backgroundColor: stats.chars > 18000 ? '#ef4444' : 
                           stats.chars > 15000 ? '#f59e0b' : '#10b981'
          }}
        />
      </div>
      
      {/* Editor Body */}
      <div className="editor-body">
        <textarea
          ref={textareaRef}
          value={content}
          onChange={(e) => handleContentChange(e.target.value)}
          placeholder="Start writing your document..."
          className="document-textarea"
          spellCheck={true}
          autoFocus={mode !== 'embedded'}
        />
      </div>
      
      {/* Footer Actions */}
      <div className="editor-footer">
        <div className="footer-left">
          <span className="shortcut-hint">
            ⌘S to save • ESC to exit
          </span>
        </div>
        
        <div className="footer-center">
          <div className="stats">
            <span>{stats.words} words</span>
            <span className="separator">•</span>
            <span>{stats.chars.toLocaleString()} / {MAX_CONTENT.toLocaleString()}</span>
            <span className="separator">•</span>
            <span>{stats.pages} / 4 pages</span>
            <span className="separator">•</span>
            <SaveIndicator status={saveStatus} />
          </div>
        </div>
        
        <div className="footer-right">
          <button 
            onClick={() => setContent('')}
            className="clear-btn secondary"
            title="Clear content"
          >
            Clear
          </button>
          <button 
            onClick={handleSave}
            className="save-btn primary"
            disabled={!content.trim()}
          >
            Save Document
          </button>
        </div>
      </div>
    </motion.div>
  );
}

// Save status indicator component
function SaveIndicator({ status }: { status: SaveStatus }) {
  return (
    <div className={`save-indicator save-indicator--${status}`}>
      {status === 'saved' && (
        <>
          <span className="icon">✓</span>
          <span>Saved</span>
        </>
      )}
      {status === 'saving' && (
        <>
          <span className="icon spinning">⟳</span>
          <span>Saving...</span>
        </>
      )}
      {status === 'unsaved' && (
        <>
          <span className="icon">○</span>
          <span>Unsaved changes</span>
        </>
      )}
    </div>
  );
}