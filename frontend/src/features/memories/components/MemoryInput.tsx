/**
 * MemoryInput Component - Memory Creation Form
 * 
 * Purpose:
 * Provides an intuitive form interface for users to create new memories with content and tags.
 * Handles automatic categorization and provides real-time feedback during the creation process.
 * 
 * Key Features:
 * - Rich text area for memory content input
 * - Dynamic tag management with keyboard shortcuts
 * - Auto-categorization using AI after memory creation
 * - Real-time form validation and submission feedback
 * - Keyboard shortcuts (Enter to submit, Shift+Enter for new line)
 * - Tag pills with easy removal functionality
 * - Status messages for success/error feedback
 * 
 * Tag Management:
 * - Add tags with Enter or comma key
 * - Remove tags with backspace when input is empty
 * - Prevents duplicate tags
 * - Case-insensitive tag handling
 * 
 * Auto-categorization:
 * - Automatically triggers AI categorization after memory creation
 * - Fails gracefully if categorization service is unavailable
 * - Does not block memory creation if categorization fails
 * 
 * State Management:
 * - content: Main memory text content
 * - tags: Array of user-defined tags
 * - tagInput: Current tag input field value
 * - isSubmitting: Loading state during memory creation
 * - status: Success/error message display
 * 
 * Integration:
 * - Calls onMemoryCreated callback to refresh parent components
 * - Uses API client for memory creation and categorization
 * - Positioned in top-right panel of Dashboard layout
 */

import React, { useState } from 'react';
import { nodesApi } from '../api/nodes';

interface MemoryInputProps {
    /** Callback function called after successful memory creation */
    onMemoryCreated: () => void;
}

const MemoryInput: React.FC<MemoryInputProps> = ({ onMemoryCreated }) => {
    const [content, setContent] = useState('');
    const [tags, setTags] = useState<string[]>([]);
    const [tagInput, setTagInput] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [status, setStatus] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

    const showStatus = (message: string, type: 'success' | 'error') => {
        setStatus({ message, type });
        setTimeout(() => setStatus(null), 3000);
    };

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
            const newNode = await nodesApi.createNode(trimmedContent, tags.length > 0 ? tags : undefined);
            
            // Auto-categorize the new node
            try {
                await nodesApi.categorizeNode(newNode.nodeId);
            } catch (categorizationError) {
                // Don't fail the whole operation if categorization fails
                console.warn('Auto-categorization failed');
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
        <div className="dashboard-container" id="input-container" data-container="input">
            <div className="container-header" data-drag-handle>
                <span className="container-title">Create a Memory</span>
                <span className="drag-handle">⋮⋮</span>
            </div>
            <div className="container-content">
                <form onSubmit={handleSubmit}>
                    <textarea 
                        value={content}
                        onChange={(e) => setContent(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="Write your memory, thought, or idea here... The system will automatically connect it to related memories."
                        rows={4}
                        required
                        disabled={isSubmitting}
                    />
                    
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
                    <button 
                        type="submit" 
                        className="primary-btn"
                        disabled={isSubmitting || !content.trim()}
                    >
                        {isSubmitting ? 'Saving...' : 'Save Memory'}
                    </button>
                </form>
                {status && (
                    <div className={`status-message ${status.type}`}>
                        {status.message}
                    </div>
                )}
            </div>
        </div>
    );
};

export default MemoryInput;