#!/usr/bin/env python3
"""Safety regressions: corrupt/incomplete backups fail before any Docker mutation."""
import json, tempfile, unittest
from pathlib import Path
from unittest.mock import patch
import recovery


class RecoverySafety(unittest.TestCase):
    def fixture(self, directory):
        for name in ['database.dump', 'config/environment', 'config/users.json', 'config/tokens.json']:
            p = directory / name; p.parent.mkdir(exist_ok=True); p.write_bytes(b'private-fixture')
        manifest = {'format': 1, 'files': {name: recovery.digest(directory / name) for name in ['database.dump', 'config/environment', 'config/users.json', 'config/tokens.json']}}
        (directory / 'manifest.json').write_text(json.dumps(manifest))
        return manifest

    def test_corrupted_dump_rejected_before_docker(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(recovery, 'output') as docker:
            path = Path(tmp); self.fixture(path); (path / 'database.dump').write_bytes(b'truncated')
            with self.assertRaisesRegex(ValueError, 'checksum mismatch'):
                recovery.verify(path)
            docker.assert_not_called()

    def test_manifest_cannot_add_paths(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(recovery, 'output') as docker:
            path = Path(tmp); manifest = self.fixture(path); manifest['files']['../../other'] = 'bad'
            (path / 'manifest.json').write_text(json.dumps(manifest))
            with self.assertRaisesRegex(ValueError, 'Unsupported'):
                recovery.verify(path)
            docker.assert_not_called()

    def test_incomplete_backup_rejected(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(recovery, 'output') as docker:
            with self.assertRaises(FileNotFoundError):
                recovery.verify(Path(tmp))
            docker.assert_not_called()

    def test_backup_outside_private_root_rejected(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(recovery, 'output') as docker:
            with self.assertRaisesRegex(ValueError, 'ignored .local'):
                recovery.backup(Path(tmp) / 'backup')
            docker.assert_not_called()

    def test_existing_backup_not_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(recovery, 'ROOT', Path(tmp)), patch.object(recovery, 'output') as docker:
            path = Path(tmp) / '.local/backup'; path.mkdir(parents=True)
            with self.assertRaises(FileExistsError):
                recovery.backup(path)
            docker.assert_not_called()


if __name__ == '__main__':
    unittest.main()
