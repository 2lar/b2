import Graph from 'graphology';

export function getNodesWithinHops(graph: Graph, startNodeId: string, maxHops: number): Set<string> {
    const visited = new Set<string>();
    const queue: Array<{ id: string; depth: number }> = [{ id: startNodeId, depth: 0 }];

    while (queue.length > 0) {
        const { id, depth } = queue.shift()!;
        if (visited.has(id)) continue;
        visited.add(id);
        if (depth < maxHops) {
            graph.forEachNeighbor(id, (neighbor) => {
                if (!visited.has(neighbor)) {
                    queue.push({ id: neighbor, depth: depth + 1 });
                }
            });
        }
    }
    return visited;
}
