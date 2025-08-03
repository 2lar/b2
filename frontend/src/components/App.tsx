import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { Session } from '@supabase/supabase-js';
import { auth, webSocketClient } from '../services';
import AuthSection from './AuthSection';
import Dashboard from './Dashboard';
import CategoriesList from './CategoriesList';
import CategoryDetail from './CategoryDetail';

const App: React.FC = () => {
    const [session, setSession] = useState<Session | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        // Check for existing session
        const checkSession = async () => {
            try {
                const currentSession = await auth.getSession();
                setSession(currentSession);
            } catch (error) {
                console.error('Error checking session:', error);
            } finally {
                setIsLoading(false);
            }
        };

        checkSession();

        // Listen for auth state changes
        const { data: { subscription } } = auth.supabase.auth.onAuthStateChange(
            (event, session) => {
                console.log('Auth state changed:', event, session);
                setSession(session);
                
                if (session) {
                    // Connect WebSocket when user signs in
                    webSocketClient.connect();
                } else {
                    // Disconnect WebSocket when user signs out
                    webSocketClient.disconnect();
                }
            }
        );

        return () => {
            subscription.unsubscribe();
        };
    }, []);

    const handleSignOut = async () => {
        try {
            webSocketClient.disconnect();
            await auth.signOut();
            setSession(null);
        } catch (error) {
            console.error('Error signing out:', error);
        }
    };

    if (isLoading) {
        return (
            <div className="loading-container">
                <div>Loading...</div>
            </div>
        );
    }

    return (
        <Router>
            {session && session.user?.email ? (
                <Routes>
                    <Route path="/" element={<Dashboard user={session.user} onSignOut={handleSignOut} />} />
                    <Route path="/categories" element={<CategoriesList />} />
                    <Route path="/categories/:categoryId" element={<CategoryDetail />} />
                    <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
            ) : (
                <AuthSection onAuthSuccess={setSession} />
            )}
        </Router>
    );
};

export default App;