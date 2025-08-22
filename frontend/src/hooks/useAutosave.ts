import { useState, useEffect, useCallback, useRef } from 'react';
import { debounce } from 'lodash-es';

interface UseAutosaveOptions {
  content: string;
  nodeId?: string;
  onSave: (content: string, title?: string) => Promise<void>;
  delay?: number;
}

export type SaveStatus = 'saved' | 'saving' | 'unsaved';

export function useAutosave({ 
  content, 
  nodeId, 
  onSave, 
  delay = 2000 
}: UseAutosaveOptions) {
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('saved');
  const [lastSavedContent, setLastSavedContent] = useState(content);
  
  // Create a stable reference to the debounced save function
  const debouncedSave = useCallback(
    debounce(async (content: string) => {
      setSaveStatus('saving');
      try {
        await onSave(content);
        setLastSavedContent(content);
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
    if (content !== lastSavedContent && content.trim()) {
      setSaveStatus('unsaved');
      debouncedSave(content);
    }
  }, [content, lastSavedContent, debouncedSave]);
  
  // Manual save trigger
  const triggerSave = useCallback(() => {
    if (content.trim()) {
      debouncedSave(content);
    }
  }, [content, debouncedSave]);
  
  // Cleanup on unmount
  useEffect(() => {
    return () => {
      debouncedSave.cancel();
    };
  }, [debouncedSave]);
  
  return { saveStatus, triggerSave, lastSavedContent };
}