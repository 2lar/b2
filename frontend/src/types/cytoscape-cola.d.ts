declare module 'cytoscape-cola' {
    import { LayoutOptions } from 'cytoscape';
    
    interface ColaLayoutOptions extends LayoutOptions {
        name: 'cola';
        animate?: boolean;
        refresh?: number;
        maxSimulationTime?: number;
        nodeSpacing?: (() => number) | number;
        edgeLength?: ((edge: any) => number) | number;
        alignment?: (node: any) => { x: number; y: number };
        gravity?: number;
        padding?: number;
        avoidOverlap?: boolean;
        randomize?: boolean;
        unconstrIter?: number;
        userConstIter?: number;
        allConstIter?: number;
        handleDisconnected?: boolean;
        convergenceThreshold?: number;
        flow?: {
            enabled: boolean;
            friction: number;
        };
        infinite?: boolean;
    }

    const cola: any;
    export = cola;
}