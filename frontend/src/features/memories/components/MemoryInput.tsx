/**
 * MemoryInput Component - Enhanced Memory Creation Interface
 * 
 * Purpose:
 * Provides an intelligent memory creation interface that automatically adapts to content length.
 * Uses SmartMemoryInput for auto-transition between simple input and rich document editing.
 * Maintains backward compatibility while offering enhanced user experience.
 * 
 * Key Features:
 * - Smart auto-transition between input modes based on content length
 * - Traditional compact mode for overlay use cases
 * - Enhanced document editing for longer content
 * - Auto-categorization and tag management
 * - Real-time content analysis and suggestions
 * 
 * Display Modes:
 * - Smart mode: Auto-transitions between simple and document editing
 * - Compact mode: Traditional streamlined interface for overlays
 * 
 * Integration:
 * - Backward compatible with existing usage patterns
 * - Enhanced with new document editing capabilities
 * - Maintains all existing callback patterns
 */

import React, { useState, useEffect, useRef } from 'react';
import { SmartMemoryInput } from '../../../components/SmartMemoryInput';
import { DocumentEditor } from '../../../components/DocumentEditor';
import { nodesApi } from '../api/nodes';

interface MemoryInputProps {
    /** Callback function called after successful memory creation */
    onMemoryCreated: () => void;
    /** Whether to render in compact mode (for overlay) */
    isCompact?: boolean;
    /** Whether this is the mobile bottom input */
    isMobile?: boolean;
    /** Callback function called when document mode is opened */
    onDocumentModeOpen?: () => void;
}

