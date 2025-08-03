import React, { useState, useEffect } from 'react';

interface HeaderProps {
    userEmail: string;
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