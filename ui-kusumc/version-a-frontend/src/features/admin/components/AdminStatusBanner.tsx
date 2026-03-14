export type AdminStatusMessage = {
  type: 'success' | 'error';
  message: string;
};

export function AdminStatusBanner({ status }: { status: AdminStatusMessage | null }) {
  if (!status) {
    return null;
  }

  const isError = status.type === 'error';
  const base = 'mb-4 rounded border px-3 py-2 text-sm';
  const tone = isError
    ? 'border-red-200 bg-red-50 text-red-700'
    : 'border-emerald-200 bg-emerald-50 text-emerald-700';

  return (
    <p className={`${base} ${tone}`} role={isError ? 'alert' : 'status'}>
      {status.message}
    </p>
  );
}
