import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

const STORAGE_KEY = 'pmkusum.activeProjectId.v1';

type ActiveProjectContextValue = {
  activeProjectId: string;
  setActiveProjectId: (projectId: string) => void;
  clearActiveProjectId: () => void;
};

const ActiveProjectContext = createContext<ActiveProjectContextValue | null>(null);

function readStoredProjectId(): string {
  if (typeof window === 'undefined' || !window.localStorage) {
    return '';
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return '';
    }
    return raw.trim();
  } catch {
    return '';
  }
}

function writeStoredProjectId(projectId: string) {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }

  const trimmed = projectId.trim();
  try {
    if (!trimmed) {
      window.localStorage.removeItem(STORAGE_KEY);
      return;
    }

    window.localStorage.setItem(STORAGE_KEY, trimmed);
  } catch {
    // ignore storage write errors
  }
}

export function ActiveProjectProvider({ children }: { children: ReactNode }) {
  const [activeProjectId, setActiveProjectIdState] = useState<string>(() => readStoredProjectId());

  useEffect(() => {
    writeStoredProjectId(activeProjectId);
  }, [activeProjectId]);

  const setActiveProjectId = useCallback((projectId: string) => {
    setActiveProjectIdState(projectId.trim());
  }, []);

  const clearActiveProjectId = useCallback(() => {
    setActiveProjectIdState('');
  }, []);

  const value = useMemo(
    () => ({ activeProjectId, setActiveProjectId, clearActiveProjectId }),
    [activeProjectId, clearActiveProjectId, setActiveProjectId],
  );

  return <ActiveProjectContext.Provider value={value}>{children}</ActiveProjectContext.Provider>;
}

export function useActiveProject(): ActiveProjectContextValue {
  const ctx = useContext(ActiveProjectContext);
  if (!ctx) {
    throw new Error('useActiveProject must be used within ActiveProjectProvider');
  }
  return ctx;
}
