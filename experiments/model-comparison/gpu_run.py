"""Run one model in a child process so its GPU memory is released on exit."""
import argparse
import json
import os
from pathlib import Path
import sys

import benchmark as b


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model', choices=list(b.MODELS), required=True)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--dataset', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--split', default='development', choices=['development', 'test'])
    parser.add_argument('--quantization', default='nf4', choices=['nf4', 'none'])
    parser.add_argument('--max-input-tokens', type=int, default=4096)
    parser.add_argument('--max-new-tokens', type=int, default=2048)
    args = parser.parse_args()
    adapter = None
    stage = 'validate_dataset'
    try:
        data = b.validate_dataset(json.loads(Path(args.dataset).read_text()))
        stage = 'load_model'
        adapter = b.TransformersAdapter(args.model, args.revision, token=os.environ.get('HF_TOKEN'),
                    quantization=args.quantization, max_input_tokens=args.max_input_tokens,
                    max_new_tokens=args.max_new_tokens)
        stage = 'run_or_resume'
        b.run(adapter, data, args.output, split=args.split)
    except Exception as exc:
        # Never dump a provider exception or environment into notebook outputs.
        print(json.dumps({'model': args.model, 'stage': stage, 'error_type': type(exc).__name__,
                          'action': 'Check GPU, model access, package compatibility and output-directory identity. See README troubleshooting.'}))
        return 1
    finally:
        if adapter is not None:
            adapter.close()
    return 0


if __name__ == '__main__':
    sys.exit(main())
