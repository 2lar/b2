import { createClient, SupabaseClient } from '@supabase/supabase-js';
import { APIGatewayRequestSimpleAuthorizerHandlerV2, APIGatewaySimpleAuthorizerResult } from 'aws-lambda';

// Retrieve secrets from environment variables set by the CDK
const supabaseUrl = process.env.SUPABASE_URL!;
const serviceRoleKey = process.env.SUPABASE_SERVICE_ROLE_KEY!;

if (!supabaseUrl || !serviceRoleKey) {
  throw new Error('Supabase URL and Service Role Key must be provided.');
}

// Create a single, reusable Supabase client for server-side operations
const supabase: SupabaseClient = createClient(supabaseUrl, serviceRoleKey, {
    auth: { persistSession: false }
});

/**
 * Main handler for the Lambda Authorizer.
 */
export const handler: APIGatewayRequestSimpleAuthorizerHandlerV2 = async (event) => {
  console.log('Authorizer invoked with event:', JSON.stringify(event, null, 2));

  const authHeader = event.headers?.authorization;
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    console.log('No valid Authorization header found.');
    return { isAuthorized: false };
  }
  
  const token = authHeader.substring(7);

  try {
    // Use the official Supabase library to validate the token. This is the most secure method.
    const { data: { user }, error } = await supabase.auth.getUser(token);

    if (error || !user) {
      console.error('Token validation failed:', error?.message);
      return { isAuthorized: false };
    }

    console.log(`Successfully authenticated user: ${user.id}`);
    
    // Success! Return an "Allow" policy with user context.
    return {
      isAuthorized: true,
      context: {
        sub: user.id, 
        email: user.email || '',
        role: user.role || 'authenticated',
      },
    };
  } catch (err) {
    console.error('An unexpected error occurred:', err);
    return { isAuthorized: false };
  }
};