type IconActionVariant = 'default' | 'danger';

type IconActionButtonProps = {
  label: string;
  onClick: () => void;
  variant?: IconActionVariant;
  disabled?: boolean;
  loading?: boolean;
  title?: string;
};

export function IconActionButton({
  label,
  onClick,
  variant = 'default',
  disabled,
  loading,
  title,
}: IconActionButtonProps) {
  const baseClasses =
    'flex h-8 w-8 items-center justify-center rounded border text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-60';
  const toneClasses =
    variant === 'danger'
      ? 'border-red-200 text-red-600 hover:bg-red-50 disabled:hover:bg-transparent'
      : 'border-slate-300 text-slate-700 hover:bg-slate-100 disabled:hover:bg-transparent';

  return (
    <button
      type="button"
      onClick={onClick}
      className={`${baseClasses} ${toneClasses}`}
      disabled={disabled || loading}
      aria-label={label}
      title={title}
    >
      <span className="sr-only">{label}</span>
      {loading ? <LoadingIcon /> : variant === 'danger' ? <TrashIcon /> : <EditIcon />}
    </button>
  );
}

function EditIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-4"
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4 13.5V16h2.5L15 7.5 12.5 5 4 13.5z" />
      <path d="m11.5 4.5 2 2" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-4"
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
    >
      <path d="M5 6h10" />
      <path d="M8 6v8" />
      <path d="M12 6v8" />
      <path d="M6 6V4.5A1.5 1.5 0 0 1 7.5 3h5A1.5 1.5 0 0 1 14 4.5V6" />
      <path d="M6 6h8l-.8 9a1 1 0 0 1-1 .9H7.8a1 1 0 0 1-1-.9L6 6Z" />
    </svg>
  );
}

function LoadingIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-4 animate-spin"
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
    >
      <path d="M10 3a7 7 0 1 1-4.95 2.05" />
    </svg>
  );
}
