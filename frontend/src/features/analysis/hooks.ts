import { useQuery } from '@tanstack/react-query';
import { getThoughtChains, getImpactAnalysis } from './api';

export function useThoughtChains(nodeID: string | null, enabled = true) {
    return useQuery({
        queryKey: ['thoughtChains', nodeID],
        queryFn: () => getThoughtChains(nodeID!),
        enabled: enabled && !!nodeID,
        staleTime: 60_000,
    });
}

export function useImpactAnalysis(nodeID: string | null, enabled = true) {
    return useQuery({
        queryKey: ['impactAnalysis', nodeID],
        queryFn: () => getImpactAnalysis(nodeID!),
        enabled: enabled && !!nodeID,
        staleTime: 60_000,
    });
}
