import React from 'react';

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
    return (
        <div className={`notification-banner notification-banner--${variant}`} role="alert">
            <span className="notification-banner__icon" aria-hidden="true">
                {variantIcon[variant]}
            </span>
            <span className="notification-banner__message">{message}</span>
            {onDismiss && (
                <button
                    type="button"
                    className="notification-banner__dismiss"
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
