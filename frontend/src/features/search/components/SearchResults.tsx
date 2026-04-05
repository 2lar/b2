import React from 'react';
import type { SearchResultItem } from '../types/search';
import styles from './SearchResults.module.css';

interface SearchResultsProps {
    results: SearchResultItem[];
    total: number;
    isLoading: boolean;
    query: string;
    onSelect: (nodeId: string) => void;
}

function highlightMatch(text: string, query: string): React.ReactNode {
    if (!query) return text;
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const parts = text.split(new RegExp(`(${escaped})`, 'gi'));
    return parts.map((part, i) =>
        part.toLowerCase() === query.toLowerCase()
            ? <mark key={i} className={styles.highlight}>{part}</mark>
            : part
    );
}

function truncateBody(body: string, query: string, maxLen = 160): string {
    if (body.length <= maxLen) return body;
    const lower = body.toLowerCase();
    const idx = lower.indexOf(query.toLowerCase());
    if (idx === -1) return body.slice(0, maxLen) + '...';
    const start = Math.max(0, idx - 40);
    const end = Math.min(body.length, idx + query.length + maxLen - 40);
    return (start > 0 ? '...' : '') + body.slice(start, end) + (end < body.length ? '...' : '');
}

const SourceBadge: React.FC<{ source: string }> = ({ source }) => {
    const label = source === 'bm25' ? 'Keyword' : source === 'semantic' ? 'Semantic' : source;
    const className = source === 'bm25' ? styles.badgeKeyword
        : source === 'semantic' ? styles.badgeSemantic
        : styles.badgeBoth;
    return <span className={`${styles.badge} ${className}`}>{label}</span>;
};

const SearchResults: React.FC<SearchResultsProps> = ({ results, total, isLoading, query, onSelect }) => {
    if (isLoading) {
        return <div className={styles.message}>Searching...</div>;
    }

    if (results.length === 0) {
        return <div className={styles.message}>No results for "{query}"</div>;
    }

    return (
        <div className={styles.list} role="listbox">
            <div className={styles.summary}>{total} result{total !== 1 ? 's' : ''}</div>
            {results.map((item) => (
                <button
                    key={item.node_id}
                    className={styles.item}
                    onClick={() => onSelect(item.node_id)}
                    role="option"
                    aria-selected={false}
                >
                    <div className={styles.itemHeader}>
                        <span className={styles.title}>
                            {highlightMatch(item.title || 'Untitled', query)}
                        </span>
                        <div className={styles.badges}>
                            {item.sources.map((s) => <SourceBadge key={s} source={s} />)}
                        </div>
                    </div>
                    <div className={styles.body}>
                        {highlightMatch(truncateBody(item.body, query), query)}
                    </div>
                    {item.tags.length > 0 && (
                        <div className={styles.tags}>
                            {item.tags.map((tag) => (
                                <span key={tag} className={styles.tag}>{tag}</span>
                            ))}
                        </div>
                    )}
                </button>
            ))}
        </div>
    );
};

export default SearchResults;
