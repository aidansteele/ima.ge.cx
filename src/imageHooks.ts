import useSWR from "swr";
import useSWRImmutable from "swr/immutable";

const ApiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL || 'https://api.ima.ge.cx/api/';

async function fetchOk(url: string): Promise<Response> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = (await res.text()).trim();
    throw new Error(`${body || res.statusText} (HTTP ${res.status})`);
  }
  return res;
}

const fetchJson = (url: string) => fetchOk(url).then(res => res.json());
const fetchText = (url: string) => fetchOk(url).then(res => res.text());

function apiUrl(endpoint: string, params: Record<string, string>) {
  return `${ApiBaseUrl}${endpoint}?${new URLSearchParams(params)}`;
}

export function useFileContents(image: string, digest: string, path: string) {
  const key = apiUrl('file', {image, digest, path});
  return useSWRImmutable<string, Error>(path === "" ? null : key, fetchText);
}

export interface DirectoryEntry {
  id: string;
  path: string;
  size: number;
  typ: number;
  linkname: string;
  layer: string;
}

interface ApiResponseDirectoryEntry {
  Offset: number;
  Spans: number[];
  Parent: string;
  Layer: string;
  Hdr: {
    Typeflag: number;
    Name: string;
    Linkname: string;
    Size: number;
  };
}

export function useDirectory(image: string, digest: string, path: string, load: boolean) {
  const key = apiUrl('dir', {image, digest, path});
  const {data, error, isLoading} = useSWRImmutable<ApiResponseDirectoryEntry[], Error>(load ? key : null, fetchJson);

  const dirents: DirectoryEntry[] = (data ?? []).map(src => ({
    id: `${src.Offset}-${src.Layer}`,
    path: src.Hdr.Name,
    size: src.Hdr.Size,
    typ: src.Hdr.Typeflag,
    linkname: src.Hdr.Linkname,
    layer: src.Layer,
  }));

  return {dirents, error, isLoading};
}

export type ImageInfoStatus = "PENDING" | "RUNNING" | "SUCCEEDED" | "FAILED";

export interface LayerProgress {
  Layer: string;
  TotalBytes: number;
  CompletedBytes: number;
  TotalFiles: number;
  CompletedFiles: number;
}

export interface HistoryEntry {
  created?: string;
  created_by?: string;
  comment?: string;
  empty_layer?: boolean;
}

// The image config JSON, see https://github.com/opencontainers/image-spec/blob/main/config.md
export interface ImageConfig {
  config?: {
    Env?: string[] | null;
  };
  history?: HistoryEntry[];
}

export interface ImageManifest {
  layers?: {
    mediaType: string;
    digest: string;
    size: number;
  }[];
}

export interface ImageInfo {
  Status: ImageInfoStatus;
  Progresses?: LayerProgress[];
  TotalSize?: number;
  CompletedSize?: number;
  Config?: ImageConfig;
  Manifest?: ImageManifest;
}

export function useImageInfo(image: string, digest: string) {
  const key = apiUrl('info', {image, digest});

  return useSWR<ImageInfo, Error>(key, fetchJson, {
    // Poll until indexing has finished.
    refreshInterval: (data) => !data || data.Status === "PENDING" || data.Status === "RUNNING" ? 1000 : 0,
  });
}

interface ImageOptionsResponse {
  Options?: ImageOption[];
  Error?: string;
}

interface ImageOption {
  Digest: string;
  Platform: Platform;
}

export interface Platform {
  architecture: string;
  os: string;
  variant?: string;
  'os.version'?: string;
}

export function useImageOptions(image: string) {
  const key = apiUrl('lookup', {image});
  return useSWR<ImageOptionsResponse, Error>(key, fetchJson);
}

interface ImageTagsResponse {
  Tags?: string[];
  Error?: string;
}

export function useImageTags(repo: string) {
  const key = apiUrl('tags', {image: repo});
  // Tags change rarely, and large repositories have thousands of them.
  return useSWRImmutable<ImageTagsResponse, Error>(key, fetchJson);
}
