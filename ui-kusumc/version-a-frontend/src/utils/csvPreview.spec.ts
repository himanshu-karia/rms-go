import { describe, expect, it } from 'vitest';

import { parseCsvPreview } from './csvPreview';

const createFile = (content: string, name = 'preview.csv') => {
  const file = new File([content], name, { type: 'text/csv' });
  if (typeof file.text !== 'function') {
    Object.assign(file, {
      text: async () => content,
    });
  }
  return file;
};

describe('parseCsvPreview', () => {
  it('parses up to the configured preview limit and trims header names', async () => {
    const file = createFile('imei , status\n123,active\n456,inactive\n789,online');

    const result = await parseCsvPreview(file, { previewRowLimit: 2 });

    expect(result.header).toEqual(['imei', 'status']);
    expect(result.rows).toHaveLength(2);
    expect(result.rows[0].rowNumber).toBe(2);
    expect(result.rows[1].rowNumber).toBe(3);
  });

  it('runs row validation and aggregates row errors', async () => {
    const file = createFile('imei,stateId\n,maharashtra\n123,');

    const result = await parseCsvPreview(file, {
      validateRow: (row) => {
        const errors: string[] = [];
        if (!row.imei) {
          errors.push('IMEI required');
        }
        if (!row.stateId) {
          errors.push('State required');
        }
        return errors;
      },
    });

    expect(result.rows).toHaveLength(2);
    expect(result.rows[0].errors).toEqual(['IMEI required']);
    expect(result.rows[1].errors).toEqual(['State required']);
  });

  it('runs file-level validation with parser metadata and errors', async () => {
    const file = createFile('imei\n123\n');

    const result = await parseCsvPreview(file, {
      validateFile: ({ meta }) => {
        return meta.fields?.includes('imei') ? [] : ['Missing imei column'];
      },
    });

    expect(result.errors).toEqual([]);
  });
});
