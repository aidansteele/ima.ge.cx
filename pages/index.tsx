import type {NextPage} from 'next'
import {CSSProperties, FormEvent, useState} from "react";
import Head from "next/head";
import Link from "next/link";
import {useRouter} from "next/router";
import {Autocomplete, Button} from "@mantine/core";
import {IconArrowRight, IconFileText, IconSearch, IconStack2, IconLink} from "@tabler/icons";
import {TopBar} from "../components/topBar";
import {colorFor} from "../components/layerChip";
import {useImageOptions, useImageTags} from "../src/imageHooks";
import {rankTagSuggestions, registryOf, splitTag} from "../src/tags";

const examples = [
  {image: "ubuntu", description: "Ubuntu base image"},
  {image: "golang:1.19", description: "Go toolchain on Debian"},
  {image: "jetbrains/teamcity-server:2022.10", description: "JetBrains TeamCity server"},
  {image: "mcr.microsoft.com/dotnet/sdk:6.0", description: "Microsoft .NET SDK"},
];

function ExampleCard({image, description}: { image: string, description: string }) {
  const {data} = useImageOptions(image);
  const {repo, tag} = splitTag(image);
  const name = repo.split("/").pop()!;
  const arches = Array.from(new Set((data?.Options ?? []).map(o => o.Platform.architecture))).slice(0, 4);

  const registry = registryOf(repo);
  const namespace = repo.split("/").slice(registry === "docker.io" ? 0 : 1, -1).join("/");

  return <Link href={`/${image}`} className="example">
    <span className="monogram" style={{"--chip": colorFor(name)} as CSSProperties}>{name[0].toUpperCase()}</span>
    <span className="example-body">
      <span className="example-name" title={image}>{name}</span>
      <span className="example-registry">{[registry, namespace].filter(Boolean).join(" / ")}</span>
      <span className="example-desc">{description}</span>
      <span className="pills">
        <span className="pill tag">{tag ?? "latest"}</span>
        {arches.map(a => <span className="pill" key={a}>{a}</span>)}
      </span>
    </span>
  </Link>;
}

function Search() {
  const router = useRouter();
  const [value, setValue] = useState("");
  const {repo, tag} = splitTag(value.trim());
  // Suggest tags once the user has typed "repo:".
  const wantsTags = tag !== undefined && repo.length > 0;
  const {data} = useImageTags(wantsTags ? repo : null);
  const suggestions = wantsTags ?
    rankTagSuggestions((data?.Tags ?? []).filter(t => t.startsWith(tag!))).slice(0, 8).map(t => `${repo}:${t}`) : [];

  const go = (image: string) => {
    const ref = image.trim().replace(/^docker pull\s+/, "").replace(/^https?:\/\//, "");
    if (ref) router.push(`/${ref}`);
  };
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    go(value);
  };

  return <form className="search" onSubmit={onSubmit}>
    <div className="search-field">
      <Autocomplete
        size="xl"
        radius="lg"
        aria-label="Image"
        icon={<IconSearch size={22}/>}
        placeholder="ubuntu:24.04 or ghcr.io/org/app"
        value={value}
        onChange={setValue}
        data={suggestions}
        filter={() => true}
        limit={8}
        onItemSubmit={item => go(item.value)}
        styles={{
          dropdown: {background: "var(--surface)", borderColor: "var(--border)", padding: 6},
          item: {fontFamily: "var(--mono)", fontSize: 14, padding: "8px 14px", borderRadius: 8},
        }}
      />
      <Button className="search-submit" type="submit" radius="md" rightIcon={<IconArrowRight size={16}/>}>Inspect</Button>
    </div>
    <div className="search-hint">
      {wantsTags && data?.Tags ? <><code>{repo}</code> has {data.Tags.length.toLocaleString()} tags</> :
        <>Or put any image in the address bar: <code>ima.ge.cx/redis:7</code></>}
    </div>
  </form>;
}

const steps = [
  {icon: <IconSearch size={20}/>, title: "Name an image", body: "Any public image from Docker Hub, GHCR, MCR, Quay and other registries."},
  {icon: <IconStack2 size={20}/>, title: "We index it once", body: "Layers are streamed and indexed in the cloud. Nothing is pulled to your machine."},
  {icon: <IconLink size={20}/>, title: "Browse and share", body: "Explore files, layers and config, and link straight to any file."},
];

const Home: NextPage = () => {
  return <div className="home">
    <Head><title>ima.ge.cx · See inside any container image</title></Head>
    <TopBar beta/>
    <section className="hero">
      <h1>See inside any <span className="accent">container image</span></h1>
      <p className="lede">Browse the files, layers and config of public images — without pulling them or running Docker.</p>
      <Search/>
    </section>

    <section className="home-section">
      <h2>Try an example</h2>
      <div className="examples">{examples.map(e => <ExampleCard key={e.image} {...e}/>)}</div>
    </section>

    <section className="home-section">
      <h2>How it works</h2>
      <div className="how">{steps.map((s, i) => <div className="how-step" key={s.title}>
        <span className="how-num">{i + 1}</span>
        <div><div className="how-title">{s.icon}{s.title}</div><div className="muted">{s.body}</div></div>
      </div>)}</div>
    </section>

    <footer className="footer">
      <IconFileText size={14}/> Open source on <a href="https://github.com/aidansteele/ima.ge.cx">GitHub</a>
    </footer>
  </div>;
}

export default Home
