import type {NextApiRequest, NextApiResponse} from "next";
import {siteUrl, socialImageUrl} from "../../src/imageMetadata";

export default function handler(req: NextApiRequest, res: NextApiResponse) {
  if (req.method !== "GET" && req.method !== "HEAD") {
    res.setHeader("Allow", "GET, HEAD");
    return res.status(405).end();
  }
  if (req.query.format !== undefined && req.query.format !== "json") {
    return res.status(501).end();
  }

  let page: URL;
  let image: string;
  let digest: string;
  try {
    if (typeof req.query.url !== "string" || req.query.url.length > 4096) throw new Error();
    page = new URL(req.query.url);
    if (page.origin !== siteUrl || page.username || page.password) throw new Error();
    const reference = decodeURIComponent(page.pathname.slice(1));
    const parts = reference.split("@");
    [image, digest = ""] = parts;
    if (parts.length > 2 || (parts.length === 2 && !/^sha256:[a-f0-9]{64}$/.test(digest)) ||
        (reference !== "" && (!image || image.length > 512 || !/^[a-zA-Z0-9][a-zA-Z0-9._:/-]*$/.test(image) ||
          image.includes("//") || /[/:]$/.test(image) || /^(api|_next)(\/|$)/.test(image)))) throw new Error();
  } catch {
    return res.status(404).end();
  }

  let scale = 1;
  for (const [key, size] of [["maxwidth", 1200], ["maxheight", 630]] as const) {
    const value = req.query[key];
    if (value === undefined) continue;
    if (typeof value !== "string" || !/^\d+$/.test(value) || !Number.isSafeInteger(Number(value)) || Number(value) < 1) {
      return res.status(400).end();
    }
    scale = Math.min(scale, Number(value) / size);
  }
  const themes = page.searchParams.getAll("theme");
  const theme = themes.length === 1 && (themes[0] === "light" || themes[0] === "dark") ? themes[0] : undefined;
  // Slack prefers oEmbed to OG/Twitter metadata. A photo without optional text
  // fields requests an image-only unfurl while other clients retain their tags.
  // Only construct our own image URL: never fetch the supplied page or index it.
  const photo = {
    version: "1.0",
    type: "photo",
    url: socialImageUrl(image, digest, theme),
    width: Math.max(1, Math.floor(1200 * scale)),
    height: Math.max(1, Math.floor(630 * scale)),
  };
  res.setHeader("Cache-Control", "public, max-age=3600, s-maxage=86400");
  res.setHeader("Content-Type", "application/json");
  res.setHeader("X-Content-Type-Options", "nosniff");
  if (req.method === "HEAD") return res.status(200).end();
  return res.status(200).json(photo);
}
