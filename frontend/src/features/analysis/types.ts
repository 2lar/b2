export interface ThoughtChain {
    entry_node_id: string;
    steps: string[];
    communities_crossed: number;
}

export interface ThoughtChainResult {
    chains: ThoughtChain[];
    total_found: number;
    hubs: string[];
}

export type RiskLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
export type ImpactTier = 'WILL_BREAK' | 'LIKELY_AFFECTED' | 'MAY_AFFECT';

export interface DependencyGroup {
    tier: ImpactTier;
    depth: number;
    node_ids: string[];
}

export interface ImpactAnalysis {
    target_node_id: string;
    risk_level: RiskLevel;
    affected_community_count: number;
    total_affected_nodes: number;
    summary: string;
    dependents: DependencyGroup[];
}
