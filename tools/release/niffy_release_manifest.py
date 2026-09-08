#!/usr/bin/env python3
"""Create or verify a content-addressed manifest for a local Niffy release."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]
SCHEMA_VERSION = "niffy/release-manifest/v1"
SEMVER = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$")


def file_digest(path: Path) -> str:
    return "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()


def run(*args: str) -> str:
    result = subprocess.run(
        args,
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def inspect_image(reference: str) -> dict[str, Any]:
    if reference.endswith(":latest") or ":latest@" in reference:
        raise ValueError(f"mutable image tag is not allowed: {reference}")
    document = json.loads(run("docker", "image", "inspect", reference))
    if not isinstance(document, list) or len(document) != 1:
        raise ValueError(f"could not uniquely inspect image: {reference}")
    image = document[0]
    return {
        "reference": reference,
        "image_id": image["Id"],
        "repo_digests": sorted(image.get("RepoDigests") or []),
        "platform": f"{image.get('Os', 'unknown')}/{image.get('Architecture', 'unknown')}",
        "created": image.get("Created", ""),
    }


def relative_path(path: Path) -> str:
    return path.resolve().relative_to(REPO_ROOT).as_posix()


def display_path(path: Path) -> str:
    try:
        return relative_path(path)
    except ValueError:
        return str(path)


def model_inventory(path: Path) -> list[dict[str, Any]]:
    root = path.resolve()
    if not root.is_dir():
        raise ValueError(f"model cache is unavailable: {path}")
    files = sorted(item for item in root.rglob("*") if item.is_file())
    if not files:
        raise ValueError(f"model cache is empty: {path}")
    return [
        {
            "path": item.relative_to(root).as_posix(),
            "size": item.stat().st_size,
            "digest": file_digest(item),
        }
        for item in files
    ]


def source_state() -> tuple[str, bool]:
    revision = run("git", "rev-parse", "HEAD")
    dirty = bool(run("git", "status", "--porcelain", "--untracked-files=all"))
    return revision, dirty


def build_manifest(args: argparse.Namespace) -> dict[str, Any]:
    if not SEMVER.fullmatch(args.version):
        raise ValueError("Niffy version must be a semantic version without a leading v")
    config_path = (REPO_ROOT / args.config).resolve()
    metadata_path = (REPO_ROOT / args.metadata).resolve()
    models_path = REPO_ROOT / args.models_dir
    revision, dirty = source_state()
    if dirty and not args.allow_dirty:
        raise ValueError(
            "release source is dirty; commit the reviewed release or pass --allow-dirty "
            "for local validation only"
        )
    metadata_version = next(
        (
            line.split(":", 1)[1].strip()
            for line in metadata_path.read_text(encoding="utf-8").splitlines()
            if line.startswith("version:")
        ),
        "",
    )
    if metadata_version != args.version:
        raise ValueError(
            f"metadata version {metadata_version!r} does not match {args.version!r}"
        )
    return {
        "schema_version": SCHEMA_VERSION,
        "niffy_version": args.version,
        "generated_at": datetime.now(UTC).isoformat(),
        "source": {"revision": revision, "dirty": dirty},
        "artifacts": {
            "config": {
                "path": relative_path(config_path),
                "digest": file_digest(config_path),
            },
            "metadata": {
                "path": relative_path(metadata_path),
                "digest": file_digest(metadata_path),
            },
            "models": {
                "path": args.models_dir,
                "files": model_inventory(models_path),
            },
        },
        "images": {
            "router": inspect_image(args.router_image),
            "dashboard": inspect_image(args.dashboard_image),
            "envoy": inspect_image(args.envoy_image),
        },
        "contracts": {
            "config": "v0.3",
            "routing_decision": "vllm-sr/routing-decision/v1",
            "recipe_metadata": "vllm-sr/recipe-metadata/v1",
        },
    }


def verify_manifest(path: Path, args: argparse.Namespace) -> None:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ValueError("unsupported Niffy release manifest schema")
    expected_config = relative_path((REPO_ROOT / args.config).resolve())
    actual_config = manifest.get("artifacts", {}).get("config", {}).get("path")
    if actual_config != expected_config:
        raise ValueError(
            f"startup config {expected_config!r} does not match manifest {actual_config!r}"
        )
    expected_references = {
        "router": args.router_image,
        "dashboard": args.dashboard_image,
        "envoy": args.envoy_image,
    }
    for name, artifact in manifest.get("artifacts", {}).items():
        if name == "models":
            expected_models = artifact.get("files", [])
            actual_models = model_inventory(REPO_ROOT / artifact["path"])
            if actual_models != expected_models:
                raise ValueError(f"release model cache drifted: {artifact['path']}")
            continue
        artifact_path = REPO_ROOT / artifact["path"]
        if file_digest(artifact_path) != artifact["digest"]:
            raise ValueError(f"release artifact drifted: {artifact['path']}")
    for name, expected in manifest.get("images", {}).items():
        requested_reference = expected_references.get(name)
        if requested_reference and requested_reference != expected["reference"]:
            raise ValueError(
                f"startup image {requested_reference!r} does not match manifest "
                f"reference {expected['reference']!r}"
            )
        actual = inspect_image(expected["reference"])
        if actual["image_id"] != expected["image_id"]:
            raise ValueError(f"release image drifted: {name} ({expected['reference']})")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--allow-dirty", action="store_true")
    parser.add_argument("--version")
    parser.add_argument("--router-image")
    parser.add_argument("--dashboard-image")
    parser.add_argument("--envoy-image")
    parser.add_argument("--config", default="config/niffy/config.yaml")
    parser.add_argument("--models-dir", default="config/niffy/.vllm-sr/models")
    parser.add_argument("--metadata", default="config/niffy/metadata.yaml")
    parser.add_argument("--output", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    output = (REPO_ROOT / args.output).resolve()
    try:
        if args.check:
            verify_manifest(output, args)
            print(f"Verified Niffy release manifest: {display_path(output)}")
            return 0
        required = (
            args.version,
            args.router_image,
            args.dashboard_image,
            args.envoy_image,
        )
        if not all(required):
            raise ValueError("version and all three image references are required")
        manifest = build_manifest(args)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        print(f"Wrote Niffy release manifest: {display_path(output)}")
        return 0
    except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"Niffy release manifest error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
