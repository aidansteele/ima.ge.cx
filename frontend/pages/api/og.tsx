import React from "react";
import type {NextApiRequest, NextApiResponse} from "next";
import {ImageResponse} from "@vercel/og";
import {readFile} from "node:fs/promises";
import path from "node:path";
import {ImageSocialCard} from "../../components/imageSocialCard";
import {readFilesystemSummary, readImageInfo} from "../../src/imageMetadata";

// Bundled fonts keep rendering independent of system fonts and external services.
const fonts = Promise.all([
  readFile(path.join(process.cwd(), "assets/Inter-Regular.ttf")),
  readFile(path.join(process.cwd(), "assets/Inter-Bold.ttf")),
]);

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  if (req.method !== "GET" && req.method !== "HEAD") {
    res.setHeader("Allow", "GET, HEAD");
    return res.status(405).end();
  }
  const {image, digest = "", theme = "dark"} = req.query;
  // Bound rendering work and only accept reference characters, not arbitrary
  // text, URLs or user-provided statistics. A registry port is allowed.
  if (typeof image !== "string" || image.length > 512 || !/^[a-zA-Z0-9][a-zA-Z0-9._:/-]*$/.test(image) ||
      image.includes("//") || /[/:]$/.test(image) ||
      typeof digest !== "string" || (digest !== "" && !/^sha256:[a-f0-9]{64}$/.test(digest))) {
    return res.status(400).send("Invalid image reference or digest");
  }
  if (theme !== "light" && theme !== "dark") {
    return res.status(400).send("Invalid theme");
  }

  const info = await readImageInfo(image, digest);
  const filesystem = info ? await readFilesystemSummary(image, digest) : undefined;
  const [regular, bold] = await fonts;
  const png = new ImageResponse(<ImageSocialCard image={image} digest={digest} info={info} filesystem={filesystem} theme={theme}/>, {
    width: 1200, height: 630,
    fonts: [
      {name: "Inter", data: regular, weight: 400, style: "normal"},
      {name: "Inter", data: bold, weight: 700, style: "normal"},
    ],
  });
  // Pages Router's Node runtime needs a NextApiResponse rather than Response.
  const body = Buffer.from(await png.arrayBuffer());
  res.setHeader("Content-Type", "image/png");
  // Don't pin a missing/pending index in a CDN's cache after indexing completes.
  res.setHeader("Cache-Control", filesystem ? "public, max-age=3600, s-maxage=86400" : "no-store");
  res.setHeader("X-Content-Type-Options", "nosniff");
  res.status(200).send(req.method === "HEAD" ? undefined : body);
}
