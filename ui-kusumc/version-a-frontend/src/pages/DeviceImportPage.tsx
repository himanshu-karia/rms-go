import { useMutation } from '@tanstack/react-query';
import { ChangeEvent, FormEvent, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  importDevicesCsv,
  importGovernmentCredentialsCsv,
  type ImportDevicesCsvResult,
  type ImportGovernmentCredentialsCsvResult,
} from '../api/devices';
import { useAuth } from '../auth';
import { parseCsvPreview, type CsvPreviewResult, type CsvPreviewRow } from '../utils/csvPreview';
import { downloadTextFile } from '../utils/download';

async function readFileAsText(file: File): Promise<string> {
  if (typeof file.text === 'function') {
    const textValue = await file.text();
    if (typeof textValue === 'string' && textValue !== '[object File]') {
      return textValue;
    }
  }

  if (typeof file.arrayBuffer === 'function') {
    const buffer = await file.arrayBuffer();
    return new TextDecoder().decode(buffer);
  }

  if (typeof FileReader !== 'undefined') {
    return await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const result = reader.result;
        resolve(typeof result === 'string' ? result : '');
      };
      reader.onerror = () => {
        reject(reader.error ?? new Error('Unable to read file'));
      };
      reader.readAsText(file);
    });
  }

  const response = new Response(file);
  return response.text();
}

function formatIssuedBy(value: string) {
  const trimmed = value.trim();
  return trimmed.length ? trimmed : undefined;
}

function formatLabel(title: string, detail: string) {
  return (
    <span className="flex flex-col leading-tight">
      <span className="font-medium text-slate-800">{title}</span>
      <span className="text-xs text-slate-500">{detail}</span>
    </span>
  );
}

type EnrollmentCsvRow = {
  imei?: string;
  stateId?: string;
  stateAuthorityId?: string;
  projectId?: string;
  serverVendorId?: string;
  protocolVersionId?: string;
  solarPumpVendorId?: string;
  [key: string]: string | undefined;
};

type GovernmentCsvRow = {
  imei?: string;
  clientId?: string;
  username?: string;
  password?: string;
  endpoints?: string;
  [key: string]: string | undefined;
};

const REQUIRED_ENROLLMENT_COLUMNS: Array<keyof EnrollmentCsvRow> = [
  'imei',
  'stateId',
  'stateAuthorityId',
  'projectId',
  'serverVendorId',
  'protocolVersionId',
  'solarPumpVendorId',
];

const REQUIRED_GOVERNMENT_COLUMNS: Array<keyof GovernmentCsvRow> = [
  'imei',
  'clientId',
  'username',
  'password',
];

function normalize(value: string | undefined) {
  return value?.trim() ?? '';
}

function findMissingColumns<T extends string>(header: string[] | undefined, required: T[]) {
  const fields = header ?? [];
  return required.filter((column) => !fields.includes(column));
}

function buildRowValidationErrors(
  row: Record<string, string | undefined>,
  requiredColumns: string[],
) {
  const errors: string[] = [];
  requiredColumns.forEach((column) => {
    if (!normalize(row[column])) {
      errors.push(`${column} is required`);
    }
  });
  return errors;
}

