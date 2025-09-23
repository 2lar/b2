import React from 'react';
import styles from './NotificationBanner.module.css';

export type NotificationVariant = 'info' | 'success' | 'warning' | 'error';

interface NotificationBannerProps {
    message: string;
    variant?: NotificationVariant;
    onDismiss?: () => void;
}

const variantIcon: Record<NotificationVariant, string> = {
    info: 'ℹ️',
    success: '✅',
    warning: '⚠️',
    error: '❌',
};

const NotificationBanner: React.FC<NotificationBannerProps> = ({ message, variant = 'info', onDismiss }) => {
    const className = [styles.root, styles[variant]].join(' ');

    return (
        <div className={className} role="alert">
            <span className={styles.icon} aria-hidden="true">
                {variantIcon[variant]}
            </span>
            <span className={styles.message}>{message}</span>
            {onDismiss && (
                <button
                    type="button"
                    className={styles.dismiss}
                    onClick={onDismiss}
                    aria-label="Dismiss notification"
                >
                    ×
                </button>
            )}
        </div>
    );
};

export default NotificationBanner;
