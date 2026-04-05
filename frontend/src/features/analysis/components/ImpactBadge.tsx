import React from 'react';
import { useImpactAnalysis } from '../hooks';
import type { RiskLevel } from '../types';
import styles from './ImpactBadge.module.css';

interface ImpactBadgeProps {
    nodeID: string;
}

const RISK_LABELS: Record<RiskLevel, string> = {
    CRITICAL: 'Critical',
    HIGH: 'High',
    MEDIUM: 'Medium',
    LOW: 'Low',
};

const ImpactBadge: React.FC<ImpactBadgeProps> = ({ nodeID }) => {
    const { data, isLoading } = useImpactAnalysis(nodeID);

    if (isLoading || !data || data.total_affected_nodes === 0) return null;

    const riskClass = styles[`risk${data.risk_level.charAt(0)}${data.risk_level.slice(1).toLowerCase()}`] || '';

    return (
        <div className={`${styles.badge} ${riskClass}`}>
            <span className={styles.label}>Impact</span>
            <span className={styles.level}>{RISK_LABELS[data.risk_level]}</span>
            <span className={styles.count}>{data.total_affected_nodes} affected</span>
        </div>
    );
};

export default ImpactBadge;
