import type {ApiResponseDirectoryEntry, ImageInfo} from "./imageHooks";
import {formatBytes} from "./formatBytes";

export const siteUrl = "https://ima.ge.cx";

export function socialImageUrl(image: string, digest = "", theme?: "light" | "dark") {
  if (!image) return `${siteUrl}/og-image${theme === "light" ? "-light" : ""}.jpg`;
  const params = new URLSearchParams({image, digest, v: "1"});
  if (theme) params.set("theme", theme);
  return `${siteUrl}/api/og?${params}`;
}

// Only use read-only endpoints here: crawlers must never start indexing jobs.
async function fetchMetadata<T>(endpoint: "metadata" | "dir", params: Record<string, string>): Promise<T | undefined> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 2000);
  try {
    const base = process.env.NEXT_PUBLIC_API_BASE_URL || "https://api.ima.ge.cx/api/";
    const response = await fetch(`${base}${endpoint}?${new URLSearchParams(params)}`, {signal: controller.signal});
    return response.ok ? await response.json() : undefined;
  } catch {
    return undefined;
  } finally {
    clearTimeout(timeout);
  }
}

export async function readImageInfo(image: string, digest: string): Promise<ImageInfo | undefined> {
  if (!digest) return undefined;
  const info = await fetchMetadata<ImageInfo>("metadata", {image, digest});
  // Pending/failed indexes can have incomplete config and size information.
  return info?.Status === "SUCCEEDED" ? info : undefined;
}

export interface FilesystemSummary {
  bytes: number;
  files: number;
  directories: {path: string; bytes: number}[];
}

export async function readFilesystemSummary(image: string, digest: string): Promise<FilesystemSummary | undefined> {
  const entries = await fetchMetadata<ApiResponseDirectoryEntry[] | null>("dir", {image, digest, path: "/"});
  if (entries === undefined) return undefined;
  // An empty index can be encoded as null by the Go API. Exclude the root
  // itself: its recursive totals would otherwise double-count its children.
  const children = (entries ?? []).filter(e => e.Hdr.Name !== "/");
  const directories = children.filter(e => e.Hdr.Typeflag === 53);
  const regularFiles = children.filter(e => e.Hdr.Typeflag === 48 || e.Hdr.Typeflag === 0);
  // The API omits zero-valued directory totals. Links don't contribute bytes.
  return {
    bytes: regularFiles.reduce((n, e) => n + e.Hdr.Size, 0) + directories.reduce((n, e) => n + (e.DirSize ?? 0), 0),
    files: regularFiles.length + directories.reduce((n, e) => n + (e.DirFiles ?? 0), 0),
    directories: directories.map(e => ({path: `/${e.Hdr.Name.replace(/^\/+|\/+$/g, "")}`, bytes: e.DirSize ?? 0}))
      .sort((a, b) => b.bytes - a.bytes || a.path.localeCompare(b.path)).slice(0, 3),
  };
}

export async function imageDescription(image: string, digest: string): Promise<string> {
  const intro = `Explore ${image}${digest ? `@${digest}` : ""} on ima.ge.cx.`;
  const browse = "Browse files, layers, environment variables and image configuration without Docker.";
  const info = await readImageInfo(image, digest);
  if (!info) return `${intro} ${browse}`;

  const platform = [info.Config?.os, info.Config?.architecture, info.Config?.variant].filter(Boolean).join("/");
  const layers = info.Manifest?.layers;
  const size = info.TotalSize ?? (layers ? layers.reduce((sum, layer) => sum + layer.size, 0) : undefined);
  const details = [
    platform,
    layers ? `${layers.length} ${layers.length === 1 ? "layer" : "layers"}` : "",
    size !== undefined ? `${formatBytes(size, 1).trim()} compressed` : "",
  ].filter(Boolean).join(" · ");

  // Put the useful facts first so previews don't truncate them after a digest.
  return `${image}${details ? ` — ${details}` : ""}. ${browse} Digest: ${digest}.`;
}
