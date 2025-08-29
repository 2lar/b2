import { defineConfig, loadEnv } from 'vite'
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
      outDir: '../dist',
      // Add rollup options for code splitting
      rollupOptions: {
        output: {
          // Manual chunks for vendor libraries
          manualChunks: {
            // React ecosystem
            'react-vendor': ['react', 'react-dom', 'react-router-dom'],
            // State management and data fetching
            'state-vendor': ['zustand', '@tanstack/react-query'],
            // Visualization libraries (the heaviest)
            'graph-vendor': ['cytoscape', 'cytoscape-cola'],
            // Utilities
            'utils-vendor': ['lodash-es'],
            // Authentication
            'auth-vendor': ['@supabase/supabase-js']
          },
          // Dynamic imports for features
          chunkFileNames: (chunkInfo) => {
            const facadeModuleId = chunkInfo.facadeModuleId ? chunkInfo.facadeModuleId.split('/').pop() : 'chunk'
            return `assets/[name]-${facadeModuleId}-[hash].js`
          }
        }
      },
      // Increase chunk size warning limit since we're manually chunking
      chunkSizeWarningLimit: 600,
      // Enable source maps for production debugging
      sourcemap: true,
      // Basic minification without terser for now
      minify: 'esbuild'
    },
    // Explicitly tell Vite where to find env files
    envDir: '../',  // This tells Vite to look in the root directory for .env files
    resolve: {
      alias: {
        '@app': resolve(__dirname, './src/app'),
        '@common': resolve(__dirname, './src/common'),
        '@features': resolve(__dirname, './src/features'),
        '@services': resolve(__dirname, './src/services'),
        '@types': resolve(__dirname, './src/types')
      }
    },
    // Optimize dependencies
    optimizeDeps: {
      include: ['cytoscape', 'cytoscape-cola', '@tanstack/react-query'],
      exclude: ['@tanstack/react-query-devtools']
    }
  }
})