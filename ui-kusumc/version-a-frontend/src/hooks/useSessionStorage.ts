import { Dispatch, SetStateAction, useEffect, useState } from 'react';

function isBrowser() {
  return typeof window !== 'undefined' && typeof window.sessionStorage !== 'undefined';
}

function readSessionStorage<T>(key: string, defaultValue: T): T {
  if (!isBrowser()) {
    return defaultValue;
  }

  try {
    const storedValue = window.sessionStorage.getItem(key);
    if (storedValue === null) {
      return defaultValue;
    }
    return JSON.parse(storedValue) as T;
  } catch (error) {
    console.warn(`Failed to read sessionStorage key "${key}":`, error);
    return defaultValue;
  }
}

export function useSessionStorage<T>(
  key: string,
  defaultValue: T,
): [T, Dispatch<SetStateAction<T>>] {
  const [value, setValue] = useState<T>(() => readSessionStorage(key, defaultValue));

  useEffect(() => {
    setValue(readSessionStorage(key, defaultValue));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  useEffect(() => {
    if (!isBrowser()) {
      return;
    }

    try {
      window.sessionStorage.setItem(key, JSON.stringify(value));
    } catch (error) {
      console.warn(`Failed to write sessionStorage key "${key}":`, error);
    }
  }, [key, value]);

  return [value, setValue];
}
