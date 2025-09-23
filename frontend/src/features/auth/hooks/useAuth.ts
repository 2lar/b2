import { useEffect, useRef, useState } from 'react';
import type { Session } from '@supabase/supabase-js';
import { auth } from '../api/auth';

const FORCE_LOGOUT_CHECK_INTERVAL_MS = 30_000;

export const useAuth = () => {
    const [session, setSession] = useState<Session | null>(null);
    const [loading, setLoading] = useState(true);
    const [authError, setAuthError] = useState<string | null>(null);
    const intervalRef = useRef<number | undefined>(undefined);

    useEffect(() => {
        let isMounted = true;

        const initializeSession = async () => {
            try {
                const currentSession = await auth.getSession();
                if (isMounted) {
                    setSession(currentSession);
                }
            } catch (error) {
                if (isMounted) {
                    setAuthError((error as Error).message);
                }
            } finally {
                if (isMounted) {
                    setLoading(false);
                }
            }
        };

        void initializeSession();

        const { data } = auth.supabase.auth.onAuthStateChange((_, nextSession) => {
            setSession(nextSession);
            if (nextSession && authError) {
                setAuthError(null);
            }
        });

        intervalRef.current = window.setInterval(() => {
            if (auth.shouldForceLogout()) {
                setAuthError('Your session has expired due to connection issues. Please sign in again.');
                setSession(null);
                void auth.signOut().catch(() => undefined);
            }
        }, FORCE_LOGOUT_CHECK_INTERVAL_MS);

        return () => {
            isMounted = false;
            data.subscription.unsubscribe();
            if (intervalRef.current) {
                window.clearInterval(intervalRef.current);
            }
        };
    }, [authError]);

    return {
        session,
        loading,
        authError,
        auth,
        clearAuthError: () => setAuthError(null),
    };
};
