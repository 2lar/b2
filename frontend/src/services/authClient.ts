/**
 * Authentication Client - Supabase Integration for Brain2
 * 
 * Provides authentication functionality using Supabase as the authentication provider.
 * Handles user registration, login, session management, and JWT token operations.
 */

import { createClient, SupabaseClient, Session, AuthChangeEvent } from '@supabase/supabase-js';

// Environment configuration
const SUPABASE_URL = import.meta.env.VITE_SUPABASE_URL;
const SUPABASE_ANON_KEY = import.meta.env.VITE_SUPABASE_ANON_KEY;

// Configuration validation

if (!SUPABASE_URL || SUPABASE_URL === 'undefined') {
    throw new Error('VITE_SUPABASE_URL is not defined. Please check your .env file.');
}

if (!SUPABASE_ANON_KEY || SUPABASE_ANON_KEY === 'undefined') {
    throw new Error('VITE_SUPABASE_ANON_KEY is not defined. Please check your .env file.');
}

// Initialize Supabase client
const supabase: SupabaseClient = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);

// Initialize auth state listener for React components
function initAuth(): void {
    supabase.auth.onAuthStateChange((event: AuthChangeEvent, session: Session | null) => {
        // React components can listen to this change through state management
    });
}

// Public auth object
export const auth = {
    // Expose Supabase client for React components
    supabase,
    
    async getSession(): Promise<Session | null> {
        const { data } = await supabase.auth.getSession();
        return data.session;
    },

    async getJwtToken(): Promise<string | null> {
        let session = await this.getSession();
        
        if (!session) {
            return null;
        }
        
        if (!session.access_token) {
            return null;
        }
        
        // Check if token is expired or will expire soon (5 minutes buffer)
        const currentTime = Date.now() / 1000;
        const bufferTime = 5 * 60; // 5 minutes in seconds
        const tokenExpiration = session.expires_at || 0;
        
        if (tokenExpiration > 0 && currentTime > (tokenExpiration - bufferTime)) {
            console.log('Token expired or expiring soon, attempting refresh...');
            
            // Try to refresh the session
            const { data, error } = await supabase.auth.refreshSession();
            
            if (error) {
                console.error('Token refresh failed:', error.message);
                return null;
            }
            
            if (data.session) {
                session = data.session;
                console.log('Token refreshed successfully');
            } else {
                console.error('Token refresh returned no session');
                return null;
            }
        }
        
        return session.access_token;
    },

    async signIn(email: string, password: string): Promise<Session | null> {
        const { data, error } = await supabase.auth.signInWithPassword({ email, password });
        if (error) throw error;
        return data.session;
    },

    async signUp(email: string, password: string): Promise<Session | null> {
        const { data, error } = await supabase.auth.signUp({ email, password });
        if (error) throw error;
        return data.session;
    },
    
    async signOut(): Promise<void> {
        await supabase.auth.signOut();
    }
};

initAuth();
