import React, { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { useAuth, AuthSection } from '../features/auth';
import { webSocketClient } from '../services';
import { ErrorBoundary, LoadingScreen } from '../common';
import { useLayoutStore } from '../stores/layoutStore';
import styles from './AppShell.module.css';

const Dashboard = lazy(() => import('../features/dashboard').then(module => ({ default: module.Dashboard })));
const CategoriesList = lazy(() => import('../features/categories').then(module => ({ default: module.CategoriesList })));
const CategoryDetail = lazy(() => import('../features/categories').then(module => ({ default: module.CategoryDetail })));

const AppSidebar: React.FC<{ isVisible: boolean }> = ({ isVisible }) => {
    const sidebarClassName = [
        styles.sidebar,
        !isVisible ? styles.sidebarCollapsed : '',
    ].filter(Boolean).join(' ');

    const linkClassName = ({ isActive }: { isActive: boolean }) => (
        [styles.link, isActive ? styles.linkActive : ''].filter(Boolean).join(' ')
    );

    return (
        <aside className={sidebarClassName}>
            <div className={styles.brand}>Brain2</div>
            <nav className={styles.nav} aria-label="Primary navigation">
                <NavLink to="/" end className={linkClassName}>
                    Dashboard
                </NavLink>
                <NavLink to="/categories" className={linkClassName}>
                    Categories
                </NavLink>
            </nav>
        </aside>
    );
};

const App: React.FC = () => {
    const { session, loading, auth } = useAuth();
    const isAppSidebarOpen = useLayoutStore(state => state.isAppSidebarOpen);
    const initializeFromViewport = useLayoutStore(state => state.initializeFromViewport);

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

    React.useEffect(() => {
        initializeFromViewport();
    }, [initializeFromViewport]);

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
                <a href="#main-content" className={styles.skipLink}>Skip to main content</a>
                <div className={styles.shell}>
                    <AppSidebar isVisible={isAppSidebarOpen} />
                    <main id="main-content" className={styles.content}>
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
                    </main>
                </div>
            </Router>
        </ErrorBoundary>
    );
};

export default App;
