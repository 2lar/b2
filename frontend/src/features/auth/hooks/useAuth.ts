import { useState, useEffect } from 'react';
import { auth } from '../api/auth';
import type { Session } from '@supabase/supabase-js';

export const useAuth = () => {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    auth.getSession().then(setSession).finally(() => setLoading(false));
    
    const { data: { subscription } } = auth.supabase.auth.onAuthStateChange(
      (event, session) => setSession(session)
    );

    return () => subscription.unsubscribe();
  }, []);

  return { session, loading, auth };
};