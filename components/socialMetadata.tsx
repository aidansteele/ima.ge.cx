import Head from "next/head";

export const siteUrl = "https://ima.ge.cx";
export const siteTitle = "ima.ge.cx · See inside any container image";
export const siteDescription = "Browse the files, layers and configuration of public Docker and OCI container images without pulling them or running Docker.";

export function SocialMetadata({title = siteTitle, description = siteDescription, path = "/", imageReference, digest = ""}: {
  title?: string;
  description?: string;
  path?: string;
  imageReference?: string;
  digest?: string;
}) {
  const url = `${siteUrl}${path}`;
  const image = imageReference ? `${siteUrl}/api/og?${new URLSearchParams({image: imageReference, digest, v: "1"})}` : `${siteUrl}/og-image.jpg`;
  const imageAlt = imageReference ? `${imageReference} — filesystem preview on ima.ge.cx` :
    "ima.ge.cx — See inside any container image. Files, layers & config. No Docker required.";

  return <Head>
    <title>{title}</title>
    <meta name="description" content={description} key="description"/>
    <link rel="canonical" href={url} key="canonical"/>
    <meta property="og:type" content="website" key="og:type"/>
    <meta property="og:site_name" content="ima.ge.cx" key="og:site_name"/>
    <meta property="og:title" content={title} key="og:title"/>
    <meta property="og:description" content={description} key="og:description"/>
    <meta property="og:url" content={url} key="og:url"/>
    <meta property="og:image" content={image} key="og:image"/>
    <meta property="og:image:type" content={imageReference ? "image/png" : "image/jpeg"} key="og:image:type"/>
    <meta property="og:image:width" content="1200" key="og:image:width"/>
    <meta property="og:image:height" content="630" key="og:image:height"/>
    <meta property="og:image:alt" content={imageAlt} key="og:image:alt"/>
    <meta name="twitter:card" content="summary_large_image" key="twitter:card"/>
    <meta name="twitter:title" content={title} key="twitter:title"/>
    <meta name="twitter:description" content={description} key="twitter:description"/>
    <meta name="twitter:image" content={image} key="twitter:image"/>
    <meta name="twitter:image:alt" content={imageAlt} key="twitter:image:alt"/>
  </Head>;
}
