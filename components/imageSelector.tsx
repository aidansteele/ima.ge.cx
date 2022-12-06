import {useRouter} from "next/router";
import {useImageOptions} from "../src/imageHooks";
import {Loader, Select} from "@mantine/core";

export function ImageSelector({image, digest}: { image: string, digest?: string }) {
  const router = useRouter();

  const {data, error} = useImageOptions(image);
  if (error) {
    return <div>error loading image digest a: {error}</div>;
  }

  if (!data) {
    return <Loader/>;
  }

  if (data.Error) {
    return <div>error loading image digest a: {data.Error}</div>;
  }

  if (data.Options!.length == 1 && digest != data.Options![0].Digest) {
    digest = data.Options![0].Digest;
    router.push(`/${image}@${data.Options![0].Digest}`);
  }

  const selectData = data.Options!.map(o => ({
    value: o.Digest,
    label: `${JSON.stringify(o.Platform)}`,
    group: o.Platform.os
  }));

  return <Select
    label={"Image variant"}
    data={selectData}
    defaultValue={digest}
    onChange={val => router.push(`/${image}@${val}`)}
  />
}
