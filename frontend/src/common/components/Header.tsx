import React, { useState, useEffect } from 'react';
import styles from './Header.module.css';

interface HeaderProps {
    userEmail: string;
    onSignOut: () => void;
    onToggleSidebar?: () => void;
    isSidebarCollapsed?: boolean;
    memoryCount?: number;
}

const Header: React.FC<HeaderProps> = ({
    userEmail,
    onSignOut,
    onToggleSidebar,
    isSidebarCollapsed,
    memoryCount,
}) => {
    const [theme, setTheme] = useState<'dark' | 'light'>('dark');
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);

    useEffect(() => {
        const savedTheme = (localStorage.getItem('theme') as 'dark' | 'light') || 'dark';
        setTheme(savedTheme);
        document.documentElement.setAttribute('data-theme', savedTheme);
    }, []);

    const toggleTheme = () => {
        const newTheme = theme === 'dark' ? 'light' : 'dark';
        setTheme(newTheme);
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
    };

    const toggleDropdown = () => {
        setIsDropdownOpen(!isDropdownOpen);
    };

    const closeDropdown = () => {
        setIsDropdownOpen(false);
    };

    return (
        <header className={styles.root}>
            <div className={styles.mobileControls}>
                {onToggleSidebar && (
                    <button
                        type="button"
                        className={styles.menuToggle}
                        onClick={onToggleSidebar}
                        title={isSidebarCollapsed ? 'Open navigation' : 'Close navigation'}
                        aria-label={isSidebarCollapsed ? 'Open navigation' : 'Close navigation'}
                    >
                        <span className={styles.hamburger}>
                            <span />
                            <span />
                            <span />
                        </span>
                    </button>
                )}
                <button
                    type="button"
                    className={styles.themeToggle}
                    onClick={toggleTheme}
                    aria-pressed={theme === 'dark'}
                    aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
                >
                    {theme === 'dark' ? '🌙 Dark' : '☀️ Light'}
                </button>
            </div>

            <div className={`${styles.brand} ${styles.center}`}>
                <h1 className={styles.heading}>Memory Book</h1>
                {typeof memoryCount === 'number' && (
                    <span className={styles.countBadge}>{memoryCount} memories</span>
                )}
            </div>

            <div className={styles.actions}>
                <div className={styles.userDropdown}>
                    <button type="button" className={styles.dropdownToggle} onClick={toggleDropdown}>
                        <span className={styles.email}>{userEmail}</span>
                        <span aria-hidden>{isDropdownOpen ? '▲' : '▼'}</span>
                    </button>

                    {isDropdownOpen && (
                        <div className={styles.menu}>
                            <button type="button" className={styles.menuButton} onClick={closeDropdown}>
                                <span aria-hidden>👤</span>
                                Profile
                            </button>
                            <button type="button" className={styles.menuButton} onClick={closeDropdown}>
                                <span aria-hidden>⚙️</span>
                                Settings
                            </button>
                            <button type="button" className={styles.menuButton} onClick={closeDropdown}>
                                <span aria-hidden>📊</span>
                                Analytics
                            </button>
                            <button type="button" className={styles.menuButton} onClick={closeDropdown}>
                                <span aria-hidden>💡</span>
                                Help
                            </button>
                            <div className={styles.divider} />
                            <button
                                type="button"
                                className={styles.menuButton}
                                onClick={() => {
                                    closeDropdown();
                                    onSignOut();
                                }}
                            >
                                <span aria-hidden>🚪</span>
                                Sign Out
                            </button>
                        </div>
                    )}
                </div>
            </div>
        </header>
    );
};

export default Header;
