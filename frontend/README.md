# Frontend - Brain2 React Application

This is the frontend React application for Brain2, built with Vite, TypeScript, and modern React patterns. The application provides an intuitive interface for managing knowledge graphs, memories, and categories.

## Overview

Brain2's frontend is a modern single-page application (SPA) that features:

- **Graph-based Knowledge Management**: Interactive visualization using Cytoscape.js
- **Memory Recording**: Rich text input with intelligent categorization
- **Real-time Collaboration**: WebSocket integration for live updates
- **Responsive Design**: Works seamlessly across desktop and mobile devices
- **Optimized Performance**: Advanced code splitting and bundle optimization

## Architecture

### Technology Stack

- **React 19** - Modern React with concurrent features
- **TypeScript** - Type-safe development
- **Vite** - Fast build tool and development server
- **Zustand** - Lightweight state management
- **TanStack Query** - Data fetching and caching
- **Cytoscape.js** - Graph visualization
- **Supabase** - Authentication and real-time features
- **React Router** - Client-side routing

### Project Structure

```
src/
├── app/                    # Application root and routing
├── common/                 # Shared components and utilities
├── features/               # Feature-specific components and logic
│   ├── dashboard/         # Main dashboard view
│   ├── categories/        # Category management
│   ├── graph/             # Graph visualization
│   └── memories/          # Memory recording and management
├── services/              # API clients and external integrations
└── types/                 # TypeScript type definitions
```

## Performance Optimization

### Code Splitting Strategy

The application uses advanced **code splitting** to optimize loading performance and reduce initial bundle size. Code splitting is the practice of breaking your application bundle into smaller chunks that can be loaded on demand.

#### 1. Manual Vendor Chunking

We use **manual chunking** in `vite.config.ts` to group related libraries into optimized vendor bundles:

```typescript
manualChunks: {
  // React ecosystem (core functionality)
  'react-vendor': ['react', 'react-dom', 'react-router-dom'],
  
  // State management and data fetching
  'state-vendor': ['zustand', '@tanstack/react-query'],
  
  // Visualization libraries (heaviest dependencies)
  'graph-vendor': ['cytoscape', 'cytoscape-cola'],
  
  // Utilities
  'utils-vendor': ['lodash-es'],
  
  // Authentication
  'auth-vendor': ['@supabase/supabase-js']
}
```

**Benefits:**
- **Better Caching**: Users don't re-download React when your app code changes
- **Parallel Loading**: Browser can download multiple chunks simultaneously  
- **Logical Separation**: Related libraries are grouped together
- **Size Optimization**: Largest libraries (like Cytoscape) are isolated

#### 2. Route-Based Code Splitting (Lazy Loading)

We use **React.lazy()** for route-based code splitting in `App.tsx`:

```typescript
// Lazy load heavy components
const Dashboard = lazy(() => import('../features/dashboard').then(module => ({ default: module.Dashboard })));
const CategoriesList = lazy(() => import('../features/categories').then(module => ({ default: module.CategoriesList })));
const CategoryDetail = lazy(() => import('../features/categories').then(module => ({ default: module.CategoryDetail })));
```

**Benefits:**
- **Faster Initial Load**: Only loads the current route's code
- **On-Demand Loading**: Additional routes load when needed
- **Memory Efficiency**: Unused components aren't loaded into memory
- **Progressive Enhancement**: App becomes more responsive as user navigates

#### 3. Dynamic Chunk Naming

Custom chunk file naming for better debugging and cache optimization:

```typescript
chunkFileNames: (chunkInfo) => {
  const facadeModuleId = chunkInfo.facadeModuleId ? chunkInfo.facadeModuleId.split('/').pop() : 'chunk'
  return `assets/[name]-${facadeModuleId}-[hash].js`
}
```

### Build Output Analysis

When you run `npm run build`, you'll see output similar to:

