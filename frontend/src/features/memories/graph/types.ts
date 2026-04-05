export interface NodeAttributes {
    label: string;
    content: string;
    title: string;
    communityId: string;
    tags: string[];
    timestamp: string;
    x: number;
    y: number;
    size: number;
    color: string;
    originalColor: string;
    fixed: boolean;
}

export interface EdgeAttributes {
    weight: number;
    type: string;
    color: string;
    size: number;
}
