#!/usr/bin/env python3
"""Validate release tags and suggest the next JST calendar version."""

import argparse
import datetime as dt
import re
import subprocess

JST = dt.timezone(dt.timedelta(hours=9), name="JST")
TAG = re.compile(r"v([0-9]{4})\.([0-9]{2})\.([0-9]{2})\.(0|[1-9][0-9]*)", re.ASCII)


def parse_tag(value):
    match = TAG.fullmatch(value)
    if not match:
        raise ValueError("expected vYYYY.0M.0D.MICRO (e.g. v2026.10.18.0) with a nonnegative, unpadded counter")
    year, month, day, counter = map(int, match.groups())
    try:
        date = dt.date(year, month, day)
    except ValueError as exc:
        raise ValueError("release tag contains an invalid calendar date") from exc
    return date, counter


def jst_date(now=None):
    now = now or dt.datetime.now(dt.timezone.utc)
    return now.astimezone(JST).date()


def next_tag(tags, date):
    counter = -1
    for tag in tags:
        try:
            tagged_date, revision = parse_tag(tag)
        except ValueError:
            continue  # Ignore unrelated tags from other naming schemes.
        if tagged_date > date:
            raise ValueError("a later-dated release already exists; do not backdate releases")
        if tagged_date == date:
            counter = max(counter, revision)
    return f"v{date.year:04d}.{date.month:02d}.{date.day:02d}.{counter + 1}"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subcommands = parser.add_subparsers(dest="command", required=True)
    validate = subcommands.add_parser("validate", help="Validate a release tag's format and calendar date")
    validate.add_argument("tag")
    next_version = subcommands.add_parser("next", help="Print the next version from local Git tags; never create a tag")
    next_version.add_argument("--date", type=dt.date.fromisoformat, help="Override the JST date (YYYY-MM-DD)")
    args = parser.parse_args()
    try:
        if args.command == "validate":
            parse_tag(args.tag)
            print(args.tag)
        else:
            result = subprocess.run(["git", "tag", "--list"], check=True, capture_output=True, text=True)
            print(next_tag(result.stdout.splitlines(), args.date or jst_date()))
    except ValueError as exc:
        parser.exit(1, f"Error: {exc}\n")
    except (OSError, subprocess.CalledProcessError):
        parser.exit(1, "Error: could not read local Git tags; run from the repository\n")


if __name__ == "__main__":
    main()
