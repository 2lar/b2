import { defineConfig, loadEnv} from 'vite'
import { resolve } from 'path'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  // Load env file based on `mode` in the current working directory.
  // Set the third parameter to '' to load all env regardless of the `VITE_` prefix.
  const env = loadEnv(mode, process.cwd(), '')
  
  // Debug logging
  console.log('Loading environment from:', process.cwd())
  console.log(
    'VITE_SUPABASE_URL:',
    'found: ' + env.VITE_SUPABASE_URL ? env.VITE_SUPABASE_URL.slice(0, 5) : 'Not found'
  )
  console.log(
    'VITE_SUPABASE_ANON_KEY:',
    'found: ' + env.VITE_SUPABASE_ANON_KEY ? env.VITE_SUPABASE_ANON_KEY.slice(0, 5) : 'Not found'
  )
  
  return {
    plugins: [react()],
    // This ensures that your `index.html` is the entry point
    root: 'src',
    // This sets the output directory for the build command
    build: {
      outDir: '../dist'
    },
    // Explicitly tell Vite where to find env files
    envDir: '../',  // This tells Vite to look in the parent directory (frontend/) for .env files
    resolve: {
      alias: {
        '@services': resolve(__dirname, './src/services'),
        '@components': resolve(__dirname, './src/components')
      }
    }
  }
})