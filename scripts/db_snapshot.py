#!/usr/bin/env python3
"""Create and restore portable CockroachDB database backups through Docker."""

import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import uuid


IDENTIFIER = re.compile(r"[a-z][a-z0-9_]{0,62}\Z")
EXTERN = "/cockroach/cockroach-data/extern"
PREFIX = "wishix-transfer"


def identifier(value):
    if not IDENTIFIER.fullmatch(value):
        raise argparse.ArgumentTypeError("Use lowercase letters, digits and underscores; start with a letter (max 63 characters).")
    return value


def docker(*args, capture=False):
    return subprocess.run(["docker", *args], check=True, text=True,
                          stdout=subprocess.PIPE if capture else None)


def resolve_container(args):
    if args.container:
        return args.container
    result = docker("compose", "-f", args.compose, "ps", "--status", "running", "-q", "db", capture=True).stdout.splitlines()
    if len(result) != 1:
        raise RuntimeError("Expected exactly one running db container. Start it first or specify --container.")
    return result[0]


def sql(container, statement, capture=False):
    return docker("exec", container, "/cockroach/cockroach", "sql", "--insecure", "--host=localhost:26257",
                  "--format=tsv", "--execute", statement, capture=capture)


def stage_path():
    return f"{PREFIX}/{uuid.uuid4().hex}"


def remove_stage(container, stage):
    # This path is generated locally, never provided by the user.
    docker("exec", container, "rm", "-rf", f"{EXTERN}/{stage}")


def pack_directory(archive, collection, metadata):
    metadata_file = collection.parent / "manifest.json"
    metadata_file.write_text(json.dumps(metadata, ensure_ascii=True) + "\n", encoding="utf-8")
    with tarfile.open(archive, "w:gz") as tar:
        tar.add(metadata_file, arcname="manifest.json", recursive=False)
        tar.add(collection, arcname="collection")


def unpack_archive(archive, root):
    # Only regular files/directories are allowed. Never trust paths from an archive.
    with tarfile.open(archive, "r:gz") as tar:
        members = tar.getmembers()
        seen = set()
        for member in members:
            path = PurePosixPath(member.name)
            parts = path.parts
            if (not parts or path.is_absolute() or ".." in parts or "\\" in member.name or ":" in member.name
                    or parts[0] not in ("manifest.json", "collection")
                    or (parts[0] == "manifest.json" and len(parts) != 1)
                    or member.name != path.as_posix()
                    or not (member.isfile() or member.isdir()) or member.name in seen):
                raise ValueError("Unsafe or unexpected entry in backup archive.")
            seen.add(member.name)
            if member.name == "manifest.json" and (not member.isfile() or member.size > 4096):
                raise ValueError("Invalid backup manifest.")
        if "manifest.json" not in seen or not any(m.name.startswith("collection/") and m.isfile() for m in members):
            raise ValueError("Backup archive is incomplete.")
        for member in members:
            path = root.joinpath(*PurePosixPath(member.name).parts)
            if member.isdir():
                path.mkdir(parents=True, exist_ok=True)
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                with tar.extractfile(member) as source, path.open("xb") as target:
                    shutil.copyfileobj(source, target)
    metadata = json.loads((root / "manifest.json").read_text(encoding="utf-8"))
    if not isinstance(metadata, dict) or metadata.get("format") != "wishix-cockroach-backup-v1":
        raise ValueError("Unsupported backup format.")
    source_db = metadata.get("database")
    if not isinstance(source_db, str) or not IDENTIFIER.fullmatch(source_db):
        raise ValueError("Invalid source database in backup manifest.")
    return source_db


