import React, { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { useAuth, AuthSection } from '../features/auth';
import { webSocketClient } from '../services';
import { useGraphStore } from '../stores/graphStore';
import { ErrorBoundary, LoadingScreen } from '../common';

const Dashboard = lazy(() => import('../features/dashboard').then(module => ({ default: module.Dashboard })));
const CategoriesList = lazy(() => import('../features/categories').then(module => ({ default: module.CategoriesList })));
const CategoryDetail = lazy(() => import('../features/categories').then(module => ({ default: module.CategoryDetail })));

const AppSidebar: React.FC = () => {
    return (
        <aside className="app-shell__sidebar">
            <div className="app-sidebar__header">
                <span className="app-sidebar__brand">Brain2</span>
            </div>
            <nav className="app-sidebar__nav" aria-label="Primary navigation">
                <NavLink
                    to="/"
                    end
                    className={({ isActive }) => `app-sidebar__link${isActive ? ' app-sidebar__link--active' : ''}`}
                >
                    Dashboard
                </NavLink>
                <NavLink
                    to="/categories"
                    className={({ isActive }) => `app-sidebar__link${isActive ? ' app-sidebar__link--active' : ''}`}
                >
                    Categories
                </NavLink>
            </nav>
        </aside>
    );
};

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

    const handleSignOut = React.useCallback(() => {
        webSocketClient.disconnect();
        void auth.signOut().catch((error: unknown) => {
            console.error('Error signing out:', error);
        });
    }, [auth]);

    if (loading) {
        return <LoadingScreen fullScreen message="Preparing your workspace…" />;
    }

    return (
        <ErrorBoundary
            name="Application Root"
            onError={(error, errorInfo) => {
                console.error('Critical application error:', error, errorInfo);
            }}
        >
            <Router>
                <div className={`app-shell${isSidebarOpen ? ' app-shell--with-sidebar' : ''}`}>
                    {isSidebarOpen && <AppSidebar />}
                    <div className="app-shell__content">
                        {session && session.user?.email ? (
                            <ErrorBoundary name="Authenticated Routes">
                                <Suspense fallback={<LoadingScreen fullScreen message="Loading application…" />}>
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
