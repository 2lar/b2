import { api as globalApi } from '../../services';
import type { ThoughtChainResult, ImpactAnalysis } from './types';

export async function getThoughtChains(
    nodeID: string,
    maxDepth = 10,
    maxBranches = 4,
): Promise<ThoughtChainResult> {
    return globalApi.getThoughtChains(nodeID, maxDepth, maxBranches);
}

export async function getImpactAnalysis(
    nodeID: string,
    maxDepth = 3,
): Promise<ImpactAnalysis> {
    return globalApi.getImpactAnalysis(nodeID, maxDepth);
}
