import React from 'react';

interface TagDisplayProps {
    tags: string[] | undefined;
    maxTags?: number;
    size?: 'small' | 'medium' | 'large';
    className?: string;
}

const TagDisplay: React.FC<TagDisplayProps> = ({ 
    tags, 
    maxTags = 5, 
    size = 'medium',
    className = '' 
}) => {
    if (!tags || tags.length === 0) {
        return null;
    }

    const displayTags = maxTags > 0 ? tags.slice(0, maxTags) : tags;
    const remainingCount = tags.length - displayTags.length;

    return (
        <div className={`tag-container ${className}`}>
            <div className="tags">
                {displayTags.map((tag, index) => (
                    <span 
                        key={index} 
                        className={`tag tag-${size}`}
                        title={tag}
                    >
                        {tag}
                    </span>
                ))}
                {remainingCount > 0 && (
                    <span className={`tag tag-${size} tag-more`}>
                        +{remainingCount}
                    </span>
                )}
            </div>
        </div>
    );
};

export default TagDisplay;