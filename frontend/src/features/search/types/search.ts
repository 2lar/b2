export interface SearchResultItem {
    node_id: string;
    title: string;
    body: string;
    score: number;
    bm25_score: number;
    semantic_score: number;
    sources: string[];
    tags: string[];
}

export interface SearchResponse {
    query: string;
    results: SearchResultItem[];
    total: number;
    offset: number;
    limit: number;
    has_more: boolean;
}
