import React, { useState, useEffect } from 'react';
import { Session } from '@supabase/supabase-js';
import { auth } from '../ts/authClient';
import { webSocketClient } from '../ts/webSocketClient';
import AuthSection from './AuthSection';
import Dashboard from './Dashboard';

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
        <>
            {session && session.user?.email ? (
                <Dashboard 
                    user={session.user} 
                    onSignOut={handleSignOut}
                />
            ) : (
                <AuthSection onAuthSuccess={setSession} />
            )}
        </>
    );
};

export default App;