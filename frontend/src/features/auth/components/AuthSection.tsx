/**
 * AuthSection Component - User Authentication Interface
 * 
 * Purpose:
 * Provides the login and registration interface for users who are not authenticated.
 * Handles both sign-in and sign-up workflows with form validation and error handling.
 * 
 * Key Features:
 * - Toggle between sign-in and sign-up modes
 * - Email and password form validation
 * - Loading states during authentication requests
 * - Error message display with user-friendly messaging
 * - Responsive form design
 * - Integration with Supabase authentication
 * 
 * State Management:
 * - isSignUp: Controls whether showing login or registration form
 * - email/password: Form input values
 * - error: Error messages from authentication attempts
 * - isLoading: Loading state during API calls
 * 
 * Authentication Flow:
 * - Uses auth service for Supabase integration
 * - Successful authentication is handled by useAuth hook in App component
 * - Form resets and shows loading state during requests
 * - Displays specific error messages for failed attempts
 * 
 * Integration:
 * - Rendered by App component when user is not authenticated
 * - Works with useAuth hook for session state management
 * - Integrates with Supabase Auth for user management
 */

import React, { useState } from 'react';
import { auth } from '../api/auth';

const AuthSection: React.FC = () => {
    const [isSignUp, setIsSignUp] = useState(false);
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setIsLoading(true);

        try {
            if (isSignUp) {
                await auth.signUp(email, password);
            } else {
                await auth.signIn(email, password);
            }
            // The useAuth hook in App.tsx will handle the session update
        } catch (err: any) {
            console.error('Auth error:', err);
            setError(err.message || 'Authentication failed. Please try again.');
        } finally {
            setIsLoading(false);
        }
    };

    const toggleMode = () => {
        setIsSignUp(!isSignUp);
        setError('');
    };

    return (
        <div className="auth-container">
            <div className="auth-box">
                <h1>Brain2</h1>
                <p className="tagline">Your Second Brain - Building Connections Between Memories</p>
                
                <div id="auth-form">
                    <h2>{isSignUp ? 'Sign Up' : 'Sign In'}</h2>
                    <form onSubmit={handleSubmit}>
                        <input 
                            type="email" 
                            placeholder="Email" 
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            required 
                        />
                        <input 
                            type="password" 
                            placeholder="Password" 
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required 
                        />
                        <button 
                            type="submit" 
                            disabled={isLoading}
                        >
                            {isLoading ? 'Loading...' : (isSignUp ? 'Sign Up' : 'Sign In')}
                        </button>
                    </form>
                    <p className="auth-switch">
                        <span>
                            {isSignUp ? 'Already have an account?' : "Don't have an account?"}
                        </span>
                        <a href="#" onClick={(e) => { e.preventDefault(); toggleMode(); }}>
                            {isSignUp ? 'Sign In' : 'Sign Up'}
                        </a>
                    </p>
                    {error && <div className="error-message">{error}</div>}
                </div>
            </div>
        </div>
    );
};

export default AuthSection;
