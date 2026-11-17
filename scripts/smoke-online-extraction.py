#!/usr/bin/env python3
"""Make one paid public-Meronem call, or two with --compare.

Reads the ignored .env without executing shell code or printing credentials.
Does not start the application or create/approve clinical rules.
"""
import os
import argparse
import json
from datetime import datetime, timezone
from pathlib import Path
import shlex
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def run():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compare', action='store_true', help='Make two paid calls: full input and retrieval; save a comparison report.')
    parser.add_argument('--provider', choices=['openai', 'nvidia'], help='Override the server provider for this test only.')
    parser.add_argument('--model', help='Override the selected provider model for this test only.')
    parser.add_argument('--timeout-seconds', type=int, default=90, help='Test-only HTTP deadline, 1–600 seconds (default: 90); does not change the server.')
    args = parser.parse_args()
    if not 1 <= args.timeout_seconds <= 600:
        parser.error('--timeout-seconds must be between 1 and 600')
    env = os.environ.copy()
    dotenv = ROOT / '.env'
    if dotenv.exists():
        for line in dotenv.read_text().splitlines():
            if not line.strip() or line.lstrip().startswith('#') or '=' not in line:
                continue
            name, value = line.split('=', 1)
            name = name.strip().removeprefix('export ')
            if name in {'OPENAI_API_KEY', 'RENALSAFE_OPENAI_MODEL', 'NVIDIA_API_KEY', 'RENALSAFE_NVIDIA_MODEL', 'RENALSAFE_EXTRACTION_PROVIDER', 'RENALSAFE_EXTRACTION_RETRIEVAL', 'RENALSAFE_OLLAMA_URL', 'RENALSAFE_EMBEDDING_MODEL'} and not env.get(name):
                env[name] = ' '.join(shlex.split(value, comments=True))
    if args.provider:
        env['RENALSAFE_EXTRACTION_PROVIDER'] = args.provider
    provider = env.get('RENALSAFE_EXTRACTION_PROVIDER') or 'openai'
    if provider not in {'openai', 'nvidia'}:
        raise SystemExit('RENALSAFE_EXTRACTION_PROVIDER must be openai or nvidia.')
    if args.model:
        env['RENALSAFE_NVIDIA_MODEL' if provider == 'nvidia' else 'RENALSAFE_OPENAI_MODEL'] = args.model
    key_name = 'NVIDIA_API_KEY' if provider == 'nvidia' else 'OPENAI_API_KEY'
    if not env.get(key_name):
        raise SystemExit(f'{key_name} is missing; configure the ignored .env file.')
    env['RENALSAFE_LIVE_EXTRACTION'] = '1'
    env['RENALSAFE_LIVE_HTTP_TIMEOUT_SECONDS'] = str(args.timeout_seconds)
    env.setdefault('GOCACHE', '/tmp/renalsafe-go-cache')
    command = ['go', 'test', './internal/extraction', '-run',
               '^TestLiveMeronemExtraction$', '-count=1', f'-timeout={args.timeout_seconds + 60}s', '-v']
    if not args.compare:
        return subprocess.run(command, cwd=ROOT / 'core', env=env).returncode

    directory = ROOT / 'outputs' / 'extraction-comparison' / datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    directory.mkdir(parents=True, mode=0o700)
    runs = {}
    passed = True
    for label, mode in [('full', 'full'), ('retrieval', 'hybrid')]:
        run_env = dict(env, RENALSAFE_EXTRACTION_RETRIEVAL=mode,
                       RENALSAFE_LIVE_OUTPUT_DIR=str(directory / label))
        print(f'Running {label} extraction (one paid request, no automatic retries).', flush=True)
        process = subprocess.run(command, cwd=ROOT / 'core', env=run_env)
        passed = passed and process.returncode == 0
        def read(name):
            path = directory / label / name
            return json.loads(path.read_text()) if path.exists() else {}
        summary = read('summary.json')
        response = read('provider-response.json')
        raw_usage = response.get('usage', {})
        usage = summary.get('usage') or {
            'input_tokens': raw_usage.get('input_tokens', raw_usage.get('prompt_tokens')),
            'output_tokens': raw_usage.get('output_tokens', raw_usage.get('completion_tokens')),
            'total_tokens': raw_usage.get('total_tokens'),
            'input_tokens_details': raw_usage.get('input_tokens_details', raw_usage.get('prompt_tokens_details', {})),
        }
        runs[label] = {
            'provider': provider,
            'test_passed': process.returncode == 0,
            'software_validation_passed': summary.get('software_validation_passed', False),
            'source_regression': read('source-review.json'),
            'source_digest': summary.get('source_digest'),
            'requested_model': summary.get('requested_model'),
            'model': summary.get('returned_model') or response.get('model'),
            'http_status': summary.get('http_status'),
            'http_timeout_seconds': summary.get('http_timeout_seconds'),
            'connection_error_kind': summary.get('connection_error_kind'),
            'response_header_seconds': summary.get('response_header_seconds'),
            'generation_settings': summary.get('generation_settings'),
            'elapsed_seconds': summary.get('elapsed_seconds'),
            'findings': summary.get('findings'),
            'retrieval': summary.get('retrieval'),
            'usage': usage,
            'error': summary.get('error'),
        }
    full, selected = runs['full'], runs['retrieval']
    comparable = bool(full['source_digest']) and full['source_digest'] == selected['source_digest'] and bool(full['model']) and full['model'] == selected['model']
    savings = {}
    if comparable:
        for field in ['input_tokens', 'output_tokens', 'total_tokens']:
            baseline, optimized = full['usage'].get(field), selected['usage'].get(field)
            if isinstance(baseline, int) and isinstance(optimized, int) and baseline > 0:
                savings[field] = {'full': baseline, 'retrieval': optimized,
                                  'saved': baseline - optimized,
                                  'reduction_percent': round(100 * (baseline - optimized) / baseline, 2)}
    report = {
        'runs': runs, 'same_source_and_model': comparable,
        'both_tests_passed': passed, 'token_comparison': savings,
        'limitations': ['One run per mode; generation and latency vary.',
                        'Source regression checks are not independent clinical validation.',
                        'Token totals do not account for different cached-input and output prices.',
                        'Semantic retrieval is only live-tested if the recorded mode is hybrid.'],
    }
    path = directory / 'comparison.json'
    path.write_text(json.dumps(report, indent=2) + '\n')
    path.chmod(0o600)
    print('Comparison report:', path)
    print(json.dumps({'both_tests_passed': passed, 'same_source_and_model': comparable, 'tokens': savings}, indent=2))
    return 0 if passed and comparable else 1


if __name__ == '__main__':
    sys.exit(run())
