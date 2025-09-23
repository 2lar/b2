import React from 'react';
import styles from './LoadingScreen.module.css';

interface LoadingScreenProps {
    message?: string;
    fullScreen?: boolean;
}

const LoadingScreen: React.FC<LoadingScreenProps> = ({ message = 'Loading…', fullScreen = false }) => {
    return (
        <div
            className={`${styles.root}${fullScreen ? ` ${styles.fullScreen}` : ''}`}
            role="status"
            aria-live="polite"
        >
            <span className={styles.spinner} aria-hidden="true" />
            <span className={styles.message}>{message}</span>
        </div>
    );
};

export default LoadingScreen;
