import { jsx as _jsx } from "react/jsx-runtime";
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useRef } from 'react';
import { MemoryRouter } from 'react-router-dom';
/**
 * Provides a fresh QueryClient instance per test run so cache state never leaks across specs.
 */
export function createTestQueryClient() {
    return new QueryClient({
        defaultOptions: {
            queries: {
                retry: false,
                gcTime: Infinity,
                staleTime: Infinity,
            },
            mutations: {
                retry: false,
            },
        },
    });
}
export function TestProviders({ children, routerProps, queryClient, }) {
    const clientRef = useRef();
    const client = queryClient ?? clientRef.current ?? createTestQueryClient();
    if (!queryClient && !clientRef.current)
        clientRef.current = client;
    return (_jsx(MemoryRouter, { ...routerProps, future: {
            v7_startTransition: true,
            v7_relativeSplatPath: true,
            ...(routerProps?.future ?? {}),
        }, children: _jsx(QueryClientProvider, { client: client, children: children }) }));
}
