#!/usr/bin/env python3
# Copied from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6 and adapted for retained manifests.
import argparse
import datetime
import os
import re


def list_versions(namespace, package, token):
    import requests
    url = f"https://api.github.com/{namespace}/packages/container/{package}/versions?per_page=100"
    headers = {"Accept": "application/vnd.github+json", "Authorization": f"Bearer {token}", "X-GitHub-Api-Version": "2022-11-28"}
    versions = []
    while url:
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()
        versions.extend(response.json())
        next_link = response.links.get("next")
        url = next_link["url"] if next_link else None
    return versions


def cleanup_actions(versions, max_age, keep_pattern, now=None):
    now = now or datetime.datetime.now(datetime.UTC)
    cutoff = now - datetime.timedelta(seconds=max_age)
    keep_re = re.compile(keep_pattern)
    kept_timestamps = []
    actions = []
    for version in versions:
        tags = version.get("metadata", {}).get("container", {}).get("tags", [])
        created = datetime.datetime.fromisoformat(version["created_at"].replace("Z", "+00:00"))
        retained = any(keep_re.search(tag) for tag in tags) or created > cutoff
        actions.append((version, retained))
        if retained:
            kept_timestamps.append(created)
    # GHCR represents a retained multi-platform manifest with untagged child versions.
    for index, (version, retained) in enumerate(actions):
        tags = version.get("metadata", {}).get("container", {}).get("tags", [])
        created = datetime.datetime.fromisoformat(version["created_at"].replace("Z", "+00:00"))
        if not retained and not tags and any(abs(created - stamp) <= datetime.timedelta(seconds=10) for stamp in kept_timestamps):
            actions[index] = (version, True)
    return actions


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--package", dest="package_name", required=True)
    parser.add_argument("--tagged-max-age", type=int, default=604800)
    parser.add_argument("--keep-tags-pattern", default=r"^(latest|latest-|git-tag-|v[0-9])")
    parser.add_argument("--really-remove", action="store_true")
    args = parser.parse_args()
    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        raise SystemExit("GITHUB_TOKEN is required")
    for version, keep in cleanup_actions(list_versions(args.namespace, args.package_name, token), args.tagged_max_age, args.keep_tags_pattern):
        if keep:
            continue
        print(f"delete version {version['id']}")
        if args.really_remove:
            import requests
            response = requests.delete(f"https://api.github.com/{args.namespace}/packages/container/{args.package_name}/versions/{version['id']}", headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}, timeout=30)
            response.raise_for_status()


if __name__ == "__main__":
    main()
