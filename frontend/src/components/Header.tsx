import React, { useState, useEffect } from 'react';

interface HeaderProps {
    userEmail: string;
    onSignOut: () => void;
}

const Header: React.FC<HeaderProps> = ({ userEmail, onSignOut }) => {
    const [theme, setTheme] = useState<'dark' | 'light'>('dark');

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
                <span id="user-email">{userEmail}</span>
                <button 
                    className="secondary-btn" 
                    onClick={onSignOut}
                >
                    Sign Out
                </button>
            </div>
        </header>
    );
};

export default Header;