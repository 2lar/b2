import React, { useState } from 'react';
import { api } from '../ts/apiClient';

interface MemoryInputProps {
    onMemoryCreated: () => void;
}

const MemoryInput: React.FC<MemoryInputProps> = ({ onMemoryCreated }) => {
    const [content, setContent] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [status, setStatus] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

    const showStatus = (message: string, type: 'success' | 'error') => {
        setStatus({ message, type });
        setTimeout(() => setStatus(null), 3000);
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        
        const trimmedContent = content.trim();
        if (!trimmedContent) return;

        setIsSubmitting(true);

        try {
            await api.createNode(trimmedContent);
            showStatus('Memory saved successfully!', 'success');
            setContent('');
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