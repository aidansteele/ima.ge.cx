// Keep automatic text decoding and CodeMirror work small, independently of downloads.
export const MaxPreviewBytes = 1024 * 1024;
// The API base64-encodes buffered responses into Lambda's 6 MB response payload.
// Leave room for that encoding and the response envelope; never save a partial file.
export const MaxDownloadBytes = 4 * 1024 * 1024;

export async function fetchFile(url: string, maxBytes: number, signal?: AbortSignal): Promise<Blob> {
  const res = await fetch(url, {signal});
  if (!res.ok) {
    await res.body?.cancel();
    throw new Error(`HTTP ${res.status}${res.statusText ? ` ${res.statusText}` : ""}`);
  }
  const tooLarge = () => new Error(`File exceeds the ${maxBytes / (1024 * 1024)} MiB limit.`);
  if (Number(res.headers.get("Content-Length")) > maxBytes) {
    await res.body?.cancel();
    throw tooLarge();
  }
  if (!res.body) return new Blob();

  // Also bound chunked responses and stale/incorrect directory metadata. Do not
  // use response.text()/blob(): those allocate the entire response before checking.
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxBytes) throw tooLarge();
      chunks.push(value);
    }
    return new Blob(chunks);
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
}
