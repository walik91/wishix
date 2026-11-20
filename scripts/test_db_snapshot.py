import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

import db_snapshot


class SnapshotArchiveTests(unittest.TestCase):
    def archive(self, destination, entries):
        with tarfile.open(destination, "w:gz") as tar:
            for name, content, kind in entries:
                member = tarfile.TarInfo(name)
                member.type = kind
                member.size = len(content) if kind == tarfile.REGTYPE else 0
                tar.addfile(member, io.BytesIO(content) if kind == tarfile.REGTYPE else None)

    def test_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "source"
            collection = source / "collection"
            collection.mkdir(parents=True)
            (collection / "data").write_bytes(b"\x00\x01\xff")
            archive = root / "copy.tar.gz"
            db_snapshot.pack_directory(archive, collection, {"format": "wishix-cockroach-backup-v1", "database": "wishix"})
            target = root / "target"
            target.mkdir()
            self.assertEqual(db_snapshot.unpack_archive(archive, target), "wishix")
            self.assertEqual((target / "collection" / "data").read_bytes(), b"\x00\x01\xff")

    def test_unsafe_archive_entries_are_rejected(self):
        manifest = json.dumps({"format": "wishix-cockroach-backup-v1", "database": "wishix"}).encode()
        for name, kind in [("collection/../../outside", tarfile.REGTYPE),
                           ("collection/C:/outside", tarfile.REGTYPE),
                           ("collection/link", tarfile.SYMTYPE),
                           ("/absolute", tarfile.REGTYPE)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / "bad.tar.gz"
                self.archive(archive, [("manifest.json", manifest, tarfile.REGTYPE),
                                       ("collection/data", b"data", tarfile.REGTYPE),
                                       (name, b"bad", kind)])
                with self.assertRaises(ValueError):
                    db_snapshot.unpack_archive(archive, root / "out")
                self.assertFalse((root / "outside").exists())

    def test_manifest_must_be_safe(self):
        for database in ("wishix'; DROP DATABASE wishix; --", "", "UPPER"):
            with self.subTest(database=database), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / "bad.tar.gz"
                manifest = json.dumps({"format": "wishix-cockroach-backup-v1", "database": database}).encode()
                self.archive(archive, [("manifest.json", manifest, tarfile.REGTYPE),
                                       ("collection/data", b"data", tarfile.REGTYPE)])
                with self.assertRaises(ValueError):
                    db_snapshot.unpack_archive(archive, root / "out")


if __name__ == "__main__":
    unittest.main()