def backup(args):
    destination = Path(args.output).expanduser().resolve()
    if destination.exists():
        raise FileExistsError(f"Output already exists: {destination}")
    if not destination.parent.is_dir():
        raise ValueError("Output directory does not exist.")
    container = resolve_container(args)
    stage = stage_path()
    uri = f"nodelocal://1/{stage}"
    version = sql(container, "SELECT version();", capture=True).stdout.strip().splitlines()[-1]
    docker("exec", container, "mkdir", "-p", f"{EXTERN}/{PREFIX}")
    complete = False
    try:
        sql(container, f'BACKUP DATABASE "{args.database}" INTO \'{uri}\';')
        sql(container, f"SHOW BACKUP LATEST IN '{uri}';")
        with tempfile.TemporaryDirectory(prefix="wishix-backup-") as tmp:
            temp = Path(tmp)
            collection = temp / "collection"
            docker("cp", f"{container}:{EXTERN}/{stage}", str(collection))
            # The complete file is published atomically, without overwriting an existing file.
            part = destination.parent / f".{destination.name}.{uuid.uuid4().hex}.part"
            try:
                fd = os.open(part, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
                os.close(fd)
                pack_directory(part, collection, {"format": "wishix-cockroach-backup-v1", "database": args.database, "cockroach_version": version})
                os.link(part, destination)
            finally:
                part.unlink(missing_ok=True)
        complete = True
    finally:
        if complete:
            remove_stage(container, stage)
        else:
            print(f"Backup did not complete; temporary database files may remain at {EXTERN}/{stage} in container {container}.", file=sys.stderr)
    print(f"Backup created: {destination}")


def restore(args):
    archive = Path(args.archive).expanduser().resolve(strict=True)
    container = resolve_container(args)
    stage = stage_path()
    uri = f"nodelocal://1/{stage}"
    complete = False
    with tempfile.TemporaryDirectory(prefix="wishix-restore-") as tmp:
        temp = Path(tmp)
        source_db = unpack_archive(archive, temp)
        # RESTORE DATABASE itself also refuses an existing destination database.
        exists = sql(container, f"SELECT count(*) FROM [SHOW DATABASES] WHERE database_name = '{args.target}';", capture=True).stdout
        if exists.strip().splitlines()[-1] != "0":
            raise ValueError(f"Target database already exists: {args.target}; nothing was overwritten.")
        if args.grant_user:
            exists = sql(container, f"SELECT count(*) FROM [SHOW USERS] WHERE username = '{args.grant_user}';", capture=True).stdout
            if exists.strip().splitlines()[-1] != "1":
                raise ValueError(f"Database user does not exist: {args.grant_user}; no restore was started.")
        docker("exec", container, "mkdir", "-p", f"{EXTERN}/{PREFIX}")
        try:
            docker("cp", str(temp / "collection"), f"{container}:{EXTERN}/{stage}")
            sql(container, f"SHOW BACKUP LATEST IN '{uri}';")
            sql(container, f'RESTORE DATABASE "{source_db}" FROM LATEST IN \'{uri}\' WITH new_db_name = \'{args.target}\';')
            if args.grant_user:
                sql(container, f'GRANT ALL ON DATABASE "{args.target}" TO "{args.grant_user}"; '
                    f'GRANT ALL ON SCHEMA "{args.target}".public TO "{args.grant_user}"; '
                    f'GRANT ALL ON ALL TABLES IN SCHEMA "{args.target}".public TO "{args.grant_user}";')
            complete = True
        finally:
            if complete:
                remove_stage(container, stage)
            else:
                print(f"Restore did not complete. Inspect database {args.target} and temporary files {EXTERN}/{stage} in {container}; nothing is deleted automatically.", file=sys.stderr)
    print(f"Restored into new database: {args.target}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("backup", "restore"):
        p = sub.add_parser(name)
        p.add_argument("--compose", default="compose.yaml", help="Compose file (default: compose.yaml)")
        p.add_argument("--container", help="Explicit running CockroachDB container instead of Compose lookup")
        if name == "backup":
            p.add_argument("--database", required=True, type=identifier, help="Source database name")
            p.add_argument("--output", required=True, help="New .tar.gz file; never overwritten")
            p.set_defaults(run=backup)
        else:
            p.add_argument("--archive", required=True, help="Backup .tar.gz file")
            p.add_argument("--target", required=True, type=identifier, help="New destination database; must not exist")
            p.add_argument("--grant-user", type=identifier, help="Existing database user to grant application access")
            p.set_defaults(run=restore)
    args = parser.parse_args()
    try:
        args.run(args)
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError, tarfile.TarError) as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
