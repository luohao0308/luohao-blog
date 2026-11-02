#!/usr/bin/env python3
"""Integrity manifests for the fixed, single-row mysqldump format. No SQL execution."""
import argparse
import gzip
import hashlib
import json
import pathlib
import re
import sys

FORMAT = 'mysqldump-single-row-v1'
IDENT = rb'[a-z_][a-z0-9_]*'
CREATE = re.compile(rb'^CREATE TABLE `(' + IDENT + rb')` \($')
INSERT = re.compile(rb'^INSERT INTO `(' + IDENT + rb')` \(([^\n]+)\) VALUES (.+);$')


def single_row(value):
    quoted = escaped = False
    depth = 0
    closed = False
    for byte in value:
        if quoted:
            if escaped:
                escaped = False
            elif byte == 92:
                escaped = True
            elif byte == 39:
                quoted = False
        elif byte == 39:
            quoted = True
        elif byte == 40:
            if closed or depth != 0:
                raise ValueError('multiple rows or expressions are unsupported')
            depth += 1
        elif byte == 41:
            depth -= 1
            if depth == 0:
                closed = True
        elif depth == 0 and byte not in b' \t':
            raise ValueError('unsupported row suffix')
        if depth < 0:
            raise ValueError('unbalanced row')
    if quoted or escaped or depth or not closed:
        raise ValueError('incomplete row')



def canonical_ddl(lines):
    # MySQL SHOW CREATE may add an explicit CHARACTER SET on re-import.
    # Infer it only for the observed char/varchar COLLATE form, where the
    # collation's charset already agrees with the table default. Preserve all
    # actual charset/collation attributes and every other DDL byte.
    charset = re.search(rb"\bDEFAULT CHARSET=([a-z0-9_]+)\b", lines[-1])
    if not charset:
        return b''.join(lines)
    name = charset[1]
    column = re.compile(rb"^(  `" + IDENT + rb"` (?:char|varchar)\([0-9]+\) )COLLATE ([a-z0-9_]+)(?= )")
    result = []
    for line in lines:
        match = column.match(line)
        if match and match[2].startswith(name + b'_'):
            line = match[1] + b'CHARACTER SET ' + name + b' COLLATE ' + line[match.end(1) + len(b'COLLATE '):]
        result.append(line)
    return b''.join(result)


def inspect_dump(path):
    tables = {}
    current = None
    ddl = []
    source_schema = None
    ended = False
    digest = hashlib.sha256()
    with gzip.open(path, 'rb') as stream:
        for raw in stream:
            digest.update(raw)
            line = raw.rstrip(b'\r\n')
            if ended and line.strip():
                raise ValueError('data after epilogue')
            start = CREATE.fullmatch(line)
            if start:
                if current is not None:
                    raise ValueError('nested DDL')
                current = start[1].decode('ascii')
                if current in tables:
                    raise ValueError('duplicate table')
                ddl = [raw]
                tables[current] = {'rows': 0, 'data': hashlib.sha256()}
                continue
            if current is not None:
                ddl.append(raw)
                if line.startswith(b') ENGINE=') and line.endswith(b';'):
                    tables[current]['ddl_sha256'] = hashlib.sha256(canonical_ddl(ddl)).hexdigest()
                    current = None
                continue
            match = INSERT.fullmatch(line)
            if match:
                name = match[1].decode('ascii')
                if name not in tables:
                    raise ValueError('insert without table')
                single_row(match[3])
                tables[name]['rows'] += 1
                tables[name]['data'].update(raw)
                if name == 'schema_migrations':
                    if source_schema is not None or match[2] != b'`version`, `dirty`':
                        raise ValueError('unsupported migration record')
                    version = re.fullmatch(rb'\((20|21),0\)', match[3])
                    if not version:
                        raise ValueError('unsupported or dirty schema')
                    source_schema = int(version[1])
            elif line.startswith((b'INSERT ', b'REPLACE ', b'CREATE TABLE ', b'DELIMITER', b'UPDATE ', b'DELETE ')):
                raise ValueError('unsupported SQL format')
            elif line.strip() and not (line.startswith(b'--') or re.fullmatch(rb'DROP TABLE IF EXISTS `' + IDENT + rb'`;', line) or re.fullmatch(rb'LOCK TABLES `' + IDENT + rb'` WRITE;', line) or line == b'UNLOCK TABLES;' or re.fullmatch(rb'/\*![0-9]+ SET [A-Za-z0-9_@=,+\x27 :.-]+ \*/;', line) or re.fullmatch(rb'/\*![0-9]+ ALTER TABLE `' + IDENT + rb'` (ENABLE|DISABLE) KEYS \*/;', line)):
                raise ValueError('unknown dump statement')
            if line == b'/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;':
                ended = True
    if current is not None or not ended or source_schema is None:
        raise ValueError('incomplete fixed-format dump')
    marker = tables.get('time_contract')
    if (source_schema == 21 and (marker is None or marker['rows'] != 1)) or (source_schema == 20 and marker is not None):
        raise ValueError('schema and contract marker differ')
    records = []
    for name, value in sorted(tables.items()):
        records.append({'name': name, 'rows': value['rows'], 'data_sha256': value['data'].hexdigest(), 'ddl_sha256': value['ddl_sha256']})
    archive_digest = hashlib.sha256()
    with pathlib.Path(path).open('rb') as archive:
        while chunk := archive.read(1024 * 1024):
            archive_digest.update(chunk)
    return {'schema_version': 1, 'format': FORMAT, 'dump_sha256': digest.hexdigest(), 'archive_sha256': archive_digest.hexdigest(), 'source_schema_version': source_schema, 'tables': records}


def compare(expected, actual):
    if expected.get('schema_version') != 1 or expected.get('format') != FORMAT:
        raise ValueError('unsupported sidecar')
    if expected['source_schema_version'] != actual['source_schema_version'] or expected['tables'] != actual['tables']:
        raise ValueError('restored snapshot differs')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['create', 'check', 'compare'])
    parser.add_argument('dump', type=pathlib.Path)
    parser.add_argument('manifest', type=pathlib.Path)
    args = parser.parse_args()
    actual = inspect_dump(args.dump)
    if args.mode == 'create':
        with args.manifest.open('x') as target:
            json.dump(actual, target, sort_keys=True, indent=2)
            target.write('\n')
        args.manifest.chmod(0o600)
    else:
        expected = json.loads(args.manifest.read_text())
        compare(expected, actual)
        if args.mode == 'check' and (expected['dump_sha256'] != actual['dump_sha256'] or expected['archive_sha256'] != actual['archive_sha256']):
            raise ValueError('backup digest differs')
    print(json.dumps({'verified': True, 'source_schema_version': actual['source_schema_version'], 'tables': len(actual['tables'])}))


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('backup manifest rejected; private diagnostics withheld', file=sys.stderr)
        sys.exit(1)
