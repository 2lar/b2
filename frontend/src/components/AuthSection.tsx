import React, { useState } from 'react';
import { Session } from '@supabase/supabase-js';
import { auth } from '../ts/authClient';

interface AuthSectionProps {
    onAuthSuccess: (session: Session) => void;
}

const AuthSection: React.FC<AuthSectionProps> = ({ onAuthSuccess }) => {
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
            let session: Session | null = null;
            
            if (isSignUp) {
                session = await auth.signUp(email, password);
            } else {
                session = await auth.signIn(email, password);
            }

            if (session) {
                onAuthSuccess(session);
            }
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