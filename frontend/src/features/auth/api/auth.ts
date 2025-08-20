import { auth as globalAuth } from '../../../services/authClient';

export const auth = {
    signIn: globalAuth.signIn,
    signUp: globalAuth.signUp,
    signOut: globalAuth.signOut,
    getSession: globalAuth.getSession,
    getJwtToken: globalAuth.getJwtToken,
    supabase: globalAuth.supabase,
    shouldForceLogout: globalAuth.shouldForceLogout,
    getAuthDebugInfo: globalAuth.getAuthDebugInfo
};