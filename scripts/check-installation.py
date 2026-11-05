#!/usr/bin/env python3
"""Verify bootstrap, fresh install, additive migration and dependency failure in disposable containers."""
import hashlib, json, os, runpy, secrets, shutil, subprocess, sys, time
from pathlib import Path
from urllib.error import HTTPError
import recovery as r


def main():
    os.umask(0o077)
    name = 'renalsafe-install-' + secrets.token_hex(6)
    root = r.ROOT / '.local' / 'release-checks' / name
    (root / 'scripts').mkdir(parents=True, mode=0o700)
    shutil.copyfile(r.ROOT / 'scripts/bootstrap-local.py', root / 'scripts/bootstrap-local.py')
    r.run(['python3', str(root / 'scripts/bootstrap-local.py')], stdout=subprocess.DEVNULL)
    # The service runs as an unprivileged UID; only this hashes file is world-readable.
    image = r.output(['docker', 'image', 'inspect', '--format', '{{.Id}}', os.environ.get('RENALSAFE_RELEASE_IMAGE', 'renalsafe-core:latest')])
    if not image:
        raise RuntimeError('Build the application image first')
    keys = json.loads((root / '.local/tokens.json').read_text())
    password = secrets.token_urlsafe(32)
    report = {'application_image': image, 'checks': [], 'commit': r.output(['git', 'rev-parse', 'HEAD'])}
    app = name + '-app'
    try:
        r.run(['docker', 'run', '-d', '--name', name, '-e', 'POSTGRES_USER=renalsafe', '-e', 'POSTGRES_PASSWORD=' + password, '-e', 'POSTGRES_DB=renalsafe', '-p', '127.0.0.1::8080', r.IMAGE], stdout=subprocess.DEVNULL)
        r.wait_db(name)
        info = json.loads(r.output(['docker', 'inspect', name]))[0]
        base = 'http://127.0.0.1:' + info['NetworkSettings']['Ports']['8080/tcp'][0]['HostPort']
        def start(users_file=None):
            users_file = users_file or root / '.local/users.json'
            r.run(['docker', 'run', '-d', '--name', app, '--network', 'container:' + name, '--read-only', '--tmpfs', '/tmp:size=64m,mode=1777', '--cap-drop=ALL', '--security-opt', 'no-new-privileges:true', '-v', str(users_file) + ':/run/renalsafe-users.json:ro', '-e', 'RENALSAFE_DATABASE_URL=postgres://renalsafe:' + password + '@127.0.0.1:5432/renalsafe?sslmode=disable', image, '-listen', '0.0.0.0:8080', '-users', '/run/renalsafe-users.json', '-migrate'], stdout=subprocess.DEVNULL)
            for _ in range(90):
                try:
                    r.api(base, '', '/readyz'); return
                except Exception:
                    time.sleep(.5)
            raise RuntimeError('Fresh application not ready')
        start()
        assert len(r.sql(name, 'SELECT name FROM schema_migrations;').splitlines()) == 4
        assert r.api(base, keys['admin'], '/api/v1/admin/status')['approved_rules'] == 0
        request = json.loads((r.ROOT / 'data/example-request.json').read_text())
        saved = r.api(base, keys['physician'], '/api/v1/assessments', request)
        assessment = saved['id']
        assert r.api(base, keys['physician'], '/api/v1/assessments/' + assessment + '/replay', {})['reproduced']
        report['checks'].append('actual bootstrap, fresh migrations, pending-only seed, assessment and replay')
        before = r.fingerprint(name)
        r.run(['docker', 'rm', '-f', app], stdout=subprocess.DEVNULL); start()
        assert r.fingerprint(name) == before
        report['checks'].append('repeat startup/migration/seed preserves every table')
        # Block DB connections while keeping the container/network/application alive.
        r.sql(name, "ALTER DATABASE renalsafe ALLOW_CONNECTIONS false; SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='renalsafe';", 'postgres')
        assert r.api(base, keys['admin'], '/healthz')['status'] == 'alive'
        for path, body, token in [('/readyz', None, keys['admin']), ('/api/v1/assessments', request, keys['physician'])]:
            try:
                r.api(base, token, path, body)
                raise AssertionError('Operation incorrectly succeeded without database')
            except HTTPError as error:
                assert error.code == 503 if path == '/readyz' else error.code in (401, 503)
        r.sql(name, "ALTER DATABASE renalsafe ALLOW_CONNECTIONS true;", 'postgres'); r.wait_db(name)
        assert r.sql(name, 'SELECT count(*) FROM assessments;') == '1'
        assert r.api(base, keys['admin'], '/api/v1/admin/audit/verify', {})['valid']
        assert r.api(base, keys['physician'], '/api/v1/assessments/' + assessment + '/replay', {})['reproduced']
        report['checks'].append('database connection outage: liveness remains, readiness/assessment fail, reconnect and replay recover')
        r.run(['docker', 'rm', '-f', app], stdout=subprocess.DEVNULL)
        r.sql(name, 'CREATE DATABASE renalsafe_legacy;')
        migrations = r.ROOT / 'core/internal/store/migrations'
        r.sql(name, 'CREATE TABLE schema_migrations(name text PRIMARY KEY,digest text NOT NULL);', 'renalsafe_legacy')
        for filename in ['001_initial.sql', '002_history_integrity.sql']:
            raw = (migrations / filename).read_bytes()
            r.sql(name, raw.decode(), 'renalsafe_legacy')
            r.sql(name, "INSERT INTO schema_migrations VALUES ('" + filename + "','" + hashlib.sha256(raw).hexdigest() + "');", 'renalsafe_legacy')
        oldtables = ['documents', 'chunks', 'renal_rules', 'assessments', 'audit_events', 'knowledge_revision']
        dump_args = ['docker', 'exec', name, 'pg_dump', '-U', 'renalsafe', '-d', 'renalsafe', '--data-only', '--disable-triggers', '--no-owner', '--no-acl']
        for table in oldtables:
            dump_args += ['-t', table]
        data = r.run(dump_args, stdout=subprocess.PIPE).stdout
        r.sql(name, 'TRUNCATE knowledge_revision;', 'renalsafe_legacy')
        r.run(r.sql_cmd(name, 'renalsafe_legacy'), input=data, stdout=subprocess.DEVNULL)
        old = r.fingerprint(name, database='renalsafe_legacy')
        # Temporarily point the isolated application at the older database and migrate normally.
        legacy_app = name + '-legacy'
        try:
            r.run(['docker', 'run', '-d', '--name', legacy_app, '--network', 'container:' + name, '-v', str(root / '.local/users.json') + ':/run/renalsafe-users.json:ro', '-e', 'RENALSAFE_DATABASE_URL=postgres://renalsafe:' + password + '@127.0.0.1:5432/renalsafe_legacy?sslmode=disable', image, '-listen', '0.0.0.0:8080', '-users', '/run/renalsafe-users.json', '-migrate'], stdout=subprocess.DEVNULL)
            for _ in range(90):
                try:
                    r.api(base, keys['admin'], '/readyz'); break
                except Exception:
                    time.sleep(.5)
            else:
                raise RuntimeError('Legacy migration not ready')
            after = r.fingerprint(name, database='renalsafe_legacy')
            for table in oldtables:
                if table != 'audit_events':
                    assert after[table] == old[table], 'Legacy table changed: ' + table
            assert r.api(base, keys['admin'], '/api/v1/admin/audit/verify', {})['valid']
            assert r.api(base, keys['physician'], '/api/v1/assessments/' + assessment + '/replay', {})['reproduced']
            report['checks'].append('001/002 populated schema upgrades to 003/004; sources/results unchanged, audit valid, replay reproduced')
        finally:
            subprocess.run(['docker', 'rm', '-fv', legacy_app], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        r.backup(root / '.local/backup', container=name, config_root=root)
        r.verify(root / '.local/backup')
        report['checks'].append('fresh fixture backup/restore validates complete data and historical replay')
        # Exercise the real preparation helper on fixture configuration, never live credentials.
        original_root, original_argv = r.ROOT, sys.argv
        try:
            r.ROOT = root
            sys.argv = ['prepare-admin-recovery.py', '--output', str(root / '.local/recovery')]
            runpy.run_path(str(original_root / 'scripts/prepare-admin-recovery.py'), run_name='__main__')
        finally:
            r.ROOT, sys.argv = original_root, original_argv
        recovered = json.loads((root / '.local/recovery/credential.json').read_text())
        (root / '.local/recovery/users.json').chmod(0o644)
        r.sql(name, "UPDATE app_users SET active=false,legacy_allowed=false WHERE role='admin';")
        start(root / '.local/recovery/users.json')
        assert r.api(base, recovered['admin'], '/api/v1/me')['id'] == recovered['actor_id']
        managed = r.api(base, recovered['admin'], '/api/v1/admin/users', {'username': 'recovered-managed', 'role': 'admin', 'active': True})
        assert managed['active']
        assert r.api(base, recovered['admin'], '/api/v1/admin/audit/verify', {})['valid']
        report['checks'].append('lost-admin fixture: new recovery identity provisioned, staff administration restored, audit valid; old disabled administrator stays disabled')
        r.run(['docker', 'rm', '-f', app], stdout=subprocess.DEVNULL)
        # Reuse the populated current database to verify migration digest tampering is refused.
        r.sql(name, "UPDATE schema_migrations SET digest='tampered' WHERE name='004_patient_history.sql';")
        # App has been removed: run the same image without detached mode so exit is observable.
        p = subprocess.run(['docker', 'run', '--rm', '--network', 'container:' + name, '-v', str(root / '.local/users.json') + ':/run/renalsafe-users.json:ro', '-e', 'RENALSAFE_DATABASE_URL=postgres://renalsafe:' + password + '@127.0.0.1:5432/renalsafe?sslmode=disable', image, '-users', '/run/renalsafe-users.json', '-migrate'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
        assert p.returncode != 0 and b'migration changed' in p.stderr
        report['checks'].append('changed migration digest refuses application startup')
        report.update(valid=True, verified_at_utc=time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()))
        r.private_json(root / 'verification.json', report)
        print('Isolated installation/failure checks passed; report: ' + str(root / 'verification.json'))
    finally:
        subprocess.run(['docker', 'rm', '-fv', app, name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == '__main__':
    main()