function buildPreviewErrorReport(rows: CsvPreviewRow<Record<string, unknown>>[]) {
  const header = ['rowNumber', 'errors'];
  const lines = rows
    .filter((row) => row.errors.length)
    .map((row) => {
      const escapedErrors = row.errors.map((err) => err.replace(/"/g, '""')).join('; ');
      return `${row.rowNumber},"${escapedErrors}"`;
    });

  return [header.join(','), ...lines].join('\n');
}

function validateEnrollmentRow(row: EnrollmentCsvRow) {
  const errors = buildRowValidationErrors(row, REQUIRED_ENROLLMENT_COLUMNS as string[]);

  const imei = normalize(row.imei);
  if (imei && !/^\d{10,16}$/.test(imei)) {
    errors.push('IMEI must be 10-16 digits');
  }

  return errors;
}

function validateEnrollmentFile(meta: { fields?: string[] }) {
  const missing = findMissingColumns(meta.fields, REQUIRED_ENROLLMENT_COLUMNS as string[]);
  return missing.length ? [`Missing columns: ${missing.join(', ')}`] : [];
}

function validateGovernmentRow(row: GovernmentCsvRow) {
  const errors = buildRowValidationErrors(row, REQUIRED_GOVERNMENT_COLUMNS as string[]);

  const endpoints = normalize(row.endpoints);
  if (!endpoints) {
    errors.push('endpoints is required');
  }

  return errors;
}

function validateGovernmentFile(meta: { fields?: string[] }) {
  const missing = findMissingColumns(meta.fields, REQUIRED_GOVERNMENT_COLUMNS as string[]);
  return missing.length ? [`Missing columns: ${missing.join(', ')}`] : [];
}

export function DeviceImportPage() {
  const { hasCapability } = useAuth();
  const canImportGovernmentCredentials = hasCapability(['devices:credentials', 'admin:all'], {
    match: 'any',
  });

  const enrollmentInputRef = useRef<HTMLInputElement | null>(null);
  const governmentInputRef = useRef<HTMLInputElement | null>(null);

  const [enrollmentFile, setEnrollmentFile] = useState<File | null>(null);
  const [enrollmentIssuedBy, setEnrollmentIssuedBy] = useState('');
  const [enrollmentError, setEnrollmentError] = useState<string | null>(null);
  const [enrollmentResult, setEnrollmentResult] = useState<ImportDevicesCsvResult | null>(null);
  const [enrollmentCompletedFileName, setEnrollmentCompletedFileName] = useState<string | null>(
    null,
  );
  const [enrollmentPreview, setEnrollmentPreview] =
    useState<CsvPreviewResult<EnrollmentCsvRow> | null>(null);
  const [enrollmentPreviewError, setEnrollmentPreviewError] = useState<string | null>(null);
  const [isEnrollmentPreviewLoading, setIsEnrollmentPreviewLoading] = useState(false);

  const [governmentFile, setGovernmentFile] = useState<File | null>(null);
  const [governmentIssuedBy, setGovernmentIssuedBy] = useState('');
  const [governmentError, setGovernmentError] = useState<string | null>(null);
  const [governmentResult, setGovernmentResult] =
    useState<ImportGovernmentCredentialsCsvResult | null>(null);
  const [governmentCompletedFileName, setGovernmentCompletedFileName] = useState<string | null>(
    null,
  );
  const [governmentPreview, setGovernmentPreview] =
    useState<CsvPreviewResult<GovernmentCsvRow> | null>(null);
  const [governmentPreviewError, setGovernmentPreviewError] = useState<string | null>(null);
  const [isGovernmentPreviewLoading, setIsGovernmentPreviewLoading] = useState(false);

  const enrollmentMutation = useMutation({
    mutationFn: importDevicesCsv,
  });

  const governmentMutation = useMutation({
    mutationFn: importGovernmentCredentialsCsv,
  });

  const enrollmentInvalidRows = enrollmentPreview?.rows.filter((row) => row.errors.length) ?? [];
  const enrollmentHasPreviewIssues = Boolean(
    enrollmentInvalidRows.length || (enrollmentPreview?.errors.length ?? 0),
  );
  const enrollmentPreviewColumns = enrollmentPreview
    ? enrollmentPreview.header.length
      ? enrollmentPreview.header
      : REQUIRED_ENROLLMENT_COLUMNS.map(String)
    : [];

  const governmentInvalidRows = governmentPreview?.rows.filter((row) => row.errors.length) ?? [];
  const governmentHasPreviewIssues = Boolean(
    governmentInvalidRows.length || (governmentPreview?.errors.length ?? 0),
  );
  const governmentPreviewColumns = governmentPreview
    ? governmentPreview.header.length
      ? governmentPreview.header
      : REQUIRED_GOVERNMENT_COLUMNS.map(String)
    : [];

  const handleEnrollmentFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const nextFile = event.target.files?.[0] ?? null;
    setEnrollmentFile(nextFile);
    setEnrollmentError(null);
    setEnrollmentResult(null);
    setEnrollmentPreview(null);
    setEnrollmentPreviewError(null);

    if (!nextFile) {
      setEnrollmentCompletedFileName(null);
      setIsEnrollmentPreviewLoading(false);
      return;
    }

    setIsEnrollmentPreviewLoading(true);

    try {
      const preview = await parseCsvPreview<EnrollmentCsvRow>(nextFile, {
        previewRowLimit: 100,
        validateRow: (row) => validateEnrollmentRow(row),
        validateFile: ({ meta }) => validateEnrollmentFile(meta),
      });
      setEnrollmentPreview(preview);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to preview CSV';
      setEnrollmentPreviewError(message);
    } finally {
      setIsEnrollmentPreviewLoading(false);
    }
  };

  const handleGovernmentFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    if (!canImportGovernmentCredentials) {
      setGovernmentError('Importing government credentials requires the devices:credentials capability.');
      if (governmentInputRef.current) {
        governmentInputRef.current.value = '';
      }
      setGovernmentFile(null);
      return;
    }

    const nextFile = event.target.files?.[0] ?? null;
    setGovernmentFile(nextFile);
    setGovernmentError(null);
    setGovernmentResult(null);
    setGovernmentPreview(null);
    setGovernmentPreviewError(null);

    if (!nextFile) {
      setGovernmentCompletedFileName(null);
      setIsGovernmentPreviewLoading(false);
      return;
    }

    setIsGovernmentPreviewLoading(true);

    try {
      const preview = await parseCsvPreview<GovernmentCsvRow>(nextFile, {
        previewRowLimit: 100,
        validateRow: (row) => validateGovernmentRow(row),
        validateFile: ({ meta }) => validateGovernmentFile(meta),
      });
      setGovernmentPreview(preview);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to preview CSV';
      setGovernmentPreviewError(message);
    } finally {
      setIsGovernmentPreviewLoading(false);
    }
  };

  const handleDownloadEnrollmentPreviewErrors = () => {
    if (!enrollmentPreview) {
      return;
    }

    const rowsWithErrors = enrollmentPreview.rows.filter((row) => row.errors.length);
    if (!rowsWithErrors.length) {
      return;
    }

    const report = buildPreviewErrorReport(rowsWithErrors);
    downloadTextFile('enrollment-preview-errors.csv', report, 'text/csv');
  };

  const handleDownloadGovernmentPreviewErrors = () => {
    if (!governmentPreview) {
      return;
    }

    const rowsWithErrors = governmentPreview.rows.filter((row) => row.errors.length);
    if (!rowsWithErrors.length) {
      return;
    }

    const report = buildPreviewErrorReport(rowsWithErrors);
    downloadTextFile('government-preview-errors.csv', report, 'text/csv');
  };
  const handleEnrollmentSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!enrollmentFile) {
      setEnrollmentError('Select a CSV file before uploading.');
      return;
    }

    const csv = await readFileAsText(enrollmentFile);

    const issuedBy = formatIssuedBy(enrollmentIssuedBy);
    const fileName = enrollmentFile.name;

    try {
      const result = await enrollmentMutation.mutateAsync({ csv, issuedBy });
      setEnrollmentResult(result);
      setEnrollmentError(null);
      setEnrollmentCompletedFileName(fileName);
      setEnrollmentFile(null);
      setEnrollmentPreview(null);
      setEnrollmentPreviewError(null);
      if (enrollmentInputRef.current) {
        enrollmentInputRef.current.value = '';
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to import devices';
      setEnrollmentError(message);
    }
  };

  const handleGovernmentSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canImportGovernmentCredentials) {
      setGovernmentError('Importing government credentials requires the devices:credentials capability.');
      return;
    }

    if (!governmentFile) {
      setGovernmentError('Select a CSV file before uploading.');
      return;
    }

    const csv = await readFileAsText(governmentFile);

    const issuedBy = formatIssuedBy(governmentIssuedBy);
    const fileName = governmentFile.name;

    try {
      const result = await governmentMutation.mutateAsync({ csv, issuedBy });
      setGovernmentResult(result);
      setGovernmentError(null);
      setGovernmentCompletedFileName(fileName);
      setGovernmentFile(null);
      setGovernmentPreview(null);
      setGovernmentPreviewError(null);
      if (governmentInputRef.current) {
        governmentInputRef.current.value = '';
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Unable to import government credentials';
      setGovernmentError(message);
    }
  };

  return (
    <div className="space-y-12">
      <section className="space-y-4">
        <h1 className="text-2xl font-semibold text-slate-900">Bulk Device Imports</h1>
        <p className="text-sm text-slate-600">
          Upload CSV files to enroll new devices or attach government-issued credential bundles. The
          sample templates below follow the backend requirements documented in
          <code className="mx-1 rounded bg-slate-200 px-1 py-0.5 text-[0.7rem]">
            device-import.service.ts
          </code>
          .
        </p>
        <p className="text-xs text-slate-500">
          Need to review previous uploads?{' '}
          <Link
            to="/devices/import/jobs"
            className="font-medium text-emerald-600 transition hover:text-emerald-700"
          >
            Open the import history view
          </Link>
          .
        </p>
        <div className="flex flex-wrap gap-3 text-sm">
          <a
            href="/csv-templates/device-full-enrollment.csv"
            download
            className="inline-flex items-center gap-2 rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-emerald-700 transition hover:border-emerald-300 hover:bg-emerald-100"
          >
            Download full enrollment template
          </a>
          <a
            href="/csv-templates/government-credentials.csv"
            download
            className="inline-flex items-center gap-2 rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-emerald-700 transition hover:border-emerald-300 hover:bg-emerald-100"
          >
            Download government credential template
          </a>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <form className="space-y-6" onSubmit={handleEnrollmentSubmit}>
          <header className="space-y-1">
            <h2 className="text-lg font-semibold text-slate-900">Full Enrollment Import</h2>
            <p className="text-sm text-slate-600">
              Includes device hierarchy assignments and optional government credential columns. CSV
              must contain the required columns
              <code className="mx-1">
                imei,stateId,stateAuthorityId,projectId,serverVendorId,protocolVersionId,solarPumpVendorId
              </code>
              and may include the optional government columns from the template.
            </p>
          </header>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-2 text-sm" htmlFor="full-enrollment-file">
              {formatLabel('Full enrollment CSV', enrollmentFile?.name ?? 'No file selected')}
              <input
                ref={enrollmentInputRef}
                id="full-enrollment-file"
                name="fullEnrollmentCsv"
                type="file"
                accept=".csv,text/csv"
                onChange={handleEnrollmentFileChange}
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              />
            </label>

            <label className="flex flex-col gap-2 text-sm" htmlFor="full-enrollment-issued-by">
              {formatLabel(
                'Issued by (optional)',
                'Stored in provisioning history for traceability',
              )}
              <input
                id="full-enrollment-issued-by"
                name="fullEnrollmentIssuedBy"
                type="text"
                value={enrollmentIssuedBy}
                onChange={(event) => setEnrollmentIssuedBy(event.target.value)}
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                placeholder="e.g. himadri-admin"
              />
            </label>
          </div>

          {isEnrollmentPreviewLoading && (
            <p className="text-xs text-slate-500" role="status">
              Generating preview…
            </p>
          )}

          {enrollmentPreviewError ? (
            <p className="text-sm text-rose-600" role="alert">
              {enrollmentPreviewError}
            </p>
          ) : null}

          {enrollmentPreview && !enrollmentPreviewError ? (
            <section className="space-y-3 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-800">
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <p className="font-semibold">
                    {enrollmentHasPreviewIssues
                      ? `⚠️ ${enrollmentInvalidRows.length} previewed rows have issues`
                      : '✅ All previewed rows look valid'}
                  </p>
                  <p className="mt-1 text-xs text-slate-600">
                    Showing first {enrollmentPreview.rows.length} rows (up to 100).
                  </p>
                  {enrollmentPreview.errors.length ? (
                    <ul className="mt-2 list-disc space-y-1 pl-5 text-xs text-amber-700">
                      {enrollmentPreview.errors.map((error) => (
                        <li key={error}>{error}</li>
                      ))}
                    </ul>
                  ) : null}
                </div>
                {enrollmentInvalidRows.length ? (
                  <button
                    type="button"
                    onClick={handleDownloadEnrollmentPreviewErrors}
                    className="inline-flex items-center justify-center rounded border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs font-medium text-amber-800 transition hover:border-amber-400 hover:bg-amber-200"
                  >
                    Download error report
                  </button>
                ) : null}
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-slate-200 text-xs">
                  <thead className="bg-slate-100 text-slate-700">
                    <tr>
                      <th className="px-3 py-2 text-left font-semibold">Row</th>
                      {enrollmentPreviewColumns.map((column) => (
                        <th key={column} className="px-3 py-2 text-left font-semibold">
                          {column}
                        </th>
                      ))}
                      <th className="px-3 py-2 text-left font-semibold">Errors</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200">
                    {enrollmentPreview.rows.map((row) => (
                      <tr
                        key={row.rowNumber}
                        className={row.errors.length ? 'bg-rose-50 text-rose-900' : ''}
                      >
                        <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-500">
                          {row.rowNumber}
                        </td>
                        {enrollmentPreviewColumns.map((column) => (
                          <td key={column} className="whitespace-nowrap px-3 py-2 align-top">
                            {row.data[column] ?? ''}
                          </td>
                        ))}
                        <td className="px-3 py-2 text-xs text-rose-700">
                          {row.errors.length ? row.errors.join('; ') : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}

          {enrollmentError ? (
            <p className="text-sm text-rose-600" role="alert">
              {enrollmentError}
            </p>
          ) : null}

          <button
            type="submit"
            className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2"
            disabled={enrollmentMutation.isPending}
          >
            {enrollmentMutation.isPending ? 'Uploading…' : 'Upload full enrollment CSV'}
          </button>

          {enrollmentResult ? (
            <section
              role="region"
              aria-labelledby="enrollment-import-summary-title"
              className="rounded border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800"
            >
              <p id="enrollment-import-summary-title" className="font-semibold">
                Enrollment import complete for {enrollmentCompletedFileName ?? 'uploaded CSV'}
              </p>
              <div className="mt-2 flex flex-wrap gap-3 text-xs text-emerald-700">
                <span>
                  Job ID{' '}
                  <code className="rounded bg-emerald-200 px-1 py-0.5 text-[0.65rem] text-emerald-900">
                    {enrollmentResult.jobId}
                  </code>
                </span>
                {enrollmentResult.stateId ? <span>State {enrollmentResult.stateId}</span> : null}
                {enrollmentResult.stateAuthorityId ? (
                  <span>Authority {enrollmentResult.stateAuthorityId}</span>
                ) : null}
                {enrollmentResult.projectId ? (
                  <span>Project {enrollmentResult.projectId}</span>
                ) : null}
              </div>
              <ul className="mt-2 space-y-1 text-xs">
                <li>Processed rows: {enrollmentResult.processed}</li>
                <li>Successfully enrolled: {enrollmentResult.enrolled}</li>
                <li>Failed rows: {enrollmentResult.failed}</li>
              </ul>
              {enrollmentResult.errors.length ? (
                <div className="mt-3 space-y-1">
                  <p className="font-medium">Row level issues</p>
                  <ul className="list-disc space-y-0.5 pl-5 text-xs">
                    {enrollmentResult.errors.map((error) => (
                      <li key={`${error.row}-${error.message}`}>
                        Row {error.row}: {error.message}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="mt-3 text-xs">No row level issues reported.</p>
              )}
            </section>
          ) : null}
        </form>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <form className="space-y-6" onSubmit={handleGovernmentSubmit}>
          <header className="space-y-1">
            <h2 className="text-lg font-semibold text-slate-900">Government Credential Import</h2>
            <p className="text-sm text-slate-600">
              Updates credential bundles for devices that already exist. Each row resolves the
              device by IMEI and replaces the active government credential set.
            </p>
            {!canImportGovernmentCredentials ? (
              <p className="text-xs text-slate-500">
                Requires <code className="rounded bg-slate-200 px-1 py-0.5 text-[0.7rem]">devices:credentials</code> capability.
              </p>
            ) : null}
          </header>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-2 text-sm" htmlFor="government-credential-file">
              {formatLabel('Government credential CSV', governmentFile?.name ?? 'No file selected')}
              <input
                ref={governmentInputRef}
                id="government-credential-file"
                name="governmentCredentialCsv"
                type="file"
                accept=".csv,text/csv"
                onChange={handleGovernmentFileChange}
                disabled={!canImportGovernmentCredentials}
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:opacity-60"
              />
            </label>

            <label className="flex flex-col gap-2 text-sm" htmlFor="government-issued-by">
              {formatLabel('Issued by (optional)', 'Stored alongside credential rotation history')}
              <input
                id="government-issued-by"
                name="governmentIssuedBy"
                type="text"
                value={governmentIssuedBy}
                onChange={(event) => setGovernmentIssuedBy(event.target.value)}
                disabled={!canImportGovernmentCredentials}
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:opacity-60"
                placeholder="e.g. maharashtra-msecdl"
              />
            </label>
          </div>

          {isGovernmentPreviewLoading && (
            <p className="text-xs text-slate-500" role="status">
              Generating preview…
            </p>
          )}

          {governmentPreviewError ? (
            <p className="text-sm text-rose-600" role="alert">
              {governmentPreviewError}
            </p>
          ) : null}

          {governmentPreview && !governmentPreviewError ? (
            <section className="space-y-3 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-800">
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <p className="font-semibold">
                    {governmentHasPreviewIssues
                      ? `⚠️ ${governmentInvalidRows.length} previewed rows have issues`
                      : '✅ All previewed rows look valid'}
                  </p>
                  <p className="mt-1 text-xs text-slate-600">
                    Showing first {governmentPreview.rows.length} rows (up to 100).
                  </p>
                  {governmentPreview.errors.length ? (
                    <ul className="mt-2 list-disc space-y-1 pl-5 text-xs text-amber-700">
                      {governmentPreview.errors.map((error) => (
                        <li key={error}>{error}</li>
                      ))}
                    </ul>
                  ) : null}
                </div>
                {governmentInvalidRows.length ? (
                  <button
                    type="button"
                    onClick={handleDownloadGovernmentPreviewErrors}
                    className="inline-flex items-center justify-center rounded border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs font-medium text-amber-800 transition hover:border-amber-400 hover:bg-amber-200"
                  >
                    Download error report
                  </button>
                ) : null}
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-slate-200 text-xs">
                  <thead className="bg-slate-100 text-slate-700">
                    <tr>
                      <th className="px-3 py-2 text-left font-semibold">Row</th>
                      {governmentPreviewColumns.map((column) => (
                        <th key={column} className="px-3 py-2 text-left font-semibold">
                          {column}
                        </th>
                      ))}
                      <th className="px-3 py-2 text-left font-semibold">Errors</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200">
                    {governmentPreview.rows.map((row) => (
                      <tr
                        key={row.rowNumber}
                        className={row.errors.length ? 'bg-rose-50 text-rose-900' : ''}
                      >
                        <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-500">
                          {row.rowNumber}
                        </td>
                        {governmentPreviewColumns.map((column) => (
                          <td key={column} className="whitespace-nowrap px-3 py-2 align-top">
                            {row.data[column] ?? ''}
                          </td>
                        ))}
                        <td className="px-3 py-2 text-xs text-rose-700">
                          {row.errors.length ? row.errors.join('; ') : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}

          {governmentError ? (
            <p className="text-sm text-rose-600" role="alert">
              {governmentError}
            </p>
          ) : null}

          <button
            type="submit"
            className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2"
            disabled={governmentMutation.isPending || !canImportGovernmentCredentials}
          >
            {governmentMutation.isPending ? 'Uploading…' : 'Upload government credentials CSV'}
          </button>

          {governmentResult ? (
            <section
              role="region"
              aria-labelledby="government-import-summary-title"
              className="rounded border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800"
            >
              <p id="government-import-summary-title" className="font-semibold">
                Government credential import complete for{' '}
                {governmentCompletedFileName ?? 'uploaded CSV'}
              </p>
              <div className="mt-2 flex flex-wrap gap-3 text-xs text-emerald-700">
                <span>
                  Job ID{' '}
                  <code className="rounded bg-emerald-200 px-1 py-0.5 text-[0.65rem] text-emerald-900">
                    {governmentResult.jobId}
                  </code>
                </span>
                {governmentResult.stateId ? <span>State {governmentResult.stateId}</span> : null}
                {governmentResult.stateAuthorityId ? (
                  <span>Authority {governmentResult.stateAuthorityId}</span>
                ) : null}
                {governmentResult.projectId ? (
                  <span>Project {governmentResult.projectId}</span>
                ) : null}
              </div>
              <ul className="mt-2 space-y-1 text-xs">
                <li>Processed rows: {governmentResult.processed}</li>
                <li>Updated bundles: {governmentResult.updated}</li>
                <li>Failed rows: {governmentResult.failed}</li>
              </ul>
              {governmentResult.errors.length ? (
                <div className="mt-3 space-y-1">
                  <p className="font-medium">Row level issues</p>
                  <ul className="list-disc space-y-0.5 pl-5 text-xs">
                    {governmentResult.errors.map((error) => (
                      <li key={`${error.row}-${error.message}`}>
                        Row {error.row}: {error.message}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="mt-3 text-xs">No row level issues reported.</p>
              )}
            </section>
          ) : null}
        </form>
      </section>
    </div>
  );
}
