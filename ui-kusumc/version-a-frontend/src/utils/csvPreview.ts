import Papa, { ParseError, ParseMeta, ParseResult } from 'papaparse';

export type CsvPreviewRow<T extends Record<string, unknown> = Record<string, unknown>> = {
  rowNumber: number;
  data: T;
  errors: string[];
};

export type CsvPreviewResult<T extends Record<string, unknown>> = {
  rows: CsvPreviewRow<T>[];
  errors: string[];
  header: string[];
  meta: ParseMeta;
};

export type CsvPreviewOptions<T extends Record<string, unknown>> = {
  delimiter?: string;
  previewRowLimit?: number;
  skipEmptyLines?: boolean | 'greedy';
  validateRow?: (row: T, context: { rowNumber: number; meta: ParseMeta }) => string[];
  validateFile?: (context: {
    meta: ParseMeta;
    errors: ParseError[];
    rows: CsvPreviewRow<T>[];
  }) => string[];
};

const DEFAULT_PREVIEW_ROWS = 100;

export async function parseCsvPreview<T extends Record<string, unknown>>(
  file: File,
  options: CsvPreviewOptions<T> = {},
): Promise<CsvPreviewResult<T>> {
  const {
    delimiter,
    previewRowLimit = DEFAULT_PREVIEW_ROWS,
    skipEmptyLines = true,
    validateRow,
    validateFile,
  } = options;

  const text =
    typeof file.text === 'function' ? await file.text() : await new Response(file).text();
  const rows: CsvPreviewRow<T>[] = [];
  const fileErrors: string[] = [];

  const parseResult = await new Promise<{ data: T[]; meta: ParseMeta; errors: ParseError[] }>(
    (resolve, reject) => {
      Papa.parse<T>(text, {
        header: true,
        delimiter,
        skipEmptyLines,
        preview: previewRowLimit,
        complete: (result: ParseResult<T>) => {
          resolve({ data: result.data as T[], meta: result.meta, errors: result.errors });
        },
        error: (error: Error) => reject(error),
        transformHeader: (header: string) => header.trim(),
      });
    },
  );

  const relevantParseErrors = parseResult.errors.filter(
    (error) => error.code !== 'UndetectableDelimiter',
  );

  relevantParseErrors.forEach((error) => {
    const location = typeof error.row === 'number' ? `row ${error.row}` : 'unknown row';
    fileErrors.push(`${location}: ${error.message}`);
  });

  parseResult.data.forEach((row, index) => {
    const rowNumber = index + 2; // account for 1-indexed rows + header row
    const rowErrors = validateRow ? validateRow(row, { rowNumber, meta: parseResult.meta }) : [];
    rows.push({ rowNumber, data: row, errors: rowErrors });
  });

  if (validateFile) {
    fileErrors.push(
      ...validateFile({
        meta: parseResult.meta,
        errors: relevantParseErrors,
        rows,
      }),
    );
  }

  const header = parseResult.meta.fields ?? [];

  return {
    rows,
    errors: fileErrors,
    header,
    meta: parseResult.meta,
  };
}
