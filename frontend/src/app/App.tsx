/**
 * App Component - Main Application Root
 * 
 * Purpose:
 * The root component of the Brain2 application that handles:
 * - Authentication state management
 * - Client-side routing between different views
 * - WebSocket connection lifecycle management
 * - User session handling and sign-out functionality
 * 
 * Key Features:
 * - Protected routing (redirects to auth if not logged in)
 * - Automatic WebSocket connection/disconnection based on auth state
 * - Loading states during authentication checks
 * - Clean session management and cleanup on sign-out
 * 
 * Routes:
 * - "/" - Main dashboard (protected)
 * - "/categories" - Categories list view (protected)  
 * - "/categories/:categoryId" - Individual category detail view (protected)
 * - Unauthenticated users see AuthSection component
 * 
 * Dependencies:
 * - useAuth hook for authentication state
 * - webSocketClient for real-time updates
 * - React Router for navigation
 */

import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, AuthSection } from '../features/auth';
import { webSocketClient } from '../services';
import { Dashboard } from '../features/dashboard';
import { CategoriesList, CategoryDetail } from '../features/categories';

const App: React.FC = () => {
    const { session, loading, auth } = useAuth();

    React.useEffect(() => {
        if (session) {
            webSocketClient.connect();
        } else {
            webSocketClient.disconnect();
        }
    }, [session]);

    const handleSignOut = async () => {
        try {
            webSocketClient.disconnect();
            await auth.signOut();
        } catch (error) {
            console.error('Error signing out:', error);
        }
    };

    if (loading) {
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
                <AuthSection />
            )}
        </Router>
    );
};

export default App;