```
dist/assets/
├── index-a1b2c3d4.css           # Main stylesheet
├── index-e5f6g7h8.js            # App entry point (small)
├── react-vendor-i9j0k1l2.js     # React libraries (~150KB)
├── graph-vendor-m3n4o5p6.js     # Cytoscape libraries (~800KB)
├── state-vendor-q7r8s9t0.js     # Zustand + TanStack Query
├── utils-vendor-u1v2w3x4.js     # Lodash utilities
├── auth-vendor-y5z6a7b8.js      # Supabase authentication
├── Dashboard-c9d0e1f2.js        # Dashboard component (lazy)
├── CategoriesList-g3h4i5j6.js   # Categories list (lazy)
└── CategoryDetail-k7l8m9n0.js   # Category detail (lazy)
```

### Performance Monitoring

#### Bundle Size Analysis

To analyze your bundle composition:

```bash
# Build with bundle analysis
npm run build

# The build output shows chunk sizes and warns about large chunks
# Chunks over 600KB will show warnings (configured in vite.config.ts)
```

#### Runtime Performance

The app includes performance monitoring:

```typescript
// Built-in Web Vitals reporting
import { onCLS, onINP, onFCP, onLCP, onTTFB } from 'web-vitals';

// Automatically reported in development console
```

### Optimizing Bundle Size

#### Adding New Vendor Chunks

When adding heavy new dependencies, consider creating dedicated chunks:

```typescript
// In vite.config.ts manualChunks
manualChunks: {
  // ... existing chunks
  'chart-vendor': ['chart.js', 'd3'],  // New chart libraries
  'editor-vendor': ['monaco-editor'],   // Heavy editor
}
```

#### Guidelines for Chunk Organization

1. **Size Considerations**:
   - Keep chunks between 100KB - 600KB when possible
   - Isolate very large libraries (>500KB) into separate chunks
   - Group small related libraries together

2. **Usage Patterns**:
   - Libraries used on every page → Core vendor chunk
   - Libraries used on specific features → Feature-specific chunk
   - Libraries loaded conditionally → Separate chunk

3. **Update Frequency**:
   - Stable libraries (React) → Vendor chunks (better caching)
   - Frequently updated code → App chunks

#### Tree Shaking Optimization

Ensure optimal tree shaking:

```typescript
// Good: Import only what you need
import { debounce } from 'lodash-es';

// Avoid: Importing entire libraries
import * as _ from 'lodash-es';
```

## Environment Configuration

The frontend uses environment variables for configuration. See the [Environment Setup Guide](../docs/ENVIRONMENT_SETUP.md) for details.

### Required Variables

```bash
VITE_SUPABASE_URL=https://your-project.supabase.co
VITE_SUPABASE_ANON_KEY=your-anon-key
VITE_API_BASE_URL=https://your-api-gateway-url
```

### Development vs Production

The build system automatically configures environment-specific settings:

- **Development**: API calls to `VITE_API_BASE_URL_LOCAL` (usually localhost)
- **Production**: API calls to `VITE_API_BASE_URL` (deployed API Gateway)

## Development

### Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Or with environment loading
npm run dev:with-env
```

### Available Scripts

```bash
npm run dev              # Start development server
npm run dev:with-env     # Start with environment variables loaded
npm run build            # Production build with optimization
npm run build:with-env   # Build with environment variables loaded
npm run preview          # Preview production build
npm run test             # Run TypeScript type checking
npm run clean            # Clean node_modules and dist
```

### Development Features

- **Hot Module Replacement**: Instant updates without losing state
- **TypeScript Integration**: Full type checking and IntelliSense
- **Source Maps**: Easy debugging in production builds
- **Path Aliases**: Clean imports using `@app`, `@features`, etc.

### Code Organization

#### Feature-Based Architecture

Each feature is self-contained with its own:

```
features/memories/
├── components/          # React components
├── hooks/              # Custom React hooks  
├── services/           # API calls and business logic
├── types/              # TypeScript types
└── index.ts            # Public API exports
```

#### Shared Resources

```
common/
├── components/         # Reusable UI components
├── hooks/             # Shared React hooks
├── utils/             # Pure utility functions
└── constants/         # App constants
```

## API Integration

### Data Fetching Strategy

We use **TanStack Query** for efficient data fetching:

```typescript
// Automatic caching, background updates, and error handling
const { data: memories, isLoading } = useQuery({
  queryKey: ['memories'],
  queryFn: () => apiClient.getMemories()
});
```

### Real-time Updates

WebSocket integration provides real-time collaboration:

```typescript
// Automatic connection management based on auth state
useEffect(() => {
  if (user) {
    webSocketClient.connect(user.access_token);
  } else {
    webSocketClient.disconnect();
  }
}, [user]);
```

## Build and Deployment

### Production Build

```bash
# Full production build
npm run build

