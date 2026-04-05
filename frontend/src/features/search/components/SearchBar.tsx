import React, { useState, useRef, useCallback, useEffect } from 'react';
import { useSearch } from '../hooks/useSearch';
import SearchResults from './SearchResults';
import styles from './SearchBar.module.css';

interface SearchBarProps {
    onNodeSelect?: (nodeId: string) => void;
}

const SearchBar: React.FC<SearchBarProps> = ({ onNodeSelect }) => {
    const [query, setQuery] = useState('');
    const [debouncedQuery, setDebouncedQuery] = useState('');
    const [isOpen, setIsOpen] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const { data, isLoading, isFetching } = useSearch(debouncedQuery);

    const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value;
        setQuery(value);
        setIsOpen(true);

        if (debounceRef.current) clearTimeout(debounceRef.current);
        debounceRef.current = setTimeout(() => {
            setDebouncedQuery(value.trim());
        }, 300);
    }, []);

    const handleClear = useCallback(() => {
        setQuery('');
        setDebouncedQuery('');
        setIsOpen(false);
        inputRef.current?.focus();
    }, []);

    const handleSelect = useCallback((nodeId: string) => {
        setIsOpen(false);
        onNodeSelect?.(nodeId);
    }, [onNodeSelect]);

    // Cmd+K / Ctrl+K shortcut
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
                e.preventDefault();
                inputRef.current?.focus();
                setIsOpen(true);
            }
            if (e.key === 'Escape') {
                setIsOpen(false);
                inputRef.current?.blur();
            }
        };
        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, []);

    // Click outside to close
    useEffect(() => {
        const handleClickOutside = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    useEffect(() => {
        return () => {
            if (debounceRef.current) clearTimeout(debounceRef.current);
        };
    }, []);

    const showResults = isOpen && debouncedQuery.length >= 2;

    return (
        <div className={styles.container} ref={containerRef}>
            <div className={styles.inputWrapper}>
                <svg className={styles.searchIcon} viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
                    <path fillRule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clipRule="evenodd" />
                </svg>
                <input
                    ref={inputRef}
                    type="text"
                    className={styles.input}
                    placeholder="Search memories... (Cmd+K)"
                    value={query}
                    onChange={handleChange}
                    onFocus={() => query.length >= 2 && setIsOpen(true)}
                    aria-label="Search memories"
                    aria-expanded={showResults}
                    role="combobox"
                    aria-controls="search-results"
                    autoComplete="off"
                />
                {(isLoading || isFetching) && debouncedQuery && (
                    <span className={styles.spinner} />
                )}
                {query && (
                    <button
                        type="button"
                        className={styles.clearButton}
                        onClick={handleClear}
                        aria-label="Clear search"
                    >
                        &times;
                    </button>
                )}
                <kbd className={styles.shortcut}>&#8984;K</kbd>
            </div>
            {showResults && (
                <div id="search-results" className={styles.dropdown}>
                    <SearchResults
                        results={data?.results ?? []}
                        total={data?.total ?? 0}
                        isLoading={isLoading && !data}
                        query={debouncedQuery}
                        onSelect={handleSelect}
                    />
                </div>
            )}
        </div>
    );
};

export default SearchBar;
