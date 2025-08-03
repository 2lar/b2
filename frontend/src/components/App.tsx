import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { webSocketClient } from '../services';
import AuthSection from './AuthSection';
import Dashboard from './Dashboard';
import CategoriesList from './CategoriesList';
import CategoryDetail from './CategoryDetail';

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