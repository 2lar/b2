import cytoscape from 'cytoscape';

// This tells TypeScript that the global 'Window' object has our custom properties.
declare global {
  interface Window {
    cy: cytoscape.Core;
    showApp: (email: string) => void;
  }
}