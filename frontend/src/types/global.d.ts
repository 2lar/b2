declare global {
  interface Window {
    showApp: (email: string) => void;
  }
}

export {};
