import { useState, useEffect, useCallback, useRef } from 'react';
import { debounce } from 'lodash-es';

interface UseAutosaveOptions {
  content: string;
  title?: string;
  nodeId?: string;
  onSave: (content: string, title?: string) => Promise<void>;
  delay?: number;
}

export type SaveStatus = 'saved' | 'saving' | 'unsaved';

export function useAutosave({ 
  content, 
  title,
  nodeId, 
  onSave, 
  delay = 2000 
}: UseAutosaveOptions) {
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('saved');
  const [lastSavedContent, setLastSavedContent] = useState(content);
  const [lastSavedTitle, setLastSavedTitle] = useState(title || '');
  
  // Create a stable reference to the debounced save function
  const debouncedSave = useCallback(
    debounce(async (content: string, title?: string) => {
      setSaveStatus('saving');
      try {
        await onSave(content, title);
        setLastSavedContent(content);
        setLastSavedTitle(title || '');
        setSaveStatus('saved');
      } catch (error) {
        setSaveStatus('unsaved');
        console.error('Auto-save failed:', error);
        // You could also show a toast notification here
      }
    }, delay),
    [onSave, nodeId, delay]
  );
  
  useEffect(() => {
    if ((content !== lastSavedContent || (title || '') !== lastSavedTitle) && content.trim()) {
      setSaveStatus('unsaved');
      debouncedSave(content, title);
    }
  }, [content, title, lastSavedContent, lastSavedTitle, debouncedSave]);
  
  // Manual save trigger
  const triggerSave = useCallback(() => {
    if (content.trim()) {
      debouncedSave(content, title);
    }
  }, [content, title, debouncedSave]);
  
  // Cleanup on unmount
  useEffect(() => {
    return () => {
      debouncedSave.cancel();
    };
  }, [debouncedSave]);
  
  return { saveStatus, triggerSave, lastSavedContent };
}