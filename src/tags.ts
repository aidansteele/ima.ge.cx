// Splits an image reference such as "ghcr.io/org/app:v1" into its repository
// and tag. A colon before the last slash belongs to a registry port.
export function splitTag(image: string): { repo: string, tag?: string } {
  const i = image.lastIndexOf(":");
  return i > image.lastIndexOf("/") ? {repo: image.slice(0, i), tag: image.slice(i + 1)} : {repo: image};
}

const numericCollator = new Intl.Collator(undefined, {numeric: true});

// Newest-looking versions first (so 1.22 comes before 1.9), with "latest" at the top.
// Search puts exact matches first, then prefixes, before the remaining tags.
export function sortTags(tags: string[], search = "") {
  const query = search.toLowerCase().trim();
  const rank = (tag: string) => {
    const value = tag.toLowerCase();
    return !query ? 0 : value === query ? 0 : value.startsWith(query) ? 1 : 2;
  };
  return [...tags].sort((a, b) =>
    rank(a) - rank(b) ||
    Number(b === "latest") - Number(a === "latest") || numericCollator.compare(b, a));
}

export function registryOf(repo: string) {
  const first = repo.split("/")[0];
  return repo.includes("/") && /[.:]|^localhost$/.test(first) ? first : "docker.io";
}

const prerelease = /(rc|alpha|beta|preview)\d*/i;

// Orders tags for suggestions: stable releases before pre-releases, and plain
// versions ("1.22") before variants ("1.22-windowsservercore-ltsc2022").
export function rankTagSuggestions(tags: string[]) {
  const sorted = sortTags(tags);
  const rank = (t: string) => Number(prerelease.test(t)) * 100 + t.split("-").length;
  return sorted.map((t, i) => ({t, i, r: rank(t)})).sort((a, b) => a.r - b.r || a.i - b.i).map(x => x.t);
}