const MemoryInput: React.FC<MemoryInputProps> = ({ onMemoryCreated, isCompact = false, isMobile = false, onDocumentModeOpen }) => {
    // State for compact mode (only used when isCompact = true)
    const [content, setContent] = useState('');
    const [title, setTitle] = useState('');
    const [tags, setTags] = useState<string[]>([]);
    const [tagInput, setTagInput] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [status, setStatus] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
    const [isDocumentMode, setIsDocumentMode] = useState(false);
    const containerRef = useRef<HTMLDivElement>(null);

    const showStatus = (message: string, type: 'success' | 'error') => {
        setStatus({ message, type });
        setTimeout(() => setStatus(null), 3000);
    };

    // Add/remove class on parent when document mode changes
    useEffect(() => {
        if (isCompact && containerRef.current) {
            const parent = containerRef.current.closest('.memory-input-overlay');
            if (parent) {
                if (isDocumentMode) {
                    parent.classList.add('document-mode-active');
                } else {
                    parent.classList.remove('document-mode-active');
                }
            }
        }
    }, [isDocumentMode, isCompact]);

    // Document mode handlers (used by compact mode)
    const openDocumentMode = () => {
        setIsDocumentMode(true);
        // Call the callback to hide any open node details
        onDocumentModeOpen?.();
    };

    const handleDocumentClose = (savedContent: string, savedTitle: string) => {
        setContent(savedContent);
        setTitle(savedTitle);
        setIsDocumentMode(false);
    };

    const handleDocumentSave = async (savedContent: string, savedTitle?: string) => {
        console.log('DEBUG MemoryInput.handleDocumentSave - savedTitle:', JSON.stringify(savedTitle));
        setContent(savedContent);
        setTitle(savedTitle || '');
        
        try {
            const newNode = await nodesApi.createNode(savedContent, tags.length > 0 ? tags : undefined, savedTitle || '');
            
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
            
            onMemoryCreated();
        } catch (error) {
            showStatus('Failed to save document. Please try again.', 'error');
            console.error('Error creating memory:', error);
        }
    };

    // For non-compact mode, use the new SmartMemoryInput
    if (!isCompact) {
        return (
            <div className="dashboard-container" id="input-container" data-container="input">
                <div className="container-header" data-drag-handle>
                    <span className="container-title">Create a Memory</span>
                    <span className="drag-handle">⋮⋮</span>
                </div>
                <div className="container-content">
                    <SmartMemoryInput onMemoryCreated={onMemoryCreated} />
                </div>
            </div>
        );
    }

    // For compact mode handlers and functions
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
            // Remove last tag if input is empty and backspace is pressed
            setTags(tags.slice(0, -1));
        }
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        
        const trimmedContent = content.trim();
        if (!trimmedContent) return;

        setIsSubmitting(true);

        try {
            const newNode = await nodesApi.createNode(trimmedContent, tags.length > 0 ? tags : undefined, title);
            
            // Auto-categorize the new node only if we have a valid nodeId
            if (newNode && newNode.nodeId && newNode.nodeId !== 'undefined') {
                try {
                    await nodesApi.categorizeNode(newNode.nodeId);
                } catch (categorizationError) {
                    // Don't fail the whole operation if categorization fails
                    console.warn('Auto-categorization failed:', categorizationError);
                }
            }
            
            showStatus('Memory saved successfully!', 'success');
            setContent('');
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

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            if (!isSubmitting) {
                handleSubmit(e as any);
            }
        }
    };

    if (isCompact) {
        const isDevelopment = process.env.NODE_ENV === 'development';
        
        return (
            <>
                {/* Backdrop for document mode */}
                {isDocumentMode && (
                    <div 
                        className="document-modal-backdrop" 
                        onClick={(e) => {
                            if (e.target === e.currentTarget) {
                                handleDocumentClose(content, title);
                            }
                        }}
                        style={isDevelopment ? { border: '3px solid blue' } : undefined}
                    />
                )}
                
                {/* Morphing container - single container that transforms */}
                <div ref={containerRef}
                     className={`memory-input-morph-container ${isDocumentMode ? 'document-state' : 'compact-state'} ${isMobile ? 'mobile-optimized' : ''}`}
                     style={isDevelopment && isDocumentMode ? { border: '2px solid red' } : undefined}>
                    {isDocumentMode ? (
                        // Document editor mode
                        <DocumentEditor
                            initialContent={content}
                            initialTitle={title}
                            onClose={handleDocumentClose}
                            onSave={handleDocumentSave}
                            mode="embedded"
                        />
                    ) : (
                        // Compact input mode
                        <>
                            <form onSubmit={handleSubmit} className="compact-form">
                                <div className="input-row">
                                    <textarea 
                                        value={content}
                                        onChange={(e) => setContent(e.target.value)}
                                        onKeyDown={handleKeyDown}
                                        placeholder={isMobile ? "What's on your mind?" : "Write a memory, thought, or idea..."}
                                        rows={isMobile ? 1 : 2}
                                        required
                                        disabled={isSubmitting}
                                        className="compact-textarea"
                                        style={isMobile ? { fontSize: '16px' } : {}} // Prevent zoom on iOS
                                    />
                                    <button 
                                        type="submit" 
                                        className="compact-submit-btn"
                                        disabled={isSubmitting || !content.trim()}
                                        title="Save Memory"
                                    >
                                        {isSubmitting ? '⏳' : '✓'}
                                    </button>
                                </div>
                                
                                {tags.length > 0 && (
                                    <div className="compact-tags">
                                        {tags.map((tag, index) => (
                                            <span key={index} className="tag-pill-compact">
                                                {tag}
                                                <button
                                                    type="button"
                                                    className="tag-remove-compact"
                                                    onClick={() => removeTag(tag)}
                                                    disabled={isSubmitting}
                                                >
                                                    ×
                                                </button>
                                            </span>
                                        ))}
                                    </div>
                                )}
                                
                                <div className="compact-tag-input">
                                    <input
                                        type="text"
                                        value={tagInput}
                                        onChange={(e) => setTagInput(e.target.value)}
                                        onKeyDown={handleTagInputKeyDown}
                                        placeholder="Add tags..."
                                        disabled={isSubmitting}
                                        className="tag-input-compact"
                                        style={isMobile ? { fontSize: '16px' } : {}} // Prevent zoom on iOS
                                    />
                                </div>
                                
                                {/* Action Footer - New Addition */}
                                <div className="compact-footer">
                                    <button
                                        type="button"
                                        onClick={openDocumentMode}
                                        className="document-mode-btn-compact"
                                        disabled={isSubmitting}
                                        title="Open document editor for longer content"
                                    >
                                        📄 Document Mode
                                    </button>
                                </div>
                            </form>
                            {status && (
                                <div className={`status-message-compact ${status.type}`}>
                                    {status.message}
                                </div>
                            )}
                        </>
                    )}
                </div>
            </>
        );
    }

    // This should never be reached since compact mode is handled above
    return null;
};

export default MemoryInput;