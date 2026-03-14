import { FormEvent, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';

import { useAuth } from '../auth';
import type { LoginPayload } from '../api/auth';
import type { SessionSnapshot } from '../api/session';

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { login, isAuthenticated } = useAuth();

  const redirectPath =
    (location.state as { from?: { pathname?: string } } | null)?.from?.pathname ?? '/';

  const [form, setForm] = useState<LoginPayload>({ username: '', password: '' });
  const [formError, setFormError] = useState<string | null>(null);

  const loginMutation = useMutation<SessionSnapshot, Error, LoginPayload>({
    mutationFn: login,
    onSuccess: () => {
      setFormError(null);
      navigate(redirectPath, { replace: true });
    },
    onError: (error: Error) => {
      setFormError(error.message);
    },
  });

  if (isAuthenticated) {
    return <Navigate to={redirectPath} replace />;
  }

  function handleChange(event: FormEvent<HTMLInputElement>) {
    const { name, value } = event.currentTarget;
    setForm((prev) => ({ ...prev, [name]: value }));
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    loginMutation.mutate(form);
  }

  return (
    <div className="mx-auto max-w-md rounded border border-slate-200 bg-white p-6 shadow-sm">
      <h2 className="text-lg font-semibold text-slate-800">Sign in</h2>
      <p className="mt-1 text-sm text-slate-500">
        Use the credentials seeded via the provisioning scripts to access the admin tools.
      </p>
      <form className="mt-6 space-y-4" onSubmit={handleSubmit}>
        <div>
          <label className="block text-sm font-medium text-slate-600" htmlFor="username">
            Username
          </label>
          <input
            id="username"
            name="username"
            type="text"
            value={form.username}
            onInput={handleChange}
            autoComplete="username"
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-emerald-500"
            placeholder="Him"
            required
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-slate-600" htmlFor="password">
            Password
          </label>
          <input
            id="password"
            name="password"
            type="password"
            value={form.password}
            onInput={handleChange}
            autoComplete="current-password"
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-emerald-500"
            placeholder="••••"
            required
          />
        </div>
        {formError && (
          <p className="text-sm text-red-600" role="alert">
            {formError}
          </p>
        )}
        <button
          type="submit"
          disabled={loginMutation.isPending}
          className="w-full rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-70"
        >
          {loginMutation.isPending ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </div>
  );
}