# Output goes to frontend/dist/
# Ready for deployment to any static hosting
```

### Build Optimizations

The production build includes:

- **Minification**: JavaScript and CSS compression with esbuild
- **Tree Shaking**: Removes unused code
- **Asset Hashing**: Cache-busting for updated files
- **Source Maps**: For production debugging
- **Chunk Splitting**: Optimized loading performance

### Deployment

The built application is a static SPA that can be deployed to:

- **AWS S3 + CloudFront** (configured in `../infra/`)
- **Vercel**, **Netlify**, or similar static hosts
- **Traditional web servers** with proper SPA routing setup

## Troubleshooting

### Common Issues

#### Environment Variables Not Loading
```bash
# Ensure variables are prefixed with VITE_
VITE_API_BASE_URL=https://api.example.com

# Check that .env file exists in project root
ls -la ../.env
```

#### Bundle Size Warnings
```bash
# If chunks are too large, consider splitting them
# Check vite.config.ts manualChunks configuration
# Use browser dev tools to analyze which libraries are largest
```

#### Route Loading Issues
```bash
# Ensure lazy-loaded components export default
export default function MyComponent() { ... }

# Check that Suspense wrapper exists in App.tsx
<Suspense fallback={<LoadingFallback />}>
  <Routes>...</Routes>
</Suspense>
```

### Performance Debugging

#### Analyzing Bundle Size

1. **Build Analysis**: Check build output for large chunks
2. **Browser DevTools**: Use Network tab to see actual load times
3. **Bundle Analyzer**: Consider adding webpack-bundle-analyzer equivalent

#### Monitoring Runtime Performance

```typescript
// Check for performance issues
import { getCLS, getFID, getFCP, getLCP, getTTFB } from 'web-vitals';

getCLS(console.log);  // Cumulative Layout Shift
getFID(console.log);  // First Input Delay
getFCP(console.log);  // First Contentful Paint
getLCP(console.log);  // Largest Contentful Paint
getTTFB(console.log); // Time to First Byte
```

## Contributing

### Code Style

- Use **TypeScript** for all new code
- Follow **React Hooks** patterns
- Implement **proper error boundaries**
- Add **JSDoc comments** for complex functions
- Use **feature-based** file organization

### Performance Guidelines

1. **Lazy load** heavy components and routes
2. **Memoize** expensive calculations with useMemo
3. **Debounce** user input for API calls
4. **Optimize** images and assets
5. **Monitor** bundle size with each change

### Testing

```bash
# Type checking (acts as basic testing)
npm run test

# For integration testing, ensure backend is running
# Frontend tests interact with real API endpoints
```

## Architecture Decisions

### Why Vite over Create React App?

- **Faster development**: ES modules + esbuild
- **Better tree shaking**: More efficient bundling
- **Modern tooling**: Native TypeScript support
- **Flexible configuration**: Easy to customize build process

### Why Zustand over Redux?

- **Simplicity**: Less boilerplate code
- **Performance**: Fine-grained reactivity
- **TypeScript**: Excellent type inference
- **Bundle size**: Smaller footprint

### Why Manual Chunking?

- **Control**: Precise control over bundle splitting
- **Caching**: Optimized cache invalidation strategy
- **Performance**: Tailored to our specific dependencies
- **Debugging**: Predictable chunk names and contents

This frontend architecture prioritizes performance, maintainability, and developer experience while providing a solid foundation for the Brain2 knowledge management application.