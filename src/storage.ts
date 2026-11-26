import { EmailTemplate } from './types';

/**
 * Grafana stores at most this many bytes of a plugin's jsonData (the size of a
 * MySQL TEXT column, applied on every database).
 */
export const MAX_SETTINGS_BYTES = 65535;

function toBase64(bytes: Uint8Array): string {
  let s = '';
  for (let i = 0; i < bytes.length; i += 0x8000) {
    s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(s);
}

function fromBase64(b64: string): Uint8Array {
  const s = atob(b64);
  const out = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) {
    out[i] = s.charCodeAt(i);
  }
  return out;
}

async function pipe(data: Uint8Array, stream: CompressionStream | DecompressionStream): Promise<Uint8Array> {
  const res = new Response(new Blob([data as BlobPart]).stream().pipeThrough(stream));
  return new Uint8Array(await res.arrayBuffer());
}

/** Templates as gzip-compressed JSON in base64, the format the backend reads from templatesGz. */
export async function encodeTemplates(templates: EmailTemplate[]): Promise<string> {
  const json = new TextEncoder().encode(JSON.stringify(templates));
  return toBase64(await pipe(json, new CompressionStream('gzip')));
}

export async function decodeTemplates(b64: string): Promise<EmailTemplate[]> {
  const json = await pipe(fromBase64(b64), new DecompressionStream('gzip'));
  return JSON.parse(new TextDecoder().decode(json));
}

/** Bytes Grafana counts for a jsonData object. */
export function settingsBytes(jsonData: unknown): number {
  return new TextEncoder().encode(JSON.stringify(jsonData)).length;
}

export class SettingsTooLargeError extends Error {
  constructor(public bytes: number) {
    super(
      `Templates and settings take ${Math.ceil(bytes / 1024)} KB, but Grafana stores at most 64 KB per plugin. ` +
        'Delete a template you no longer use, or shorten long ones, then save again.'
    );
  }
}
