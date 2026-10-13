import {useMemo} from "react";
import {useRouter} from "next/router";
import {Loader, Select, Text} from "@mantine/core";
import {useImageTags} from "../src/imageHooks";

// Splits an image reference such as "ghcr.io/org/app:v1" into its repository
// and tag. A colon before the last slash belongs to a registry port.
export function splitTag(image: string): { repo: string, tag?: string } {
  const i = image.lastIndexOf(":");
  return i > image.lastIndexOf("/") ? {repo: image.slice(0, i), tag: image.slice(i + 1)} : {repo: image};
}

const numericCollator = new Intl.Collator(undefined, {numeric: true});

// Newest-looking versions first (so 1.22 comes before 1.9), with "latest" at the top.
function sortTags(tags: string[]) {
  return [...tags].sort((a, b) =>
    Number(b === "latest") - Number(a === "latest") || numericCollator.compare(b, a));
}

export function TagSelector({image}: { image: string }) {
  const router = useRouter();
  const {repo, tag} = splitTag(image);
  const {data, error} = useImageTags(repo);
  const tags = useMemo(() => sortTags(data?.Tags ?? []), [data]);

  if (error || data?.Error) {
    return <Text color="red">Error listing tags: {error?.message ?? data?.Error}</Text>;
  }

  if (!data) {
    return <Loader/>;
  }

  return <Select
    label={"Tag"}
    placeholder={`Search ${tags.length} tags`}
    searchable
    // Rendering thousands of options is slow; searching narrows the list.
    limit={200}
    nothingFound={"No matching tags"}
    data={tags}
    value={tag ?? null}
    // Drop the digest so the variant list is looked up for the new tag, but
    // keep the hash so the same file is shown if it exists there.
    onChange={val => router.push(`/${repo}:${val}${window.location.hash}`)}
  />;
}
