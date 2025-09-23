import React, { useState } from 'react';
import { auth } from '../api/auth';
import styles from './AuthSection.module.css';

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
        <div className={styles.container}>
            <div className={styles.panel}>
                <div>
                    <h1 className={styles.title}>Brain2</h1>
                    <p className={styles.tagline}>Your second brain for connected memories</p>
                </div>
                <div>
                    <h2 className={styles.formHeading}>{isSignUp ? 'Create an account' : 'Welcome back'}</h2>
                    <form className={styles.form} onSubmit={handleSubmit}>
                        <input
                            className={styles.input}
                            type="email"
                            placeholder="Email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            required
                        />
                        <input
                            className={styles.input}
                            type="password"
                            placeholder="Password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                        />
                        <button type="submit" className={styles.submit} disabled={isLoading}>
                            {isLoading ? 'Loading…' : isSignUp ? 'Create account' : 'Sign in'}
                        </button>
                    </form>
                    <p className={styles.switchRow}>
                        {isSignUp ? 'Already have an account?' : "Don't have an account?"}
                        <a
                            className={styles.switchLink}
                            href="#"
                            onClick={(e) => {
                                e.preventDefault();
                                toggleMode();
                            }}
                        >
                            {isSignUp ? 'Sign in' : 'Sign up'}
                        </a>
                    </p>
                    {error && <div className={styles.error}>{error}</div>}
                </div>
            </div>
        </div>
    );
};

export default AuthSection;
