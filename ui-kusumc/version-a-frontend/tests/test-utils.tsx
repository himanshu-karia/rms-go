import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactNode, useRef } from 'react';
import { MemoryRouter, MemoryRouterProps } from 'react-router-dom';

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

export function TestProviders({
  children,
  routerProps,
  queryClient,
}: {
  children: ReactNode;
  routerProps?: MemoryRouterProps;
  queryClient?: QueryClient;
}) {
  const clientRef = useRef<QueryClient>();
  const client = queryClient ?? clientRef.current ?? createTestQueryClient();
  if (!queryClient && !clientRef.current) clientRef.current = client;

  return (
    <MemoryRouter
      {...routerProps}
      future={{
        v7_startTransition: true,
        v7_relativeSplatPath: true,
        ...(routerProps?.future ?? {}),
      }}
    >
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    </MemoryRouter>
  );
}
