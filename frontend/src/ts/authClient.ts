/**
 * =============================================================================
 * Authentication Client - Supabase Integration for Brain2
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This module provides complete authentication functionality for the Brain2
 * application using Supabase as the authentication provider. It demonstrates
 * modern web authentication patterns, JWT token management, and secure
 * client-side authentication flows.
 * 
 * 🏗️ KEY AUTHENTICATION CONCEPTS:
 * 
 * 1. THIRD-PARTY AUTHENTICATION SERVICE:
 *    - Supabase handles user registration, login, and session management
 *    - JWT token-based authentication for stateless security
 *    - Built-in password hashing and security best practices
 *    - OAuth integration capabilities for social logins
 * 
 * 2. CLIENT-SIDE AUTHENTICATION PATTERNS:
 *    - Browser-based authentication with secure token storage
 *    - Automatic session management and renewal
 *    - Event-driven authentication state changes
 *    - Cross-tab session synchronization
 * 
 * 3. SECURITY CONSIDERATIONS:
 *    - Environment variable-based configuration
 *    - Anonymous vs service role key usage
 *    - Token expiration and refresh handling
 *    - HTTPS-only production deployment
 * 
 * 4. USER EXPERIENCE DESIGN:
 *    - Seamless sign-in/sign-up toggle functionality
 *    - Real-time authentication state feedback
 *    - Error handling with user-friendly messages
 *    - Automatic app transition on successful authentication
 * 
 * 🔐 AUTHENTICATION WORKFLOW:
 * 1. User enters email/password credentials
 * 2. Supabase validates credentials and returns JWT token
 * 3. Token stored securely in browser for API requests
 * 4. App transitions to authenticated state
 * 5. Token automatically renewed before expiration
 * 6. WebSocket connections authenticated with same token
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Modern web authentication implementation
 * - JWT token-based security patterns
 * - Third-party authentication service integration
 * - Client-side session management
 * - Security best practices for frontend apps
 */

import { createClient, SupabaseClient, Session, AuthError } from '@supabase/supabase-js';

// =============================================================================
// Environment Configuration and Validation
// =============================================================================

// VITE ENVIRONMENT VARIABLES:
// Vite exposes environment variables prefixed with VITE_ to the browser
// This enables configuration without hardcoding sensitive values

// Supabase project URL - unique identifier for your Supabase project
// SECURITY: This is safe to expose to the browser (public information)
const SUPABASE_URL = import.meta.env.VITE_SUPABASE_URL;

// Supabase anonymous key - public key for client-side operations
// SECURITY: Anonymous key has limited permissions, safe for browser exposure
// CONTRAST: Service role key (server-side only) has full admin permissions
const SUPABASE_ANON_KEY = import.meta.env.VITE_SUPABASE_ANON_KEY;

// =============================================================================
// Configuration Validation - Fail Fast Pattern
// =============================================================================

// DEFENSIVE PROGRAMMING:
// Validate configuration at startup to catch deployment issues early
// Better to fail fast than have mysterious runtime errors

if (!SUPABASE_URL || SUPABASE_URL === 'undefined') {
    throw new Error('VITE_SUPABASE_URL is not defined. Please check your .env file.');
}

if (!SUPABASE_ANON_KEY || SUPABASE_ANON_KEY === 'undefined') {
    throw new Error('VITE_SUPABASE_ANON_KEY is not defined. Please check your .env file.');
}

// =============================================================================
// Supabase Client Initialization
// =============================================================================

// CREATE CLIENT INSTANCE:
// Initialize Supabase client with project configuration
// This client handles all authentication operations and API calls
const supabase: SupabaseClient = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);

// DOM Elements
const authForm = document.getElementById('auth-submit-form') as HTMLFormElement;
const authTitle = document.getElementById('auth-title') as HTMLElement;
const authButton = document.getElementById('auth-button') as HTMLButtonElement;
const authSwitchText = document.getElementById('auth-switch-text') as HTMLElement;
const authSwitchLink = document.getElementById('auth-switch-link') as HTMLAnchorElement;
const authErrorEl = document.getElementById('auth-error') as HTMLElement;
const emailInput = document.getElementById('email') as HTMLInputElement;
const passwordInput = document.getElementById('password') as HTMLInputElement;

// State
let isSignUp: boolean = false;

// Initialize the auth module
function initAuth(): void {
    authForm.addEventListener('submit', handleAuthSubmit);

    authSwitchLink.addEventListener('click', (e: Event) => {
        e.preventDefault();
        toggleAuthMode();
    });

    supabase.auth.onAuthStateChange((event, session) => {
        if (event === 'SIGNED_IN' && session?.user?.email) {
            window.showApp(session.user.email);
        }
    });
}

// Toggle between sign in and sign up modes
function toggleAuthMode(): void {
    isSignUp = !isSignUp;

    authTitle.textContent = isSignUp ? 'Sign Up' : 'Sign In';
    authButton.textContent = isSignUp ? 'Sign Up' : 'Sign In';
    authSwitchText.textContent = isSignUp ? 'Already have an account?' : "Don't have an account?";
    authSwitchLink.textContent = isSignUp ? 'Sign In' : 'Sign Up';

    authErrorEl.textContent = '';
}

// Handle auth form submission
async function handleAuthSubmit(e: Event): Promise<void> {
    e.preventDefault();

    const email = emailInput.value.trim();
    const password = passwordInput.value;

    if (!email || !password) {
        showAuthError('Please fill in all fields');
        return;
    }

    authButton.disabled = true;
    authErrorEl.textContent = '';

    try {
        if (isSignUp) {
            const { data, error } = await supabase.auth.signUp({ email, password });
            if (error) throw error;
            if (data.user && !data.session) {
                showAuthError('Please check your email to confirm your account.');
                return;
            }
        } else {
            const { data, error } = await supabase.auth.signInWithPassword({ email, password });
            if (error) throw error;

            // Handle successful sign in immediately
            if (data.session?.user?.email) {
                window.showApp(data.session.user.email);
            }
        }
    } catch (error) {
        console.error('Auth error:', error);
        showAuthError((error as AuthError).message || 'Authentication failed.');
    } finally {
        authButton.disabled = false;
    }
}

// Display an authentication error message
function showAuthError(message: string): void {
    authErrorEl.textContent = message;
    setTimeout(() => {
        authErrorEl.textContent = '';
    }, 5000);
}

// Public auth object
export const auth = {
    async getSession(): Promise<Session | null> {
        const { data } = await supabase.auth.getSession();
        return data.session;
    },

    async getJwtToken(): Promise<string | null> {
        const session = await this.getSession();
        return session ? session.access_token : null;
    },
    
    async signOut(): Promise<void> {
        await supabase.auth.signOut();
    }
};

initAuth();
