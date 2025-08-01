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
        if (event === 'SIGNED_IN' && session?.user?.email) {
            // React components can listen to this change
            console.log('User signed in:', session.user.email);
        }
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
        const session = await this.getSession();
        return session ? session.access_token : null;
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
