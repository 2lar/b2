import express, { Request, Response } from 'express';
import cors from 'cors';
import path from 'path';
import dotenv from 'dotenv';
import fs from 'fs';

// Load environment variables
dotenv.config();

const app = express();
const PORT = process.env.PORT || 5000;

// Middleware
app.use(cors({
  origin: ['http://localhost:3000', 'http://localhost:5000'],
  credentials: true
}));
app.use(express.json());

// Import routes
import { notesRouter } from './routes/notes';
import { graphRouter } from './routes/graph';
import { queryRouter } from './routes/query';
import { llmRouter } from './routes/llm';
import { categoryRouter } from './routes/category';
import { chatModesRouter } from './routes/chatModes';

// API Routes
app.use('/api/notes', notesRouter);
app.use('/api/graph', graphRouter);
app.use('/api/query', queryRouter);
app.use('/api/llm', llmRouter);
app.use('/api/categories', categoryRouter);
app.use('/api/chatModes', chatModesRouter);

// Serve static files in production
if (process.env.NODE_ENV === 'production') {
  const clientBuildPath = path.join(process.cwd(), 'packages/server/dist/client/build');
    // Add these lines near where you define clientBuildPath
    console.log(`__dirname is: ${__dirname}`);
    console.log(`Resolved client path is: ${path.join(__dirname, '../client/build')}`);
    console.log(`This path exists: ${fs.existsSync(path.join(__dirname, '../client/build'))}`);
  
  if (fs.existsSync(clientBuildPath)) {
    console.log(`Serving static files from: ${clientBuildPath}`);
    
    // Serve static files
    app.use(express.static(clientBuildPath));
    
    // All other requests go to the React app
    app.get('*', (req: Request, res: Response) => {
      res.sendFile(path.join(clientBuildPath, 'index.html'));
    });
  } else {
    console.warn(`Client build directory not found at: ${clientBuildPath}`);
  }
}

// Start the server
app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
  console.log(`Environment: ${process.env.NODE_ENV || 'development'}`);
  console.log(`Current working directory: ${process.cwd()}`);
});