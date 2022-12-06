import useSWR from "swr";
import useSWRImmutable from "swr/immutable";

//@ts-ignore
const fetcher = (...args) => fetch(...args).then(res => res.json())

const ApiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL || 'https://api.ima.ge.cx/api/';

export function useFileContents(image: string, digest: string, path: string) {
  const usp = new URLSearchParams();
  usp.set('image', image);
  usp.set('digest', digest)
  usp.set('path', path);

  const key = `${ApiBaseUrl}file?${usp}`;
  const {
    data,
    error,
    isLoading
  } = useSWRImmutable(path == "" ? null : key, (...args) => fetch(...args).then(res => res.text()));

  return {
    data: data as string | null,
    error,
    isLoading,
  }
}

export interface DirectoryEntry {
  id: string;
  path: string;
  // parent: string;
  size: number;
  typ: number;
  linkname: string;
  deletion: boolean;
  layer: string;

  // Mode: number;
  // Uid: number;
  // Gid: number;
  // Uname: string;
  // Gname: string;
  // ModTime: Date;
  // AccessTime: Date;
  // ChangeTime: Date;
  // Devmajor: number;
  // Devminor: number;
  // Xattrs?: any;
  // PAXRecords?: any;
  // Format: number;
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
    Mode: number;
    Uid: number;
    Gid: number;
    Uname: string;
    Gname: string;
    ModTime: Date;
    AccessTime: Date;
    ChangeTime: Date;
    Devmajor: number;
    Devminor: number;
    Xattrs?: any;
    PAXRecords?: any;
    Format: number;
  };
}

export function useDirectory(image: string, digest: string, path: string, load: boolean) {
  const usp = new URLSearchParams();
  usp.set('image', image);
  usp.set('digest', digest)
  usp.set('path', path);

  const key = `${ApiBaseUrl}dir?${usp}`;
  const {data, error, isLoading} = useSWRImmutable(load ? key : null, fetcher);

  const dirents: DirectoryEntry[] = (data || []).map((src: ApiResponseDirectoryEntry): DirectoryEntry => {
    return {
      id: `${src.Offset}-${src.Layer}`,
      path: src.Hdr.Name,
      // parent: src.Parent,
      size: src.Hdr.Size,
      typ: src.Hdr.Typeflag,
      linkname: src.Hdr.Linkname,
      deletion: false,
      layer: src.Layer,
    };
  })

  return {
    dirents,
    error,
    isLoading,
  }
}

export function useImageInfo(image: string, digest: string) {
  const usp = new URLSearchParams();
  usp.set('image', image);
  usp.set('digest', digest);
  const key = `${ApiBaseUrl}info?${usp}`;

  const {data, error, isLoading} = useSWR(key, fetcher, {
    refreshInterval: (data) => {
      if (!data) return 1000;
      switch (data.Status) {
        case "PENDING":
        case "RUNNING":
          return 1000;
        case "SUCCEEDED":
        case "FAILED":
        default:
          return 0;
      }
    }
  });

  return {
    data,
    error,
    isLoading,
  }
}

interface ImageOptionsResponse {
  Options?: ImageOption[];
  Error?: string;
}

interface ImageOption {
  Digest: string;
  Platform: Platform;
}

interface Platform {
  architecture: string;
  os: string;

  [key: string]: string;
}

export function useImageOptions(image: string) {
  const usp = new URLSearchParams();
  usp.set('image', image);
  const key = `${ApiBaseUrl}lookup?${usp}`;

  const {data, error, isLoading} = useSWR(key, fetcher);

  return {
    data: data as ImageOptionsResponse | null,
    error,
    isLoading,
  }
}
