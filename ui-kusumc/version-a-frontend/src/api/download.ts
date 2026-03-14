import { apiFetch, readJsonBody } from './http';

function inferFilenameFromDisposition(disposition: string, fallback: string): string {
  if (!disposition) return fallback;

  const encodedMatch = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (encodedMatch && encodedMatch[1]) {
    try {
      return decodeURIComponent(encodedMatch[1]);
    } catch {
      return encodedMatch[1];
    }
  }

  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  if (plainMatch && plainMatch[1]) {
    return plainMatch[1];
  }

  return fallback;
}

export async function downloadWithAuth(args: {
  url: string;
  filenameFallback: string;
  accept?: string;
}): Promise<void> {
  const response = await apiFetch(args.url, {
    headers: {
      Accept: args.accept ?? '*/*',
    },
  });

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? '';
    let message = 'Download failed';

    if (contentType.includes('application/json')) {
      const body = await readJsonBody<any>(response);
      message = body?.message ?? body?.error ?? message;
    } else {
      const text = await response.text().catch(() => null);
      if (typeof text === 'string' && text.trim()) {
        message = text.trim();
      }
    }

    throw new Error(message);
  }

  const blob = await response.blob();
  const disposition = response.headers.get('content-disposition') ?? '';
  const filename = inferFilenameFromDisposition(disposition, args.filenameFallback);

  const objectUrl = URL.createObjectURL(blob);
  try {
    const link = document.createElement('a');
    link.href = objectUrl;
    link.download = filename;
    link.rel = 'noopener noreferrer';
    document.body.appendChild(link);
    link.click();
    link.remove();
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
