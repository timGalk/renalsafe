#!/usr/bin/env python3
"""Private snapshot backup and isolated restore drill; never restores over the live database."""
import argparse, hashlib, json, os, re, secrets, shutil, subprocess, tempfile, time
from pathlib import Path
from urllib.request import Request, build_opener, HTTPRedirectHandler

ROOT = Path(__file__).resolve().parents[1]
IMAGE = 'pgvector/pgvector:pg18@sha256:2358fcba361ed2233a5ed81b5fe4ca779ccb304120ce531a3bf51c0ed7e2bc11'


def run(args, **kwargs):
    try:
        result = subprocess.run(args, cwd=ROOT, stderr=subprocess.PIPE, timeout=300, **kwargs)
    except subprocess.TimeoutExpired:
        raise RuntimeError('Operation timed out: ' + args[0]) from None
    if result.returncode:
        raise RuntimeError('Operation failed: ' + ' '.join(args[:3]) + ' (exit ' + str(result.returncode) + ')' )
    return result


def output(args):
    return run(args, stdout=subprocess.PIPE).stdout.decode().strip()


def sql_cmd(container, database="renalsafe"):
    return ['docker', 'exec', '-i', container, 'psql', '-XqAt', '-v', 'ON_ERROR_STOP=1', '-U', 'renalsafe', '-d', database]


def sql(container, query, database="renalsafe"):
    return run(sql_cmd(container, database), input=query.encode(), stdout=subprocess.PIPE).stdout.decode().strip()


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def private_json(path, value):
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


