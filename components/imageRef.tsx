import {useEffect, useMemo} from "react";
import {useRouter} from "next/router";
import {Select, Tooltip} from "@mantine/core";
import {IconBox} from "@tabler/icons";
import {Platform, useImageOptions, useImageTags} from "../src/imageHooks";
import {sortTags, splitTag} from "../src/tags";
import {useTheme} from "../src/theme";

export function platformLabel(p: Platform) {
  const label = [p.os, p.architecture, p.variant].filter(Boolean).join("/");
  return p['os.version'] ? `${label} (${p['os.version']})` : label;
}

const compactSelect = {
  input: {fontFamily: "var(--mono)", fontSize: 13, height: 34, minHeight: 34},
};

// The image reference as three controls: repository, tag and platform.
export function ImageRef({image, digest}: { image: string, digest?: string }) {
  const router = useRouter();
  const {themedPath} = useTheme();
  const {repo, tag} = splitTag(image);
  const hash = () => window.location.hash;

  const {data: tagData, error: tagError} = useImageTags(repo);
  const tags = useMemo(() => sortTags(tagData?.Tags ?? []), [tagData]);

  const {data: options, error: optionsError} = useImageOptions(image);

  // Go straight to the only variant. Replace rather than push so that the
  // back button doesn't return here and redirect again.
  const onlyDigest = options?.Options?.length === 1 ? options.Options[0].Digest : undefined;
  useEffect(() => {
    if (onlyDigest && onlyDigest !== digest) {
      router.replace(themedPath(`/${image}@${onlyDigest}${window.location.hash}`));
    }
  }, [onlyDigest, digest, image, router, themedPath]);

  const platforms = (options?.Options ?? []).map(o => ({value: o.Digest, label: platformLabel(o.Platform)}));
  // A link can pin a digest that the tag no longer points to.
  if (digest && !platforms.some(p => p.value === digest)) {
    platforms.unshift({value: digest, label: `${digest.slice(0, 19)}…`});
  }

  const tagFailed = tagError?.message ?? tagData?.Error;
  const platformFailed = optionsError?.message ?? options?.Error;

  return <div className="image-ref">
    <span className="image-ref-repo" title={repo}><IconBox size={16}/>{repo}</span>
    <span className="image-ref-sep">:</span>
    <Tooltip label={tagFailed} disabled={!tagFailed} withinPortal>
      <Select
        aria-label="Tag"
        className="image-tag"
        styles={compactSelect}
        placeholder={tagFailed ? "tags unavailable" : tagData ? "tag" : "loading tags…"}
        searchable
        // Rendering thousands of options is slow; searching narrows the list.
        limit={200}
        nothingFound="No matching tags"
        data={tags}
        value={tag ?? null}
        // Drop the digest so the platform list is looked up for the new tag,
        // but keep the hash so the same file is shown if it exists there.
        onChange={val => router.push(themedPath(`/${repo}:${val}${hash()}`))}
      />
    </Tooltip>
    <span className="image-ref-sep">·</span>
    <Tooltip label={platformFailed} disabled={!platformFailed} withinPortal>
      <Select
        aria-label="Platform"
        className="image-platform"
        styles={compactSelect}
        placeholder={platformFailed ? "lookup failed" : options ? "platform" : "loading…"}
        data={platforms}
        value={digest ?? null}
        // Keep the hash so the same file is shown for the other platform.
        onChange={val => router.push(themedPath(`/${image}@${val}${hash()}`))}
      />
    </Tooltip>
  </div>;
}
