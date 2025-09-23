import React from 'react';

interface LoadingScreenProps {
    /** Message shown under the spinner */
    message?: string;
    /** Enable full-screen centering */
    fullScreen?: boolean;
}

const LoadingScreen: React.FC<LoadingScreenProps> = ({ message = 'Loading…', fullScreen = false }) => {
    return (
        <div
            className={`loading-screen${fullScreen ? ' loading-screen--fullscreen' : ''}`}
            role="status"
            aria-live="polite"
        >
            <span className="loading-screen__spinner" aria-hidden="true" />
            <span className="loading-screen__message">{message}</span>
        </div>
    );
};

export default LoadingScreen;
