#!/usr/bin/env python3
"""Evaluate the live local evidence API against explicit, versioned source labels.

Searches append audit events. This tool never imports, indexes or approves rules.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import statistics
import time
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ValueError('API redirects are forbidden')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False,
                                     separators=(',', ':')).encode()).hexdigest()


def validate(dataset):
    if dataset.get('schema_version') != 1 or dataset.get('purpose') not in ('software_fixture', 'independent_evaluation'):
        raise ValueError('schema_version 1 and an explicit evaluation purpose required')
    if dataset['purpose'] == 'independent_evaluation':
        review = dataset.get('review', {})
        if not all(review.get(k) for k in ('reviewer', 'reviewed_at', 'method')):
            raise ValueError('independent evaluation requires reviewer, date and method metadata')
    cases = dataset.get('cases', [])
    if not cases:
        raise ValueError('at least one labeled case required')
    seen = set()
    for case in cases:
        if not case.get('id') or case['id'] in seen:
            raise ValueError('case IDs must be nonempty and unique')
        seen.add(case['id'])
        context = case.get('context', {})
        if set(context) != {'product', 'route', 'form', 'indication'} or not all(context.values()):
            raise ValueError('exact product context required')
        if not 2 <= len(case.get('query', '')) <= 500:
            raise ValueError('query must contain 2–500 characters')
        labels = case.get('relevant')
        if not isinstance(labels, list) or (not labels and case.get('expect_abstention') is not True):
            raise ValueError('relevant labels or explicit expected abstention required')
        if labels and case.get('expect_abstention'):
            raise ValueError('abstention case cannot contain positive labels')
        if len({digest(label) for label in labels}) != len(labels):
            raise ValueError('duplicate relevance labels')
        for label in labels:
            sha = label.get('document_sha256', '')
            if len(sha) != 64 or any(c not in '0123456789abcdef' for c in sha):
                raise ValueError('each label requires a lowercase source SHA-256')
            if label.get('section') not in ('4.1','4.2','4.3','4.4','4.5','4.6','4.7','4.8','4.9','5.1','5.2') or not label.get('quote', '').strip():
                raise ValueError('each label requires an exact source quote and section')
    return dataset


def matches(hit, label):
    return (hit['source']['document_sha256'] == label['document_sha256']
            and hit['chunk']['section'] == label['section']
            and label['quote'] in hit['chunk']['text'])


def score(case, result, allowed_sources, allowed_document_ids=None):
    hits, labels = result['hits'], case['relevant']
    authorized = [h['source']['document_sha256'] in allowed_sources and
                  (allowed_document_ids is None or h['chunk'].get('document_id') in allowed_document_ids)
                  for h in hits]
    relevant = [ok and any(matches(h, label) for label in labels) for h, ok in zip(hits, authorized)]
    found = sum(any(ok and matches(h, label) for h, ok in zip(hits, authorized)) for label in labels)
    wrong = sum(not ok for ok in authorized)
    # Precision uses all returned passages; recall uses labeled evidence units,
    # so retrieving duplicate passages cannot inflate recall.
    return {
        'recall_at_k': found / len(labels) if labels else None,
        'citation_precision': sum(relevant) / len(hits) if hits else None,
        'reciprocal_rank': next((1 / (i+1) for i, ok in enumerate(relevant) if ok), 0) if labels else None,
        'wrong_source_count': wrong,
        'unlabeled_passage_count': len(hits)-sum(relevant),
        'abstained': not hits,
        'expected_abstention_correct': not hits if case.get('expect_abstention') else None,
        'status_consistent': result['status'] == ('EVIDENCE_ONLY' if hits else 'NO_EVIDENCE'),
    }


def summarize(rows):
    def mean(key):
        values = [r['metrics'][key] for r in rows if r['metrics'][key] is not None]
        return statistics.mean(values) if values else None
    latencies = sorted(r['latency_ms'] for r in rows)
    return {
        'cases': len(rows), 'mean_recall_at_k': mean('recall_at_k'),
        'mean_citation_precision': mean('citation_precision'),
        'mean_reciprocal_rank': mean('reciprocal_rank'),
        'wrong_source_count': sum(r['metrics']['wrong_source_count'] for r in rows),
        'unlabeled_passage_count': sum(r['metrics']['unlabeled_passage_count'] for r in rows),
        'abstentions': sum(r['metrics']['abstained'] for r in rows),
        'expected_abstention_accuracy': mean('expected_abstention_correct'),
        'degraded_cases': sum(bool(r['response'].get('degraded_reason')) for r in rows),
        'median_latency_ms': statistics.median(latencies),
        'p95_latency_ms': latencies[max(0, (95*len(latencies)+99)//100-1)],
    }


class API:
    def __init__(self, base, token):
        url = urlsplit(base)
        if url.scheme != 'http' or url.hostname not in ('localhost', '127.0.0.1', '::1') or url.username or url.password or url.query or url.fragment or url.path not in ('', '/'):
            raise ValueError('a loopback HTTP origin is required')
        self.base, self.token = base.rstrip('/'), token
        self.opener = build_opener(NoRedirect)

    def call(self, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        request = Request(self.base+'/api/v1'+path, data=data,
                          headers={'Authorization': 'Bearer '+self.token, 'Content-Type': 'application/json'})
        with self.opener.open(request, timeout=95) as response:
            return json.load(response)


def run(api, dataset, modes, k, model_revision):
    documents = api.call('/documents')
    corpus = sorted(({'id': d['id'], 'context': d['context'], 'source': d['source'],
                      'page_start': d['page_start'], 'page_end': d['page_end']}
                     for d in documents if d['status'] == 'ACTIVE'), key=digest)
    for case in dataset['cases']:
        allowed = {d['source']['document_sha256'] for d in corpus if d['context'] == case['context']}
        for label in case['relevant']:
            if label['document_sha256'] not in allowed:
                raise ValueError('case '+case['id']+': labeled source is absent from the active exact-context corpus')
    report = {'schema_version': 1, 'started_at': datetime.now(timezone.utc).isoformat(),
              'purpose': dataset['purpose'], 'dataset_sha256': digest(dataset),
              'corpus_sha256': digest(corpus), 'corpus': corpus, 'k': k,
              'declared_model_revision': model_revision, 'runs': {}, 'complete': False}
    revisions, models = set(), set()
    for mode in modes:
        rows = []
        for case in dataset['cases']:
            request = {key: case[key] for key in ('context', 'query', 'section') if key in case}
            request.update(mode=mode, limit=k)
            started = time.perf_counter()
            result = api.call('/evidence/search', request)
            elapsed = (time.perf_counter()-started)*1000
            revisions.add(result['knowledge_revision'])
            if result.get('embedding_model'):
                models.add(result['embedding_model'])
            allowed = {d['source']['document_sha256'] for d in corpus if d['context'] == case['context']}
            rows.append({'case_id': case['id'], 'latency_ms': elapsed, 'request': request,
                         'metrics': score(case, result, allowed, {d['id'] for d in corpus if d['context']==case['context']}), 'response': result})
        report['runs'][mode] = {'summary': summarize(rows), 'cases': rows}
    final_documents = api.call('/documents')
    final_corpus = sorted(({'id': d['id'], 'context': d['context'], 'source': d['source'],
                            'page_start': d['page_start'], 'page_end': d['page_end']}
                           for d in final_documents if d['status'] == 'ACTIVE'), key=digest)
    report['knowledge_revisions'] = sorted(revisions)
    report['embedding_models'] = sorted(models)
    report['complete'] = len(revisions) == 1 and digest(final_corpus) == digest(corpus) and len(models) <= 1
    report['comparison_valid'] = report['complete'] and all(
        not row['response'].get('degraded_reason') and row['response']['mode'] == mode
        for mode, run in report['runs'].items() for row in run['cases'])
    report['finished_at'] = datetime.now(timezone.utc).isoformat()
    report['limits'] = ['Relevance depends on supplied labels; unlabelled passages are not proven unsafe.',
                        'Extractive retrieval metrics do not establish clinical validity or generative answer quality.',
                        'Model revision is operator-declared; this runner does not attest model weights.']
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('dataset', type=Path)
    parser.add_argument('--base-url', default='http://127.0.0.1:8080')
    parser.add_argument('--tokens', type=Path, default=Path('.local/tokens.json'))
    parser.add_argument('--modes', nargs='+', choices=['lexical', 'hybrid'], default=['lexical','hybrid'])
    parser.add_argument('--model-revision', help='immutable local model weights revision; required for hybrid comparison')
    parser.add_argument('--k', type=int, default=5)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if not 1 <= args.k <= 20:
        parser.error('k must be 1–20')
    if 'hybrid' in args.modes and not args.model_revision:
        parser.error('hybrid requires --model-revision; run --modes lexical until a pinned model is configured')
    dataset = validate(json.loads(args.dataset.read_text()))
    token = json.loads(args.tokens.read_text())['physician']
    report = run(API(args.base_url, token), dataset, list(dict.fromkeys(args.modes)), args.k, args.model_revision)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2, ensure_ascii=False)+'\n')
    print(json.dumps({'report': str(args.output), 'complete': report['complete'],
                      'comparison_valid': report['comparison_valid'],
                      'summaries': {mode: run['summary'] for mode, run in report['runs'].items()}}, indent=2))
    return 0 if report['comparison_valid'] else 2


if __name__ == '__main__':
    raise SystemExit(main())
