import React, { useState } from 'react';
import { useThoughtChains } from '../hooks';
import type { ThoughtChain } from '../types';
import styles from './ThoughtChainPanel.module.css';

interface ThoughtChainPanelProps {
    nodeID: string;
    onNodeClick?: (nodeId: string) => void;
    onHighlightPath?: (steps: string[]) => void;
}

const ThoughtChainPanel: React.FC<ThoughtChainPanelProps> = ({ nodeID, onNodeClick, onHighlightPath }) => {
    const { data, isLoading, error } = useThoughtChains(nodeID);
    const [expandedIndex, setExpandedIndex] = useState<number | null>(null);

    if (isLoading) return <div className={styles.loading}>Tracing thought chains...</div>;
    if (error || !data) return null;
    if (data.chains.length === 0) return <div className={styles.empty}>No thought chains found</div>;

    const toggleExpand = (idx: number) => {
        const next = expandedIndex === idx ? null : idx;
        setExpandedIndex(next);
        if (next !== null && onHighlightPath) {
            onHighlightPath(data.chains[next].steps);
        } else if (onHighlightPath) {
            onHighlightPath([]);
        }
    };

    return (
        <div className={styles.panel}>
            <div className={styles.header}>
                <span className={styles.title}>Thought Chains</span>
                <span className={styles.count}>{data.total_found} found</span>
            </div>
            <div className={styles.list}>
                {data.chains.map((chain: ThoughtChain, idx: number) => (
                    <ChainItem
                        key={idx}
                        chain={chain}
                        index={idx}
                        expanded={expandedIndex === idx}
                        onToggle={() => toggleExpand(idx)}
                        onNodeClick={onNodeClick}
                    />
                ))}
            </div>
        </div>
    );
};

interface ChainItemProps {
    chain: ThoughtChain;
    index: number;
    expanded: boolean;
    onToggle: () => void;
    onNodeClick?: (nodeId: string) => void;
}

const ChainItem: React.FC<ChainItemProps> = ({ chain, index, expanded, onToggle, onNodeClick }) => {
    return (
        <div className={`${styles.chainItem} ${expanded ? styles.expanded : ''}`}>
            <button className={styles.chainHeader} onClick={onToggle}>
                <span className={styles.chainIndex}>#{index + 1}</span>
                <span className={styles.chainMeta}>
                    {chain.steps.length} steps
                    {chain.communities_crossed > 0 && (
                        <span className={styles.crossBadge}>
                            {chain.communities_crossed} cross-community
                        </span>
                    )}
                </span>
                <span className={styles.chevron}>{expanded ? '\u25B2' : '\u25BC'}</span>
            </button>
            {expanded && (
                <div className={styles.chainSteps}>
                    {chain.steps.map((stepId, stepIdx) => (
                        <button
                            key={stepIdx}
                            className={styles.step}
                            onClick={() => onNodeClick?.(stepId)}
                        >
                            <span className={styles.stepNum}>{stepIdx + 1}</span>
                            <span className={styles.stepId}>{stepId.substring(0, 8)}...</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
};

export default ThoughtChainPanel;
