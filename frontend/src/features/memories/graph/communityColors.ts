const COMMUNITY_PALETTE = [
    '#00d4ff', '#ff006e', '#8338ec', '#ffbe0b', '#fb5607',
    '#3a86ff', '#06ffa5', '#ff4081', '#7209b7', '#f72585',
    '#00b4d8', '#e63946', '#2ec4b6', '#ff9f1c', '#6a4c93',
    '#43aa8b', '#f94144', '#577590', '#ffd166', '#90be6d',
];

export function getCommunityColor(communityId: string): string {
    if (!communityId) return COMMUNITY_PALETTE[0];
    let hash = 0;
    for (let i = 0; i < communityId.length; i++) {
        hash = ((hash << 5) - hash) + communityId.charCodeAt(i);
        hash |= 0;
    }
    return COMMUNITY_PALETTE[Math.abs(hash) % COMMUNITY_PALETTE.length];
}

export { COMMUNITY_PALETTE };
