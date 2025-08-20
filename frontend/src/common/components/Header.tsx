/**
 * Header Component - Application Navigation Bar
 * 
 * Purpose:
 * Provides the top navigation bar with user controls, application-wide settings,
 * and quick access to main features like sidebar and memory list toggles.
 * 
 * Key Features:
 * - Dark/Light theme toggle with persistence
 * - User profile dropdown with email display
 * - Sign-out functionality
 * - Sidebar collapse/expand toggle
 * - Memory counter display in center
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
 * - Receives user email and controls from parent Dashboard
 * - Calls callback for sidebar toggle
 * - Displays live memory count
 * - Works with CSS custom properties for theming
 */

import React, { useState, useEffect } from 'react';

interface HeaderProps {
    /** Email address of the authenticated user */
    userEmail: string;
    /** Callback function to handle user sign-out */
    onSignOut: () => void;
    /** Callback to toggle sidebar visibility */
    onToggleSidebar?: () => void;
    /** Whether sidebar is currently collapsed */
    isSidebarCollapsed?: boolean;
    /** Total number of memories for display */
    memoryCount?: number;
}

const Header: React.FC<HeaderProps> = ({ 
    userEmail, 
    onSignOut, 
    onToggleSidebar, 
    isSidebarCollapsed, 
    memoryCount 
}) => {
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
            {/* Mobile Menu Button */}
            <div className="header-left">
                {onToggleSidebar && (
                    <button 
                        className="mobile-menu-toggle" 
                        onClick={onToggleSidebar}
                        title={isSidebarCollapsed ? 'Open Menu' : 'Close Menu'}
                        aria-label={isSidebarCollapsed ? 'Open Menu' : 'Close Menu'}
                    >
                        <span className="hamburger-icon">
                            <span></span>
                            <span></span>
                            <span></span>
                        </span>
                    </button>
                )}
            </div>

            <div className="header-center">
                <h1>Memory Book</h1>
                {memoryCount !== undefined && (
                    <span className="memory-counter-mobile">{memoryCount}</span>
                )}
            </div>
            
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