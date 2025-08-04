/**
 * Header Component - Application Navigation Bar
 * 
 * Purpose:
 * Provides the top navigation bar with user controls and application-wide settings.
 * Displays user information and provides access to key application functions.
 * 
 * Key Features:
 * - Dark/Light theme toggle with persistence
 * - User profile dropdown with email display
 * - Sign-out functionality
 * - Responsive design that adapts to screen size
 * - Click-outside detection for dropdown management
 * 
 * Theme Management:
 * - Persists theme preference in localStorage
 * - Applies theme changes to document root element
 * - Defaults to dark theme for new users
 * 
 * State Management:
 * - theme: Current theme setting ('dark' | 'light')
 * - isDropdownOpen: Controls visibility of user dropdown menu
 * 
 * Integration:
 * - Receives user email from parent Dashboard
 * - Calls onSignOut callback when user signs out
 * - Works with CSS custom properties for theming
 */

import React, { useState, useEffect } from 'react';

interface HeaderProps {
    /** Email address of the authenticated user */
    userEmail: string;
    /** Callback function to handle user sign-out */
    onSignOut: () => void;
}

const Header: React.FC<HeaderProps> = ({ userEmail, onSignOut }) => {
    const [theme, setTheme] = useState<'dark' | 'light'>('dark');
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);

    useEffect(() => {
        // Get saved theme or default to dark
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
        <header>
            <h1>Memory Book</h1>
            <div className="header-actions">
                <button 
                    className="theme-toggle" 
                    onClick={toggleTheme}
                >
                    {theme === 'dark' ? '🌙 Dark' : '☀️ Light'}
                </button>
                
                <div className="user-dropdown">
                    <button 
                        className="user-dropdown-toggle"
                        onClick={toggleDropdown}
                    >
                        <span className="user-email">{userEmail}</span>
                        <span className="dropdown-arrow">{isDropdownOpen ? '▲' : '▼'}</span>
                    </button>
                    
                    {isDropdownOpen && (
                        <div className="user-dropdown-menu">
                            <button className="dropdown-item" onClick={closeDropdown}>
                                <span className="dropdown-icon">👤</span>
                                Profile
                            </button>
                            <button className="dropdown-item" onClick={closeDropdown}>
                                <span className="dropdown-icon">⚙️</span>
                                Settings
                            </button>
                            <button className="dropdown-item" onClick={closeDropdown}>
                                <span className="dropdown-icon">📊</span>
                                Analytics
                            </button>
                            <button className="dropdown-item" onClick={closeDropdown}>
                                <span className="dropdown-icon">💡</span>
                                Help
                            </button>
                            <div className="dropdown-divider"></div>
                            <button 
                                className="dropdown-item sign-out-item" 
                                onClick={() => {
                                    closeDropdown();
                                    onSignOut();
                                }}
                            >
                                <span className="dropdown-icon">🚪</span>
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