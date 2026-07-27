#!/usr/bin/env python3
"""
ROOT File Reader for CEPC Analysis Copilot.

Provides inspect and convert commands for ROOT/nanoAOD files.
Called by the Go Data Agent via exec.

Usage:
    python root_reader.py inspect <file.root>
    python root_reader.py convert <file.root> --output <out.parquet>
"""

import sys
import json
import argparse
import os


def inspect_root(filepath: str) -> dict:
    """Inspect a ROOT file and return metadata as JSON."""
    try:
        import uproot
    except ImportError:
        return _fallback_inspect(filepath)

    file = uproot.open(filepath)
    tree_name = None
    for key in file.keys():
        obj = file[key]
        if hasattr(obj, "num_entries"):
            tree_name = key
            break

    if tree_name is None:
        return {"error": "No TTree found in file", "file": filepath}

    tree = file[tree_name]
    branches = tree.keys()
    event_count = tree.num_entries

    # Compute basic branch metadata
    branch_info = []
    for b in branches:
        interp = tree[b].interpretation
        branch_info.append({
            "name": b,
            "type": str(interp.to_numpy.dtype) if interp else "unknown"
        })

    result = {
        "file": filepath,
        "tree_name": tree_name,
        "event_count": event_count,
        "branches": branches,
        "branch_info": branch_info,
        "has_nan": False,
        "file_size_bytes": os.path.getsize(filepath),
    }

    # Quick NaN check on first 1000 events
    try:
        batch = tree.arrays(library="np", entry_stop=min(1000, event_count))
        for b in branches:
            arr = batch[b]
            if hasattr(arr, 'dtype') and 'float' in str(arr.dtype):
                import numpy as np
                if np.any(np.isnan(arr)):
                    result["has_nan"] = True
                    break
    except Exception:
        pass

    return result


def convert_to_parquet(filepath: str, output_path: str) -> dict:
    """Convert a ROOT TTree to Parquet format."""
    try:
        import uproot
    except ImportError:
        return {"error": "uproot not installed", "converted": False}

    file = uproot.open(filepath)
    tree_name = None
    for key in file.keys():
        obj = file[key]
        if hasattr(obj, "num_entries"):
            tree_name = key
            break

    if tree_name is None:
        return {"error": "No TTree found", "converted": False}

    tree = file[tree_name]
    os.makedirs(os.path.dirname(output_path) or ".", exist_ok=True)
    tree.to_parquet(output_path)

    return {
        "storage_path": output_path,
        "converted": True,
        "event_count": tree.num_entries,
        "branches": tree.keys(),
    }


def _fallback_inspect(filepath: str) -> dict:
    """Fallback inspection when uproot is not available."""
    if not os.path.exists(filepath):
        return {"error": f"File not found: {filepath}"}
    return {
        "file": filepath,
        "event_count": 0,
        "branches": [],
        "warning": "uproot not installed; returning file metadata only",
        "file_size_bytes": os.path.getsize(filepath),
    }


def main():
    parser = argparse.ArgumentParser(description="CEPC ROOT File Reader")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # inspect subcommand
    inspect_parser = subparsers.add_parser("inspect", help="Inspect ROOT file metadata")
    inspect_parser.add_argument("filepath", help="Path to ROOT file")

    # convert subcommand
    convert_parser = subparsers.add_parser("convert", help="Convert ROOT to Parquet")
    convert_parser.add_argument("filepath", help="Path to ROOT file")
    convert_parser.add_argument("--output", required=True, help="Output Parquet path")

    args = parser.parse_args()

    if args.command == "inspect":
        result = inspect_root(args.filepath)
    elif args.command == "convert":
        result = convert_to_parquet(args.filepath, args.output)
    else:
        result = {"error": "Unknown command"}

    print(json.dumps(result, indent=2, default=str))


if __name__ == "__main__":
    main()
