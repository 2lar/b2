import { createClient, SupabaseClient, Session, AuthError } from '@supabase/supabase-js';

// Initialize Supabase client using environment variables
const SUPABASE_URL = import.meta.env.VITE_SUPABASE_URL;
const SUPABASE_ANON_KEY = import.meta.env.VITE_SUPABASE_ANON_KEY;

// Validate that required environment variables are set
if (!SUPABASE_URL || SUPABASE_URL === 'undefined') {
    throw new Error('VITE_SUPABASE_URL is not defined. Please check your .env file.');
}

if (!SUPABASE_ANON_KEY || SUPABASE_ANON_KEY === 'undefined') {
    throw new Error('VITE_SUPABASE_ANON_KEY is not defined. Please check your .env file.');
}

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
