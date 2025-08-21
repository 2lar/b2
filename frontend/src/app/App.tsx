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

import React, { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, AuthSection } from '../features/auth';
import { webSocketClient } from '../services';
import { useGraphStore } from '../stores/graphStore';
import { ErrorBoundary } from '../common';

// Lazy load heavy components
const Dashboard = lazy(() => import('../features/dashboard').then(module => ({ default: module.Dashboard })));
const CategoriesList = lazy(() => import('../features/categories').then(module => ({ default: module.CategoriesList })));
const CategoryDetail = lazy(() => import('../features/categories').then(module => ({ default: module.CategoryDetail })));

function LoadingFallback() {
    return (
        <div className="loading-container" style={{ 
            display: 'flex', 
            justifyContent: 'center', 
            alignItems: 'center', 
            height: '100vh',
            flexDirection: 'column' 
        }}>
            <div className="loading-spinner" style={{ 
                width: '40px', 
                height: '40px', 
                border: '4px solid #f3f3f3', 
                borderTop: '4px solid #3498db', 
                borderRadius: '50%', 
                animation: 'spin 2s linear infinite',
                marginBottom: '16px'
            }}></div>
            <div>Loading...</div>
            <style>{`
                @keyframes spin {
                    0% { transform: rotate(0deg); }
                    100% { transform: rotate(360deg); }
                }
            `}</style>
        </div>
    );
}

const App: React.FC = () => {
    const { session, loading, auth } = useAuth();
    const { isSidebarOpen } = useGraphStore();

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
        <ErrorBoundary 
            name="Application Root"
            onError={(error, errorInfo) => {
                console.error('Critical application error:', error, errorInfo);
            }}
        >
            <Router>
                <div style={{ display: 'flex' }}>
                    {isSidebarOpen && (
                        <div style={{ width: '200px', background: '#f0f0f0', padding: '1rem' }}>
                            <h2>Sidebar</h2>
                            <p>This is the sidebar content.</p>
                        </div>
                    )}
                    <div style={{ flex: 1 }}>
                        {session && session.user?.email ? (
                            <ErrorBoundary name="Authenticated Routes">
                                <Suspense fallback={<LoadingFallback />}>
                                    <Routes>
                                        <Route 
                                            path="/" 
                                            element={
                                                <ErrorBoundary name="Dashboard">
                                                    <Dashboard user={session.user} onSignOut={handleSignOut} />
                                                </ErrorBoundary>
                                            } 
                                        />
                                        <Route 
                                            path="/categories" 
                                            element={
                                                <ErrorBoundary name="Categories List">
                                                    <CategoriesList />
                                                </ErrorBoundary>
                                            } 
                                        />
                                        <Route 
                                            path="/categories/:categoryId" 
                                            element={
                                                <ErrorBoundary name="Category Detail">
                                                    <CategoryDetail />
                                                </ErrorBoundary>
                                            } 
                                        />
                                        <Route path="*" element={<Navigate to="/" replace />} />
                                    </Routes>
                                </Suspense>
                            </ErrorBoundary>
                        ) : (
                            <ErrorBoundary name="Authentication">
                                <AuthSection />
                            </ErrorBoundary>
                        )}
                    </div>
                </div>
            </Router>
        </ErrorBoundary>
    );
};

export default App;