def fingerprint(container, snapshot=None, database="renalsafe"):
    # Each query imports the same still-open snapshot used by pg_dump.
    prefix = "SET timezone='UTC'; SET bytea_output='hex'; SET extra_float_digits=3; "
    prefix += 'BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY; '
    if snapshot:
        if not re.fullmatch(r'[0-9A-Fa-f-]+', snapshot):
            raise ValueError('Invalid snapshot identifier')
        prefix += "SET TRANSACTION SNAPSHOT '" + snapshot + "'; "
    tables = sql(container, prefix + "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename; COMMIT;", database).splitlines()
    result = {}
    for table in tables:
        if not re.fullmatch(r'[a-z_][a-z_0-9]*', table):
            raise ValueError('Unexpected table identifier')
        query = prefix + 'COPY (SELECT row_to_json(t)::text AS row FROM public."' + table + '" t ORDER BY row_to_json(t)::text COLLATE "C") TO STDOUT; COMMIT;'
        # Hash streamed rows, including stored PDF bytes/password hashes, without printing them.
        process = subprocess.Popen(sql_cmd(container, database), cwd=ROOT, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        try:
            process.stdin.write(query.encode()); process.stdin.close()
            h = hashlib.sha256(); rows = 0
            for line in process.stdout:
                h.update(line); rows += 1
            if process.wait() != 0:
                raise RuntimeError('Fingerprint query failed')
            result[table] = {'rows': rows, 'sha256': h.hexdigest()}
        finally:
            process.stdout.close()
            if process.poll() is None:
                process.kill(); process.wait()
    return result


def backup(destination, container=None, config_root=None):
    destination = destination.resolve()
    if not destination.is_relative_to((ROOT / '.local').resolve()):
        raise ValueError('Backup must be inside ignored .local/ storage')
    destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    destination.mkdir(mode=0o700)  # Refuse overwrites, including incomplete earlier backups.
    config_root = config_root or ROOT
    container = container or output(['docker', 'compose', 'ps', '-q', 'db'])
    if not container:
        raise RuntimeError('Local database is not running')
    keeper = subprocess.Popen(sql_cmd(container), cwd=ROOT, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
    try:
        keeper.stdin.write('BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY; SELECT pg_export_snapshot();\n'); keeper.stdin.flush()
        snapshot = keeper.stdout.readline().strip()
        if not re.fullmatch(r'[0-9A-Fa-f-]+', snapshot):
            raise RuntimeError('Could not export a database snapshot')
        with os.fdopen(os.open(destination / 'database.dump', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as stream:
            run(['docker', 'exec', container, 'pg_dump', '-U', 'renalsafe', '-d', 'renalsafe', '-Fc', '--no-owner', '--no-acl', '--snapshot=' + snapshot], stdout=stream)
        tables = fingerprint(container, snapshot)
    finally:
        keeper.stdin.close(); keeper.stdout.close()
        if keeper.poll() is None:
            keeper.terminate()
        keeper.wait()
    # Files are private, but are plaintext: operator must encrypt copies off the host.
    config = destination / 'config'; config.mkdir(mode=0o700)
    files = {}
    for relative, name in [('.env', 'environment'), ('.local/users.json', 'users.json'), ('.local/tokens.json', 'tokens.json')]:
        with (config_root / relative).open('rb') as source:
            with os.fdopen(os.open(config / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as target:
                shutil.copyfileobj(source, target)
        files['config/' + name] = digest(config / name)
    files['database.dump'] = digest(destination / 'database.dump')
    manifest = {'format': 1, 'created_at_utc': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
                'commit': output(['git', 'rev-parse', 'HEAD']),
                'working_tree_dirty': bool(output(['git', 'status', '--porcelain', '--untracked-files=no'])),
                'database_image': output(['docker', 'inspect', '--format', '{{.Image}}', container]),
                'restore_image': IMAGE, 'postgres_version': sql(container, 'SHOW server_version;'),
                'tables': tables, 'files': files,
                'limits': 'One PostgreSQL database; configuration files copied after snapshot, keep configuration unchanged during backup. No cluster roles, model weights or off-host replication.'}
    private_json(destination / 'manifest.json', manifest)  # Written last: completion marker.
    print('Private backup complete: ' + str(destination))


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise RuntimeError('Redirect forbidden')


def api(base, token, path, body=None):
    request = Request(base + path, data=None if body is None else json.dumps(body).encode(), headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
    with build_opener(NoRedirect).open(request, timeout=30) as response:
        return json.load(response)


def wait_db(name):
    for _ in range(90):
        p = subprocess.run(['docker', 'exec', name, 'pg_isready', '-h', '127.0.0.1', '-U', 'renalsafe', '-d', 'renalsafe'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if p.returncode == 0:
            return
        time.sleep(.5)
    raise RuntimeError('Disposable database did not become ready')


def verify(directory, browser=False):
    directory = directory.resolve()
    manifest = json.loads((directory / 'manifest.json').read_text())
    if manifest['format'] != 1 or set(manifest['files']) != {'database.dump', 'config/environment', 'config/users.json', 'config/tokens.json'}:
        raise ValueError('Unsupported backup manifest')
    for name, expected in manifest['files'].items():
        if digest(directory / name) != expected:
            raise ValueError('Backup checksum mismatch: ' + name)
    name = 'renalsafe-restore-' + secrets.token_hex(6); app = name + '-app'
    password = secrets.token_urlsafe(32)
    image = output(['docker', 'image', 'inspect', '--format', '{{.Id}}', os.environ.get('RENALSAFE_RELEASE_IMAGE', 'renalsafe-core:latest')])
    if not image:
        raise RuntimeError('Build the application image first')
    runtime = tempfile.TemporaryDirectory(prefix='restore-runtime-', dir=ROOT / '.local')
    runtime_users = Path(runtime.name) / 'users.json'
    shutil.copyfile(directory / 'config/users.json', runtime_users)
    runtime_users.chmod(0o644)  # Hashes only; private parent, read-only container mount.
    report = {'backup_sha256': manifest['files']['database.dump'], 'source_commit': manifest['commit'], 'application_image': image, 'checks': []}
    try:
        run(['docker', 'run', '-d', '--name', name, '-e', 'POSTGRES_USER=renalsafe', '-e', 'POSTGRES_PASSWORD=' + password, '-e', 'POSTGRES_DB=renalsafe', '-p', '127.0.0.1::8080', IMAGE], stdout=subprocess.DEVNULL)
        wait_db(name)
        with (directory / 'database.dump').open('rb') as stream:
            run(['docker', 'exec', '-i', name, 'pg_restore', '-U', 'renalsafe', '-d', 'renalsafe', '--no-owner', '--no-acl', '--exit-on-error'], stdin=stream, stdout=subprocess.DEVNULL)
        if fingerprint(name) != manifest['tables']:
            raise RuntimeError('Restored table data does not match backup snapshot')
        report['checks'].append('all public table counts and canonical row hashes match source snapshot')
        run(['docker', 'restart', name], stdout=subprocess.DEVNULL); wait_db(name)
        if fingerprint(name) != manifest['tables']:
            raise RuntimeError('Restored data changed after database restart')
        report['checks'].append('identical table data after database restart')
        # Invalidate potentially resurrected browser sessions and reset links, only in the clone.
        sql(name, "BEGIN; DELETE FROM auth_sessions; UPDATE app_users SET reset_hash='',reset_expires=NULL; COMMIT;")
        report['checks'].append('restored browser sessions and outstanding activation/reset links invalidated')
        run(['docker', 'run', '-d', '--name', app, '--network', 'container:' + name, '--read-only', '--tmpfs', '/tmp:size=64m,mode=1777', '--cap-drop=ALL', '--security-opt', 'no-new-privileges:true', '-v', str(runtime_users) + ':/run/renalsafe-users.json:ro', '-e', 'RENALSAFE_DATABASE_URL=postgres://renalsafe:' + password + '@127.0.0.1:5432/renalsafe?sslmode=disable', image, '-listen', '0.0.0.0:8080', '-users', '/run/renalsafe-users.json', '-seed', '', '-migrate'], stdout=subprocess.DEVNULL)
        info = json.loads(output(['docker', 'inspect', name]))[0]
        port = info['NetworkSettings']['Ports']['8080/tcp'][0]['HostPort']; base = 'http://127.0.0.1:' + port
        token = json.loads((directory / 'config/tokens.json').read_text())['admin']
        for _ in range(90):
            try:
                api(base, token, '/readyz'); break
            except Exception:
                time.sleep(.5)
        else:
            raise RuntimeError('Restored application did not become ready')
        audit = api(base, token, '/api/v1/admin/audit/verify', {})
        if not audit['valid']:
            raise RuntimeError('Restored audit chain invalid')
        ids = sql(name, 'SELECT id FROM assessments ORDER BY id;').splitlines()
        for assessment in ids:
            if not re.fullmatch(r'[A-Za-z0-9_-]+', assessment):
                raise ValueError('Unexpected assessment identifier')
            api(base, token, '/api/v1/assessments/' + assessment + '/export')
            if not api(base, token, '/api/v1/assessments/' + assessment + '/replay', {})['reproduced']:
                raise RuntimeError('Historical replay mismatch')
        report.update(assessments_replayed=len(ids), table_counts={k: v['rows'] for k, v in manifest['tables'].items()}, audit=audit)
        report['checks'].append('restored application migration, audit chain, assessment exports and historical replay pass')
        if browser:
            env = os.environ.copy(); env['RENALSAFE_BASE_URL'] = base
            # Fixtures read host setup keys; ensure they match this private backup before use.
            if digest(ROOT / '.local/tokens.json') != manifest['files']['config/tokens.json']:
                raise RuntimeError('Browser fixture credentials differ from backup')
            run(['node', 'scripts/smoke-workspace.cjs'], env=env, stdout=subprocess.DEVNULL)
            report['checks'].append('full multi-role React browser acceptance on isolated restored application')
        report['valid'] = True
        report['verified_at_utc'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
        private_json(directory / ('verification-' + secrets.token_hex(4) + '.json'), report)
        print('Isolated restore verified: ' + str(len(ids)) + ' assessments; live database unchanged.')
    finally:
        subprocess.run(['docker', 'rm', '-fv', app, name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        runtime.cleanup()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['backup', 'verify'])
    parser.add_argument('directory', type=Path)
    parser.add_argument('--browser', action='store_true', help='Run all-role browser acceptance against the restored clone')
    args = parser.parse_args()
    os.umask(0o077)
    if args.command == 'backup':
        backup(args.directory)
    else:
        verify(args.directory, args.browser)


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, ValueError, OSError, KeyError) as error:
        raise SystemExit('Recovery operation failed: ' + str(error))
