import React, { useState } from 'react';
import { SmartMemoryInput } from '../../../components/SmartMemoryInput';
import { DocumentEditor } from '../../../components/DocumentEditor';
import { nodesApi } from '../api/nodes';
import styles from './MemoryInput.module.css';

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

    const showStatus = (message: string, type: 'success' | 'error') => {
        setStatus({ message, type });
        setTimeout(() => setStatus(null), 3000);
    };

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

    // For non-compact mode, render full editor layout
    if (!isCompact) {
        return (
            <div className={styles.fullContainer}>
                <div className={styles.fullHeader}>
                    <span>Create a memory</span>
                    <span aria-hidden>⋮⋮</span>
                </div>
                <div className={styles.fullBody}>
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

    return (
        <>
            {isDocumentMode && (
                <div
                    className={styles.modalBackdrop}
                    onClick={(event) => {
                        if (event.target === event.currentTarget) {
                            handleDocumentClose(content, title);
                        }
                    }}
                >
                    <div className={`${styles.documentContainer} ${isMobile ? styles.mobileOptimized : ''}`}>
                        <DocumentEditor
                            initialContent={content}
                            initialTitle={title}
                            onClose={handleDocumentClose}
                            onSave={handleDocumentSave}
                            mode="embedded"
                        />
                    </div>
                </div>
            )}

            <div className={`${styles.compactContainer} ${isMobile ? styles.mobileOptimized : ''}`}>
                <form onSubmit={handleSubmit} className={styles.compactForm}>
                    <div className={styles.compactRow}>
                        <textarea
                            className={styles.textarea}
                            value={content}
                            onChange={(event) => setContent(event.target.value)}
                            onKeyDown={handleKeyDown}
                            placeholder={isMobile ? "What's on your mind?" : 'Write a memory, thought, or idea…'}
                            rows={isMobile ? 1 : 2}
                            required
                            disabled={isSubmitting}
                        />
                        <button
                            type="submit"
                            className={styles.submitButton}
                            disabled={isSubmitting || !content.trim()}
                            title="Save memory"
                        >
                            {isSubmitting ? '⏳' : '✓'}
                        </button>
                    </div>

                    {tags.length > 0 && (
                        <div className={styles.tags}>
                            {tags.map((tag, index) => (
                                <span key={`${tag}-${index}`} className={styles.tagPill}>
                                    #{tag}
                                    <button
                                        type="button"
                                        className={styles.tagRemove}
                                        onClick={() => removeTag(tag)}
                                        disabled={isSubmitting}
                                        aria-label={`Remove tag ${tag}`}
                                    >
                                        ×
                                    </button>
                                </span>
                            ))}
                        </div>
                    )}

                    <div className={styles.tagInputRow}>
                        <input
                            type="text"
                            value={tagInput}
                            onChange={(event) => setTagInput(event.target.value)}
                            onKeyDown={handleTagInputKeyDown}
                            placeholder="Add tags…"
                            disabled={isSubmitting}
                            className={styles.tagInput}
                        />
                    </div>

                    <div className={styles.footer}>
                        <button
                            type="button"
                            className={styles.documentButton}
                            onClick={openDocumentMode}
                            disabled={isSubmitting}
                        >
                            📄 Document mode
                        </button>
                    </div>
                </form>

                {status && (
                    <div className={`${styles.status} ${status.type === 'success' ? styles.statusSuccess : styles.statusError}`}>
                        {status.message}
                    </div>
                )}
            </div>
        </>
    );
};

export default MemoryInput;
