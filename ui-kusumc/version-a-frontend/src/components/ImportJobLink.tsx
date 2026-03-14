import { Link } from 'react-router-dom';

type ImportJobLinkProps = {
  jobId: string;
  showLabel?: boolean;
};

export function ImportJobLink({ jobId, showLabel = false }: ImportJobLinkProps) {
  const href = `/devices/import/jobs?jobId=${encodeURIComponent(jobId)}`;

  return (
    <Link
      to={href}
      className="inline-flex items-center gap-1 text-emerald-600 transition hover:text-emerald-700"
    >
      <code className="rounded bg-slate-200 px-1 py-0.5 text-[11px] text-slate-700">{jobId}</code>
      {showLabel ? (
        <span className="text-[10px] font-semibold uppercase tracking-wide">Open</span>
      ) : null}
    </Link>
  );
}
