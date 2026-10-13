import {useEffect} from "react";
import {useRouter} from "next/router";
import {Platform, useImageOptions} from "../src/imageHooks";
import {Loader, Select, Text} from "@mantine/core";

function platformLabel(p: Platform) {
  const label = [p.os, p.architecture, p.variant].filter(Boolean).join("/");
  return p['os.version'] ? `${label} (${p['os.version']})` : label;
}

export function ImageSelector({image, digest}: { image: string, digest?: string }) {
  const router = useRouter();
  const {data, error} = useImageOptions(image);

  // Go straight to the only variant. Replace rather than push so that the
  // back button doesn't return here and redirect again.
  const onlyDigest = data?.Options?.length === 1 ? data.Options[0].Digest : undefined;
  useEffect(() => {
    if (onlyDigest && onlyDigest !== digest) {
      // Keep the hash so that links to a file in a single-platform image work without a digest.
      router.replace(`/${image}@${onlyDigest}${window.location.hash}`);
    }
  }, [onlyDigest, digest, image, router]);

  if (error) {
    return <Text color="red">Error looking up image: {error.message}</Text>;
  }

  if (!data) {
    return <Loader/>;
  }

  if (data.Error) {
    return <Text color="red">Error looking up image: {data.Error}</Text>;
  }

  const selectData = (data.Options ?? []).map(o => ({
    value: o.Digest,
    label: platformLabel(o.Platform),
    group: o.Platform.os
  }));

  return <Select
    label={"Image variant"}
    placeholder={"Select a platform"}
    data={selectData}
    value={digest ?? null}
    // Keep the hash so the same file is shown for the other platform.
    onChange={val => router.push(`/${image}@${val}${window.location.hash}`)}
  />
}
