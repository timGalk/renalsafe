#!/usr/bin/env python3
"""Record public source/build identities without configuration values or credentials."""
import argparse, json, platform, subprocess, time
from pathlib import Path
import recovery as r

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
files = set(r.output(['git', 'ls-files', '--', 'core', 'web', 'api', 'data', 'scripts', 'compose.yaml', '.github/workflows/core.yml']).splitlines())
files.update(str(x.relative_to(r.ROOT)) for x in (r.ROOT / 'scripts').glob('*.py'))
files = {x for x in files if (r.ROOT / x).is_file()}
lock = json.loads((r.ROOT / 'web/package-lock.json').read_text())
manifest = {
    'created_at_utc': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
    'commit': r.output(['git', 'rev-parse', 'HEAD']),
    'working_tree_dirty': bool(r.output(['git', 'status', '--porcelain', '--untracked-files=normal', '--', 'core', 'web', 'api', 'data', 'scripts', 'compose.yaml', '.github/workflows/core.yml'])),
    'platform': platform.system() + ' ' + platform.machine(),
    'tools': {label: r.output(command) for label, command in {'go': ['go', 'version'], 'node': ['node', '--version'], 'npm': ['npm', '--version'], 'docker': ['docker', '--version']}.items()},
    'images': {service: r.output(['docker', 'image', 'inspect', '--format', '{{.Id}}', 'renalsafe-core:latest' if service == 'core' else r.IMAGE]) for service in ['core', 'db']},
    'frontend_dependencies': lock['packages'][''],
    'sha256': {name: r.digest(r.ROOT / name) for name in sorted(files)},
    'model_evidence': 'No real extraction/embedding model benchmark is claimed. Local Qwen/MedGemma extraction is planned, not implemented.',
}
a.output.parent.mkdir(parents=True, exist_ok=True)
r.private_json(a.output, manifest)
print('Release manifest saved: ' + str(a.output))
