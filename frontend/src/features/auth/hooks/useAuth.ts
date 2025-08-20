import { useState, useEffect, useRef } from 'react';
import { auth } from '../api/auth';
import type { Session } from '@supabase/supabase-js';

export const useAuth = () => {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [authError, setAuthError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  useEffect(() => {
    auth.getSession().then(setSession).finally(() => setLoading(false));
    
    const { data: { subscription } } = auth.supabase.auth.onAuthStateChange(
      (event, session) => {
        setSession(session);
        // Clear auth error on successful auth state change
        if (session && authError) {
          setAuthError(null);
        }
      }
    );

    // Check for persistent auth failures every 30 seconds
    intervalRef.current = window.setInterval(() => {
      if (auth.shouldForceLogout()) {
        setAuthError('Your session has expired due to connection issues. Please sign in again.');
        setSession(null);
        // Clear the failed session
        auth.signOut().catch(() => {
          // Ignore errors during cleanup
        });
      }
    }, 30000);

    return () => {
      subscription.unsubscribe();
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [authError]);

  return { 
    session, 
    loading, 
    authError,
    auth,
    // Method to manually clear auth error
    clearAuthError: () => setAuthError(null)
  };
};