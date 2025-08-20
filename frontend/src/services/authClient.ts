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

// Auth retry configuration
const AUTH_RETRY_CONFIG = {
    maxRetries: 3,
    baseDelay: 1000, // 1 second
    maxDelay: 10000, // 10 seconds
    backoffMultiplier: 2
};

// Track failed refresh attempts to prevent spam
let consecutiveRefreshFailures = 0;
let lastRefreshAttempt = 0;

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
            // Check if we should attempt refresh based on retry limits
            if (consecutiveRefreshFailures >= AUTH_RETRY_CONFIG.maxRetries) {
                const timeSinceLastAttempt = Date.now() - lastRefreshAttempt;
                const cooldownPeriod = Math.min(
                    AUTH_RETRY_CONFIG.baseDelay * Math.pow(AUTH_RETRY_CONFIG.backoffMultiplier, consecutiveRefreshFailures),
                    AUTH_RETRY_CONFIG.maxDelay
                );
                
                if (timeSinceLastAttempt < cooldownPeriod) {
                    // Still in cooldown, don't attempt refresh
                    return null;
                }
                
                // Reset failure count after cooldown
                consecutiveRefreshFailures = 0;
            }
            
            if (import.meta.env.MODE === 'development') {
                console.log('Token expired or expiring soon, attempting refresh...');
            }
            
            lastRefreshAttempt = Date.now();
            
            try {
                // Try to refresh the session
                const { data, error } = await supabase.auth.refreshSession();
                
                if (error) {
                    consecutiveRefreshFailures++;
                    
                    // Check if it's a network error
                    if (error.message?.includes('fetch') || error.message?.includes('network') || 
                        error.message?.includes('ERR_NAME_NOT_RESOLVED')) {
                        // Network error - don't spam console in production
                        if (import.meta.env.MODE === 'development') {
                            console.warn('Network error during token refresh, will retry with backoff');
                        }
                        return null;
                    }
                    
                    // Other auth errors - log and potentially sign out
                    if (consecutiveRefreshFailures >= AUTH_RETRY_CONFIG.maxRetries) {
                        console.error('Max refresh attempts exceeded. User may need to re-authenticate.');
                        // Clear the session to force re-login
                        await supabase.auth.signOut();
                        return null;
                    }
                    
                    if (import.meta.env.MODE === 'development') {
                        console.error('Token refresh failed:', error.message);
                    }
                    return null;
                }
                
                if (data.session) {
                    // Reset failure count on success
                    consecutiveRefreshFailures = 0;
                    session = data.session;
                    if (import.meta.env.MODE === 'development') {
                        console.log('Token refreshed successfully');
                    }
                } else {
                    consecutiveRefreshFailures++;
                    if (import.meta.env.MODE === 'development') {
                        console.error('Token refresh returned no session');
                    }
                    return null;
                }
            } catch (networkError) {
                consecutiveRefreshFailures++;
                // Network-level errors (DNS, connection failures)
                if (import.meta.env.MODE === 'development') {
                    console.warn('Network error during token refresh:', (networkError as Error).message);
                }
                return null;
            }
        }
        
        return session.access_token;
    },

    async signIn(email: string, password: string): Promise<Session | null> {
        try {
            const { data, error } = await supabase.auth.signInWithPassword({ email, password });
            if (error) {
                // Reset refresh failure count on successful auth attempt
                consecutiveRefreshFailures = 0;
                throw error;
            }
            // Reset refresh failure count on successful sign in
            consecutiveRefreshFailures = 0;
            return data.session;
        } catch (error) {
            if (import.meta.env.MODE === 'development') {
                console.error('Sign in failed:', (error as Error).message);
            }
            throw error;
        }
    },

    async signUp(email: string, password: string): Promise<Session | null> {
        try {
            const { data, error } = await supabase.auth.signUp({ email, password });
            if (error) {
                throw error;
            }
            return data.session;
        } catch (error) {
            if (import.meta.env.MODE === 'development') {
                console.error('Sign up failed:', (error as Error).message);
            }
            throw error;
        }
    },
    
    async signOut(): Promise<void> {
        try {
            // Reset refresh failure count on sign out
            consecutiveRefreshFailures = 0;
            lastRefreshAttempt = 0;
            await supabase.auth.signOut();
        } catch (error) {
            if (import.meta.env.MODE === 'development') {
                console.error('Sign out failed:', (error as Error).message);
            }
            throw error;
        }
    },

    // Add method to check if user should be logged out due to persistent auth failures
    shouldForceLogout(): boolean {
        return consecutiveRefreshFailures >= AUTH_RETRY_CONFIG.maxRetries;
    },

    // Add method to get current auth state for debugging
    getAuthDebugInfo(): { failures: number, lastAttempt: number } {
        return {
            failures: consecutiveRefreshFailures,
            lastAttempt: lastRefreshAttempt
        };
    }
};

initAuth();
